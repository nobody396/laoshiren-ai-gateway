package httputil

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/klauspost/compress/zstd"
)

const (
	requestBodyReadInitCap    = 512
	requestBodyReadMaxInitCap = 1 << 20
	maxDecompressedBodySize   = 64 << 20
)

// ReadRequestBodyWithPrealloc reads request body with preallocated buffer based on content length.
func ReadRequestBodyWithPrealloc(req *http.Request) ([]byte, error) {
	return readRequestBodyWithPrealloc(req, 0)
}

// ReadRequestBodyWithPreallocLimit also limits the decoded body. The request
// body may already be bounded by http.MaxBytesReader, but compressed requests
// need a second limit after decompression to prevent bypassing endpoint limits.
func ReadRequestBodyWithPreallocLimit(req *http.Request, maxDecodedBytes int64) ([]byte, error) {
	return readRequestBodyWithPrealloc(req, maxDecodedBytes)
}

func readRequestBodyWithPrealloc(req *http.Request, maxDecodedBytes int64) ([]byte, error) {
	if req == nil || req.Body == nil {
		return nil, nil
	}
	explicitLimit := maxDecodedBytes > 0
	decodedLimit := maxDecodedBytes
	if decodedLimit <= 0 {
		decodedLimit = maxDecompressedBodySize
	}

	capHint := requestBodyReadInitCap
	if req.ContentLength > 0 {
		switch {
		case req.ContentLength < int64(requestBodyReadInitCap):
			capHint = requestBodyReadInitCap
		case req.ContentLength > int64(requestBodyReadMaxInitCap):
			capHint = requestBodyReadMaxInitCap
		default:
			capHint = int(req.ContentLength)
		}
	}

	buf := bytes.NewBuffer(make([]byte, 0, capHint))
	if _, err := io.Copy(buf, req.Body); err != nil {
		return nil, err
	}
	raw := buf.Bytes()
	if explicitLimit && int64(len(raw)) > decodedLimit {
		return nil, &http.MaxBytesError{Limit: decodedLimit}
	}

	enc := strings.ToLower(strings.TrimSpace(req.Header.Get("Content-Encoding")))
	if enc == "" || enc == "identity" {
		return raw, nil
	}

	decoded, err := decompressRequestBody(enc, raw, decodedLimit)
	if err != nil {
		return nil, fmt.Errorf("decode Content-Encoding %q: %w", enc, err)
	}
	req.Header.Del("Content-Encoding")
	req.Header.Del("Content-Length")
	req.ContentLength = int64(len(decoded))
	return decoded, nil
}

func decompressRequestBody(encoding string, raw []byte, maxDecodedBytes int64) ([]byte, error) {
	switch encoding {
	case "zstd":
		dec, err := zstd.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer dec.Close()
		return readDecompressedWithLimit(dec, maxDecodedBytes)
	case "gzip", "x-gzip":
		gr, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer func() { _ = gr.Close() }()
		return readDecompressedWithLimit(gr, maxDecodedBytes)
	case "deflate":
		zr, err := zlib.NewReader(bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		defer func() { _ = zr.Close() }()
		return readDecompressedWithLimit(zr, maxDecodedBytes)
	default:
		return nil, errors.New("unsupported Content-Encoding")
	}
}

func readDecompressedWithLimit(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = maxDecompressedBodySize
	}
	limited := io.LimitReader(r, maxBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, &http.MaxBytesError{Limit: maxBytes}
	}
	return data, nil
}
