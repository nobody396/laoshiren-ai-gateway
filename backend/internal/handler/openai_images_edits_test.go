package handler

import (
	"bytes"
	"encoding/base64"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/server/middleware"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type openAIImagesEditsUpstream struct {
	service.HTTPUpstream
	request *http.Request
	body    []byte
}

func (u *openAIImagesEditsUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.request = req
	u.body, _ = io.ReadAll(req.Body)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
			"x-request-id": []string{"req_image_edit"},
		},
		Body: io.NopCloser(strings.NewReader(`{
			"created":1710000000,
			"data":[{"b64_json":"` + codexNativeImageBridgeTestPNG + `"}],
			"usage":{"input_tokens":10,"output_tokens":20,"total_tokens":30}
		}`)),
	}, nil
}

func TestOpenAIImagesEdits_MultipartForwardsFilesMaskAndParameters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	group := &service.Group{
		ID:                   51,
		Name:                 "GPT Image 2",
		Platform:             service.PlatformOpenAI,
		AllowImageGeneration: true,
	}
	account := service.Account{
		ID:          39,
		Name:        "image-edit-upstream",
		Platform:    service.PlatformOpenAI,
		Type:        service.AccountTypeAPIKey,
		Status:      service.StatusActive,
		Schedulable: true,
		Concurrency: 4,
		Credentials: map[string]any{
			"api_key":       "test-only-key",
			"base_url":      "https://images.example.test/v1",
			"model_mapping": map[string]any{"gpt-image-2": "gpt-image-2-upstream"},
		},
		Extra: map[string]any{
			"supports_images":                                       true,
			"supports_image_edits":                                  true,
			service.OpenAIImageGenerationPriorityExtraKey:           1,
			service.OpenAIImageGenerationModelsExtraKey:             []any{"gpt-image-2"},
			service.OpenAIImageGenerationTransportExtraKey:          service.OpenAIImageGenerationTransportImages,
			service.OpenAIImageGenerationOmitResponseFormatExtraKey: true,
		},
		AccountGroups: []service.AccountGroup{{AccountID: 39, GroupID: group.ID, Priority: 1}},
	}
	upstream := &openAIImagesEditsUpstream{}
	handler := newCodexResponsesTestHandler(t, []service.Account{account}, upstream)

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)
	require.NoError(t, writer.WriteField("model", "gpt-image-2"))
	require.NoError(t, writer.WriteField("prompt", "replace the background"))
	require.NoError(t, writer.WriteField("size", "1024x1024"))
	require.NoError(t, writer.WriteField("quality", "low"))
	require.NoError(t, writer.WriteField("background", "opaque"))
	require.NoError(t, writer.WriteField("output_format", "png"))
	require.NoError(t, writer.WriteField("output_compression", "80"))
	require.NoError(t, writer.WriteField("input_fidelity", "high"))
	require.NoError(t, writer.WriteField("moderation", "auto"))
	require.NoError(t, writer.WriteField("response_format", "b64_json"))
	imagePart, err := writer.CreateFormFile("image", "source.png")
	require.NoError(t, err)
	pngBytes, err := base64.StdEncoding.DecodeString(codexNativeImageBridgeTestPNG)
	require.NoError(t, err)
	largeImage := append(append([]byte(nil), pngBytes...), bytes.Repeat([]byte{0x0}, (1<<20)+17)...)
	_, err = imagePart.Write(largeImage)
	require.NoError(t, err)
	secondImagePart, err := writer.CreateFormFile("image[]", "reference.png")
	require.NoError(t, err)
	_, err = secondImagePart.Write(pngBytes)
	require.NoError(t, err)
	maskPart, err := writer.CreateFormFile("mask", "mask.png")
	require.NoError(t, err)
	_, err = maskPart.Write(pngBytes)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", bytes.NewReader(requestBody.Bytes()))
	c.Request.Header.Set("Content-Type", writer.FormDataContentType())
	c.Request.Header.Set("User-Agent", "curl/8.0")
	apiKey := &service.APIKey{ID: 44, GroupID: &group.ID, Group: group, User: &service.User{ID: 2, Status: service.StatusActive}}
	c.Set(string(middleware.ContextKeyAPIKey), apiKey)
	c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2, Concurrency: 4})

	handler.Images(c)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	_, loggedMultipart := c.Get(service.OpsUpstreamRequestBodyKey)
	require.False(t, loggedMultipart, "uploaded image bytes must not be stored in ops request context")
	require.NotNil(t, upstream.request)
	require.Equal(t, "/v1/images/edits", upstream.request.URL.Path)
	mediaType, params, err := mime.ParseMediaType(upstream.request.Header.Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)

	reader := multipart.NewReader(bytes.NewReader(upstream.body), params["boundary"])
	form, err := reader.ReadForm(1 << 20)
	require.NoError(t, err)
	t.Cleanup(func() { _ = form.RemoveAll() })
	require.Equal(t, []string{"gpt-image-2-upstream"}, form.Value["model"])
	require.Equal(t, []string{"replace the background"}, form.Value["prompt"])
	require.Equal(t, []string{"1024x1024"}, form.Value["size"])
	require.Equal(t, []string{"low"}, form.Value["quality"])
	require.Equal(t, []string{"opaque"}, form.Value["background"])
	require.Equal(t, []string{"png"}, form.Value["output_format"])
	require.Equal(t, []string{"80"}, form.Value["output_compression"])
	require.Equal(t, []string{"high"}, form.Value["input_fidelity"])
	require.Equal(t, []string{"auto"}, form.Value["moderation"])
	require.NotContains(t, form.Value, "response_format", "account compatibility flag must remove only this field")
	require.Len(t, form.File["image"], 1)
	require.Equal(t, "source.png", form.File["image"][0].Filename)
	require.Equal(t, int64(len(largeImage)), form.File["image"][0].Size)
	require.Len(t, form.File["image[]"], 1)
	require.Equal(t, "reference.png", form.File["image[]"][0].Filename)
	require.Len(t, form.File["mask"], 1)
	require.Equal(t, "mask.png", form.File["mask"][0].Filename)
}
