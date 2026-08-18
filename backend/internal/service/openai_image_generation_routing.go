package service

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	// OpenAIFixedImageRendererModel is the stable public model routed to the
	// dedicated image pool when Codex asks for a new image in natural language.
	OpenAIFixedImageRendererModel = "gpt-image-2"
	// OpenAIImageGenerationPriorityExtraKey opts an OpenAI account into
	// Responses API image-generation routing. Lower positive values win.
	OpenAIImageGenerationPriorityExtraKey = "openai_image_generation_priority"
	// OpenAIImageGenerationModelsExtraKey optionally restricts that preference
	// to public request models. Entries support the same trailing-* wildcard as
	// account model mappings. A present but empty/invalid list fails closed.
	OpenAIImageGenerationModelsExtraKey = "openai_image_generation_models"
	// OpenAIImageGenerationTransportExtraKey selects the upstream protocol for
	// an image route. The missing value preserves the existing Responses API
	// behavior; "images" opts an API-key account into the native Images API.
	OpenAIImageGenerationTransportExtraKey  = "openai_image_generation_transport"
	OpenAIImageGenerationTransportResponses = "responses"
	OpenAIImageGenerationTransportImages    = "images"
	// OpenAIImageGenerationOmitResponseFormatExtraKey enables compatibility
	// with native Images providers (notably Azure-backed routes) that always
	// return base64 image data and reject the OpenAI response_format field.
	OpenAIImageGenerationOmitResponseFormatExtraKey = "openai_image_generation_omit_response_format"
)

var (
	openAIEnglishImageActionPattern = regexp.MustCompile(`(?i)\b(generate|create|draw|paint|illustrate|design|render|make|edit|modify|transform|restyle|redraw)\b`)
	openAIEnglishImageNounPattern   = regexp.MustCompile(`(?i)\b(image|images|picture|pictures|photo|photos|illustration|illustrations|poster|posters|logo|logos|icon|icons|wallpaper|wallpapers|avatar|avatars|artwork|cover art)\b`)
	openAIEnglishImageHowToPattern  = regexp.MustCompile(`(?i)\bhow\s+(?:to|do\s+i|can\s+i|should\s+i)\s+(?:generate|create|draw|paint|illustrate|design|render|make|edit|modify|transform|restyle|redraw)\b`)
	openAIChineseImageHowToPattern  = regexp.MustCompile(`(?:如何|怎么|怎样|怎么才能|怎样才能).{0,16}(?:生成|绘制|画|创作|制作|创建|设计|渲染|重绘|编辑|修改).{0,16}(?:图片|图像|图|插图|海报|封面|头像|壁纸|图标|照片|logo)`)
)

// IsExplicitOpenAIImageGenerationIntent detects only native Responses API
// image-generation signals. Passive tool catalogs (for example an image_gen
// namespace with tool_choice=auto) deliberately do not activate this route.
func IsExplicitOpenAIImageGenerationIntent(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil || !openAIToolChoiceAllowsImageBridge(reqBody["tool_choice"]) {
		return false
	}
	if openAIToolChoiceSelectsImageGeneration(gjson.GetBytes(body, "tool_choice")) {
		return true
	}
	if tools := gjson.GetBytes(body, "tools"); tools.IsArray() {
		found := false
		tools.ForEach(func(_, item gjson.Result) bool {
			found = strings.TrimSpace(item.Get("type").String()) == "image_generation"
			return !found
		})
		if found {
			prompt, hasInputImage, ok := latestOpenAIUserPrompt(body)
			return ok && openAIUserPromptRequestsImage(prompt, hasInputImage)
		}
	}
	return false
}

// IsOpenAICodexSemanticImageGenerationIntent recognizes an explicit natural-
// language request to create or edit an image in the latest user turn. The
// caller must separately verify that the request came from an official Codex
// client. Passive tool catalogs alone never activate this path.
func IsOpenAICodexSemanticImageGenerationIntent(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	prompt, hasInputImage, ok := latestOpenAIUserPrompt(body)
	return ok &&
		!openAICodexImageRequestNeedsAgentWorkflow(prompt, hasInputImage) &&
		openAIUserPromptRequestsImage(prompt, hasInputImage)
}

// openAICodexImageRequestNeedsAgentWorkflow keeps file-producing, editing and
// multi-asset requests in Codex's local ImageGen workflow. Simple preview-only
// generations can use the shorter fixed-renderer path, while requests that
// need local files, references or post-processing retain Codex's native
// generated_images/save-path behavior.
func openAICodexImageRequestNeedsAgentWorkflow(prompt string, hasInputImage bool) bool {
	if hasInputImage {
		return true
	}
	prompt = strings.ToLower(strings.TrimSpace(prompt))
	if prompt == "" {
		return false
	}
	return containsAnyString(prompt, []string{
		"保存到", "保存至", "存到", "存入", "写入", "复制到", "拷贝到", "放到", "放进",
		"下载", "导出", "文件夹", "目录", "路径", "项目素材", "仓库素材",
		"批量", "多张", "几张", "一组图片", "多个版本", "多版", "变体",
		"参考图", "基于这张", "编辑这张", "修改这张", "重绘这张", "去背景", "抠图", "透明背景",
		"save to", "write to", "copy to", "export", "download", "folder", "directory", "file path",
		"batch", "multiple images", "several images", "variants", "reference image",
		"edit this", "modify this", "redraw this", "remove the background", "transparent background",
	})
}

// HasOpenAICodexExecImageRenderTool reports whether the request exposes the
// Codex Desktop code-mode host's freeform exec tool. Raw Responses
// image_generation_call items are persisted by Codex, but current Desktop
// builds do not turn them into a visible generated-image card. When this exact
// client capability is present, the gateway can safely hand the validated
// image bytes back through exec's generatedImage helper instead.
func HasOpenAICodexExecImageRenderTool(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	if openAICodexToolArrayHasExec(gjson.GetBytes(body, "tools")) {
		return true
	}
	inputs := gjson.GetBytes(body, "input")
	if !inputs.IsArray() {
		return false
	}
	found := false
	inputs.ForEach(func(_, input gjson.Result) bool {
		if !strings.EqualFold(strings.TrimSpace(input.Get("type").String()), "additional_tools") {
			return true
		}
		found = openAICodexToolArrayHasExec(input.Get("tools"))
		return !found
	})
	return found
}

// HasOpenAICodexAdditionalToolsEnvelope recognizes the Responses Lite
// capability envelope emitted by Codex Desktop/app-server. In that exact
// client mode, the bundled code-mode host understands the reserved exec tool
// and its generatedImage helper even when exec is not duplicated in top-level
// hosted tools.
func HasOpenAICodexAdditionalToolsEnvelope(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	inputs := gjson.GetBytes(body, "input")
	if !inputs.IsArray() {
		return false
	}
	found := false
	inputs.ForEach(func(_, input gjson.Result) bool {
		found = strings.EqualFold(strings.TrimSpace(input.Get("type").String()), "additional_tools") && input.Get("tools").IsArray()
		return !found
	})
	return found
}

func openAICodexToolArrayHasExec(tools gjson.Result) bool {
	if !tools.IsArray() {
		return false
	}
	found := false
	tools.ForEach(func(_, item gjson.Result) bool {
		found = strings.EqualFold(strings.TrimSpace(item.Get("type").String()), "custom") &&
			strings.EqualFold(strings.TrimSpace(item.Get("name").String()), "exec")
		return !found
	})
	return found
}

// ShouldUseFixedOpenAIImageRenderer reports whether an official Codex
// Responses request can be rendered by the dedicated GPT Image pool. Image
// edits keep the original Responses tool path because native generation-only
// providers cannot faithfully reproduce an attached-image edit.
//
// A passive image_generation tool catalog is not enough by itself. The latest
// user turn must explicitly request a generated image, or tool_choice must
// force the hosted image tool. This keeps normal text traffic isolated.
func ShouldUseFixedOpenAIImageRenderer(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil || !openAIToolChoiceAllowsImageBridge(reqBody["tool_choice"]) {
		return false
	}
	prompt, hasInputImage, ok := latestOpenAIUserPrompt(body)
	if !ok || hasInputImage {
		return false
	}
	if openAIToolChoiceSelectsImageGeneration(gjson.GetBytes(body, "tool_choice")) {
		return true
	}
	return openAIUserPromptRequestsImage(prompt, false)
}

// LatestOpenAIUserPromptForImageRenderer extracts the single latest user turn
// accepted by the fixed renderer. It intentionally rejects input-image edits.
func LatestOpenAIUserPromptForImageRenderer(body []byte) (string, error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return "", fmt.Errorf("invalid OpenAI Responses request body")
	}
	prompt, hasInputImage, ok := latestOpenAIUserPrompt(body)
	if !ok || strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("image generation prompt is required")
	}
	if hasInputImage {
		return "", fmt.Errorf("fixed image renderer does not support input-image edits")
	}
	return strings.TrimSpace(prompt), nil
}

// PrepareOpenAICodexImageGenerationRequest makes the native Responses
// image_generation capability available to a semantically explicit Codex image
// request. Existing client tools and auto tool selection are preserved so the
// user-selected text model still decides whether to invoke image generation. A
// client-forced non-image tool choice is respected and returns activated=false.
func PrepareOpenAICodexImageGenerationRequest(body []byte) (prepared []byte, activated bool, err error) {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return body, false, fmt.Errorf("invalid OpenAI Responses request body")
	}

	var reqBody map[string]any
	if err := json.Unmarshal(body, &reqBody); err != nil {
		return body, false, fmt.Errorf("decode OpenAI Responses request body: %w", err)
	}
	if !openAIToolChoiceAllowsImageBridge(reqBody["tool_choice"]) {
		return body, false, nil
	}

	tools := make([]any, 0, 1)
	if rawTools, exists := reqBody["tools"]; exists && rawTools != nil {
		var ok bool
		tools, ok = rawTools.([]any)
		if !ok {
			return body, false, fmt.Errorf("tools must be an array")
		}
	}

	hasNativeImageTool := false
	for _, rawTool := range tools {
		tool, ok := rawTool.(map[string]any)
		if ok && strings.TrimSpace(firstNonEmptyString(tool["type"])) == "image_generation" {
			hasNativeImageTool = true
			break
		}
	}
	if !hasNativeImageTool {
		tools = append(tools, map[string]any{"type": "image_generation"})
		reqBody["tools"] = tools
	}

	// Do not force tool_choice. Leaving an absent choice at the Responses API
	// default, or preserving the client's "auto", keeps the native GPT behavior:
	// the selected text model decides whether the image tool is appropriate.
	prepared, err = json.Marshal(reqBody)
	if err != nil {
		return body, false, fmt.Errorf("encode OpenAI Responses image request: %w", err)
	}
	return prepared, true, nil
}

// ForceOpenAICodexImageGenerationToolChoice is used only after the stronger
// generation-only intent gate has selected the fixed image renderer path. That
// path already guaranteed an image before Responses-native routing existed, so
// forcing the hosted tool preserves its contract while allowing native stream
// events and multiple completed image calls from a Responses-capable provider.
func ForceOpenAICodexImageGenerationToolChoice(body []byte) ([]byte, error) {
	prepared, activated, err := PrepareOpenAICodexImageGenerationRequest(body)
	if err != nil {
		return nil, err
	}
	if !activated {
		return nil, fmt.Errorf("image generation tool choice is not allowed")
	}
	forced, err := sjson.SetBytes(prepared, "tool_choice", map[string]any{"type": "image_generation"})
	if err != nil {
		return nil, fmt.Errorf("force image generation tool choice: %w", err)
	}
	return forced, nil
}

func openAIToolChoiceAllowsImageBridge(choice any) bool {
	if choice == nil {
		return true
	}
	switch value := choice.(type) {
	case string:
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "", "auto", "required", "image_generation":
			return true
		default:
			return false
		}
	case map[string]any:
		return openAIMapToolChoiceSelectsImageGeneration(value)
	default:
		return false
	}
}

func openAIMapToolChoiceSelectsImageGeneration(choice map[string]any) bool {
	if choice == nil {
		return false
	}
	if strings.TrimSpace(firstNonEmptyString(choice["type"])) == "image_generation" {
		return true
	}
	if tool, ok := choice["tool"].(map[string]any); ok {
		return openAIMapToolChoiceSelectsImageGeneration(tool)
	}
	return false
}

func latestOpenAIUserPrompt(body []byte) (text string, hasInputImage bool, ok bool) {
	input := gjson.GetBytes(body, "input")
	if input.Type == gjson.String {
		return strings.TrimSpace(input.String()), false, strings.TrimSpace(input.String()) != ""
	}
	if !input.IsArray() {
		return "", false, false
	}

	items := input.Array()
	for i := len(items) - 1; i >= 0; i-- {
		item := items[i]
		if item.Type == gjson.String {
			value := strings.TrimSpace(item.String())
			return value, false, value != ""
		}
		if !item.IsObject() {
			continue
		}

		itemType := strings.TrimSpace(item.Get("type").String())
		if itemType == "additional_tools" {
			continue
		}
		role := strings.TrimSpace(item.Get("role").String())
		if role == "user" {
			return openAIMessagePrompt(item)
		}
		if itemType == "input_text" {
			value := strings.TrimSpace(item.Get("text").String())
			return value, false, value != ""
		}

		// A tool result or assistant item after the previous user turn means this
		// is a continuation, not a new image request. Do not walk back and replay
		// the previous user's image intent.
		return "", false, false
	}
	return "", false, false
}

func openAIMessagePrompt(message gjson.Result) (text string, hasInputImage bool, ok bool) {
	content := message.Get("content")
	if content.Type == gjson.String {
		value := strings.TrimSpace(content.String())
		return value, false, value != ""
	}
	if !content.IsArray() {
		value := strings.TrimSpace(message.Get("text").String())
		return value, false, value != ""
	}

	parts := make([]string, 0, len(content.Array()))
	content.ForEach(func(_, part gjson.Result) bool {
		partType := strings.TrimSpace(part.Get("type").String())
		switch partType {
		case "input_text", "text":
			if value := strings.TrimSpace(part.Get("text").String()); value != "" {
				parts = append(parts, value)
			}
		case "input_image", "image", "image_url":
			hasInputImage = true
		}
		return true
	})
	text = strings.TrimSpace(strings.Join(parts, "\n"))
	return text, hasInputImage, text != ""
}

func openAIUserPromptRequestsImage(prompt string, hasInputImage bool) bool {
	prompt = strings.TrimSpace(strings.ToLower(prompt))
	if prompt == "" {
		return false
	}

	for _, negative := range []string{
		"不要生成", "别生成", "不需要生成", "无需生成", "不用生成", "先别生成",
		"不要画", "别画", "不要绘制", "不要制作", "不要创建", "不要设计", "别做图",
		"do not generate", "don't generate", "dont generate", "never generate",
		"do not create", "don't create", "do not draw", "don't draw",
	} {
		if strings.Contains(prompt, negative) {
			return false
		}
	}

	for _, informationalPrefix := range []string{
		"如何生成", "怎么生成", "怎样生成", "如何画", "怎么画", "如何使用生图", "怎么使用生图",
		"how to generate", "how do i generate", "how can i generate", "how to create an image",
		"explain image generation", "write code to generate", "show me code to generate",
	} {
		if strings.HasPrefix(prompt, informationalPrefix) {
			return false
		}
	}
	if openAIEnglishImageHowToPattern.MatchString(prompt) || openAIChineseImageHowToPattern.MatchString(prompt) {
		return false
	}
	if strings.Contains(prompt, "生成图片的提示词") || strings.Contains(prompt, "生图提示词") ||
		strings.Contains(prompt, "image generation api") {
		return false
	}
	// Do not turn implementation/documentation work about image generation into
	// an actual paid generation. Native Codex can discuss or edit image tooling,
	// prompts, APIs and routing code with the passive image tool still present.
	// These narrow phrases cover that ambiguity while leaving direct imperatives
	// such as "生成一个 logo" and "create an image" untouched.
	for _, meta := range []string{
		"生成图片的代码", "生成图像的代码", "生成图片的函数", "生成图像的函数",
		"图片生成代码", "图像生成代码", "图片生成逻辑", "图像生成逻辑",
		"生图代码", "生图函数", "生图接口", "生图 api", "生图api", "生图 sdk", "生图sdk",
		"image generation code", "image generation function", "image generation logic",
		"image generation endpoint", "image generation sdk", "image generation docs",
		"code to generate an image", "code that generates an image",
		"code to create an image", "code that creates an image", "image generation prompt",
	} {
		if strings.Contains(prompt, meta) {
			return false
		}
	}

	for _, strong := range []string{
		"帮我生图", "给我生图", "直接生图", "开始生图", "生成一张图", "生成张图",
		"画一张图", "画张图", "给我画", "帮我画", "出一张图", "做一张图",
	} {
		if strings.Contains(prompt, strong) {
			return true
		}
	}

	chineseAction := containsAnyString(prompt, []string{
		"生成", "绘制", "画", "创作", "制作", "创建", "设计", "渲染", "重绘",
		"编辑", "修改", "改成", "换成", "变成",
	})
	chineseNoun := containsAnyString(prompt, []string{
		"图片", "图像", "一张图", "这张图", "那张图", "插图", "海报", "封面", "头像",
		"壁纸", "图标", "视觉稿", "艺术图", "照片", "logo",
		"风景图", "场景图", "效果图", "概念图", "示意图", "配图", "产品图", "人物图", "宣传图",
	})
	if chineseAction && (chineseNoun || hasInputImage) {
		return true
	}

	englishAction := openAIEnglishImageActionPattern.MatchString(prompt)
	englishNoun := openAIEnglishImageNounPattern.MatchString(prompt)
	return englishAction && (englishNoun || hasInputImage)
}

func containsAnyString(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

func openAIToolChoiceSelectsImageGeneration(choice gjson.Result) bool {
	if !choice.Exists() {
		return false
	}
	if choice.Type == gjson.String {
		return strings.TrimSpace(choice.String()) == "image_generation"
	}
	if !choice.IsObject() {
		return false
	}
	if strings.TrimSpace(choice.Get("type").String()) == "image_generation" {
		return true
	}
	if tool := choice.Get("tool"); tool.IsObject() {
		return openAIToolChoiceSelectsImageGeneration(tool)
	}
	return false
}

// OpenAIImageGenerationRoutingPriority returns an account's opt-in image route
// priority for the public requested model. Invalid configuration fails closed
// so an accidental extra value cannot redirect production traffic.
func (a *Account) OpenAIImageGenerationRoutingPriority(requestedModel string) (int, bool) {
	if a == nil || !a.IsOpenAI() || a.Extra == nil {
		return 0, false
	}
	rawPriority, ok := a.Extra[OpenAIImageGenerationPriorityExtraKey]
	if !ok {
		return 0, false
	}
	priority := ParseExtraInt(rawPriority)
	if priority <= 0 {
		return 0, false
	}

	transport, ok := a.OpenAIImageGenerationTransport(requestedModel)
	if !ok {
		return 0, false
	}
	rawModels, restricted := a.Extra[OpenAIImageGenerationModelsExtraKey]
	if restricted && !openAIImageGenerationModelAllowed(rawModels, requestedModel) {
		// Existing Responses image routes are restricted to their Responses
		// model (currently gpt-5.6-sol), while Codex's built-in ImageGen client
		// enters through /v1/images/generations as gpt-image-2. Treat that exact
		// model as an alias only for the Responses bridge. Native Images routes
		// still have to opt in to gpt-image-2 explicitly.
		if transport != OpenAIImageGenerationTransportResponses ||
			!strings.EqualFold(strings.TrimSpace(requestedModel), gptImageOnlyModel) ||
			!openAIImageGenerationModelAllowed(rawModels, CodexNativeImageBridgeModel()) {
			return 0, false
		}
	}
	return priority, true
}

// OpenAIImageGenerationTransport resolves the protocol used by a configured
// image route. Native Images routes fail closed unless the account is an
// image-capable API-key account and its model mapping resolves the public
// request model to a valid gpt-image model.
func (a *Account) OpenAIImageGenerationTransport(requestedModel string) (string, bool) {
	if a == nil || !a.IsOpenAI() || a.Extra == nil {
		return "", false
	}
	raw, exists := a.Extra[OpenAIImageGenerationTransportExtraKey]
	transport := OpenAIImageGenerationTransportResponses
	if exists {
		value, ok := raw.(string)
		if !ok {
			return "", false
		}
		transport = strings.ToLower(strings.TrimSpace(value))
	}
	switch transport {
	case OpenAIImageGenerationTransportResponses:
		return transport, true
	case OpenAIImageGenerationTransportImages:
		if !supportsOpenAIImages(a) {
			return "", false
		}
		mappedModel, matched := a.ResolveMappedModel(strings.TrimSpace(requestedModel))
		if !matched {
			return "", false
		}
		if err := validateOpenAIImagesModel(mappedModel); err != nil {
			return "", false
		}
		return transport, true
	default:
		return "", false
	}
}

// OmitOpenAIImageGenerationResponseFormat reports whether an account's native
// Images provider rejects response_format. Invalid values fail closed.
func (a *Account) OmitOpenAIImageGenerationResponseFormat() bool {
	if a == nil || a.Extra == nil {
		return false
	}
	value, ok := a.Extra[OpenAIImageGenerationOmitResponseFormatExtraKey].(bool)
	return ok && value
}

func openAIImageGenerationModelAllowed(raw any, requestedModel string) bool {
	requestedModel = strings.TrimSpace(requestedModel)
	if requestedModel == "" {
		return false
	}

	matches := func(pattern string) bool {
		pattern = strings.TrimSpace(pattern)
		return pattern != "" && matchWildcard(pattern, requestedModel)
	}

	switch values := raw.(type) {
	case []string:
		for _, value := range values {
			if matches(value) {
				return true
			}
		}
	case []any:
		for _, value := range values {
			pattern, ok := value.(string)
			if ok && matches(pattern) {
				return true
			}
		}
	}
	return false
}
