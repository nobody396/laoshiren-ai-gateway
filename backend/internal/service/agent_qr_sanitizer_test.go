package service

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeAgentQRCode_StripsJPEGMetadataAndNormalizesPNG(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			source.Set(x, y, color.RGBA{R: 20, G: 40, B: 60, A: 255})
		}
	}
	var encoded bytes.Buffer
	require.NoError(t, jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 90}))

	metadata := []byte("Exif\x00\x00SENSITIVE-GPS-METADATA")
	app1 := make([]byte, 4+len(metadata))
	app1[0], app1[1] = 0xff, 0xe1
	binary.BigEndian.PutUint16(app1[2:4], uint16(len(metadata)+2))
	copy(app1[4:], metadata)
	jpegWithEXIF := append([]byte{}, encoded.Bytes()[:2]...)
	jpegWithEXIF = append(jpegWithEXIF, app1...)
	jpegWithEXIF = append(jpegWithEXIF, encoded.Bytes()[2:]...)

	sanitized, err := sanitizeAgentQRCode(
		bytes.NewReader(jpegWithEXIF),
		int64(len(jpegWithEXIF)),
		"image/jpeg",
	)
	require.NoError(t, err)
	require.Equal(t, "image/png", sanitized.ContentType)
	require.Equal(t, ".png", sanitized.Extension)
	require.True(t, bytes.HasPrefix(sanitized.Data, []byte{0x89, 'P', 'N', 'G'}))
	require.NotContains(t, string(sanitized.Data), "SENSITIVE-GPS-METADATA")
	_, err = png.Decode(bytes.NewReader(sanitized.Data))
	require.NoError(t, err)
}

func TestSanitizeAgentQRCode_RejectsMIMEAndSizeMismatches(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, source))
	raw := encoded.Bytes()

	_, err := sanitizeAgentQRCode(bytes.NewReader(raw), int64(len(raw)), "image/jpeg")
	require.Error(t, err)

	_, err = sanitizeAgentQRCode(bytes.NewReader(raw), int64(len(raw)+1), "image/png")
	require.Error(t, err)
}
