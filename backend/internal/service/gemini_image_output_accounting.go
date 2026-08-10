package service

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const geminiImageOutputCounterKey = "gemini_image_output_counter"

// geminiImageOutputCounter keeps the largest number of inline images observed
// in a single payload. Gemini-compatible SSE providers may resend cumulative
// payloads, so summing chunks would double charge the same image.
type geminiImageOutputCounter struct {
	count int
}

func beginGeminiImageOutputObservation(c *gin.Context) {
	if c != nil {
		c.Set(geminiImageOutputCounterKey, &geminiImageOutputCounter{})
	}
}

func geminiImageOutputCounterFromContext(c *gin.Context) *geminiImageOutputCounter {
	if c == nil {
		return nil
	}
	value, ok := c.Get(geminiImageOutputCounterKey)
	if !ok {
		return nil
	}
	counter, _ := value.(*geminiImageOutputCounter)
	return counter
}

func observeGeminiImageOutputs(c *gin.Context, payload []byte) {
	counter := geminiImageOutputCounterFromContext(c)
	if counter == nil {
		return
	}
	if count := countGeminiInlineImageOutputs(payload); count > counter.count {
		counter.count = count
	}
}

func observedGeminiImageOutputs(c *gin.Context) int {
	if counter := geminiImageOutputCounterFromContext(c); counter != nil {
		return counter.count
	}
	return 0
}

func resolveGeminiImageCount(c *gin.Context, requestedModel, upstreamModel string) int {
	if observed := observedGeminiImageOutputs(c); observed > 0 {
		return observed
	}
	if isImageGenerationModel(requestedModel) || isImageGenerationModel(upstreamModel) {
		return 1
	}
	return 0
}

func countGeminiInlineImageOutputs(payload []byte) int {
	if len(payload) == 0 || !gjson.ValidBytes(payload) {
		return 0
	}
	count := 0
	gjson.GetBytes(payload, "candidates").ForEach(func(_, candidate gjson.Result) bool {
		candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
			if geminiPartIsInlineImage(part) {
				count++
			}
			return true
		})
		return true
	})
	return count
}

func geminiPartIsInlineImage(part gjson.Result) bool {
	inline := part.Get("inlineData")
	if !inline.Exists() {
		inline = part.Get("inline_data")
	}
	if !inline.Exists() {
		return false
	}
	mimeType := inline.Get("mimeType")
	if !mimeType.Exists() {
		mimeType = inline.Get("mime_type")
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(mimeType.String())), "image/") {
		return false
	}
	return strings.TrimSpace(inline.Get("data").String()) != ""
}
