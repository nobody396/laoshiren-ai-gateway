package httputil

import (
	"bytes"
	"compress/gzip"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadRequestBodyWithPreallocLimitRejectsRawBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader("12345"))

	_, err := ReadRequestBodyWithPreallocLimit(req, 4)
	require.Error(t, err)

	var maxErr *http.MaxBytesError
	require.True(t, errors.As(err, &maxErr))
	require.Equal(t, int64(4), maxErr.Limit)
}

func TestReadRequestBodyWithPreallocLimitRejectsDecodedBody(t *testing.T) {
	var compressed bytes.Buffer
	zw := gzip.NewWriter(&compressed)
	_, err := zw.Write([]byte(strings.Repeat("x", 32)))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(compressed.Bytes()))
	req.Header.Set("Content-Encoding", "gzip")

	_, err = ReadRequestBodyWithPreallocLimit(req, 16)
	require.Error(t, err)

	var maxErr *http.MaxBytesError
	require.True(t, errors.As(err, &maxErr))
	require.Equal(t, int64(16), maxErr.Limit)
}

func TestReadRequestBodyWithPreallocLimitAcceptsBoundary(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader("1234"))

	body, err := ReadRequestBodyWithPreallocLimit(req, 4)
	require.NoError(t, err)
	require.Equal(t, "1234", string(body))
}
