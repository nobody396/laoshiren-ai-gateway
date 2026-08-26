package domain

// ReasoningEffortMapping rewrites one OpenAI/Codex reasoning effort value to
// another before the group ceiling is applied. The reserved source "default"
// applies when the request omits an effort, unless thinking was disabled.
type ReasoningEffortMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}
