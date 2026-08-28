package apicompat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ChatCompletionsResponseToResponses converts a buffered Chat Completions
// response into the Responses wire shape.
func ChatCompletionsResponseToResponses(resp *ChatCompletionsResponse, model string) *ResponsesResponse {
	if resp == nil {
		return nil
	}
	state := NewChatChunkToResponsesState(model)
	state.ID = responsesIDFromChatID(resp.ID)
	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		if reasoning := choice.Message.reasoningText(); reasoning != "" {
			state.reasoning.WriteString(reasoning)
			state.ensureReasoningOutput(nil)
		}
		if len(choice.Message.Content) > 0 {
			var content string
			if json.Unmarshal(choice.Message.Content, &content) == nil && content != "" {
				state.text.WriteString(content)
				state.ensureMessageOutput(nil)
			}
		}
		for index, call := range choice.Message.ToolCalls {
			idx := index
			if call.Index != nil {
				idx = *call.Index
			}
			tool := state.ensureToolOutput(idx, call.ID, call.Function.Name, nil)
			tool.arguments.WriteString(call.Function.Arguments)
		}
		state.finishReason = choice.FinishReason
	}
	state.usage = responsesUsageFromChatUsage(resp.Usage)
	return state.finalResponse()
}

type chatResponsesOutputRef struct {
	kind      string
	toolIndex int
	index     int
}

type chatResponsesToolState struct {
	outputIndex int
	itemID      string
	callID      string
	name        string
	arguments   strings.Builder
}

// ChatChunkToResponsesState accumulates a Chat Completions SSE stream while
// emitting protocol-correct Responses deltas.
type ChatChunkToResponsesState struct {
	ID                 string
	Model              string
	CreatedAt          int64
	Sequence           int
	CreatedEmitted     bool
	Finalized          bool
	messageOutputID    string
	messageOutputIdx   int
	reasoningOutputID  string
	reasoningOutputIdx int
	text               strings.Builder
	reasoning          strings.Builder
	tools              map[int]*chatResponsesToolState
	outputOrder        []chatResponsesOutputRef
	finishReason       string
	usage              *ResponsesUsage
}

func NewChatChunkToResponsesState(model string) *ChatChunkToResponsesState {
	return &ChatChunkToResponsesState{
		ID:                 generateResponsesID("resp"),
		Model:              model,
		CreatedAt:          time.Now().Unix(),
		messageOutputIdx:   -1,
		reasoningOutputIdx: -1,
		tools:              make(map[int]*chatResponsesToolState),
	}
}

// ChatChunkToResponsesEvents converts one Chat streaming chunk into zero or
// more Responses events. Call FinalizeChatResponsesStream after [DONE]/EOF.
func ChatChunkToResponsesEvents(chunk *ChatCompletionsChunk, state *ChatChunkToResponsesState) []ResponsesStreamEvent {
	if chunk == nil || state == nil || state.Finalized {
		return nil
	}
	if chunk.ID != "" && !state.CreatedEmitted {
		state.ID = responsesIDFromChatID(chunk.ID)
	}
	if state.Model == "" && chunk.Model != "" {
		state.Model = chunk.Model
	}

	var events []ResponsesStreamEvent
	state.ensureCreated(&events)
	if chunk.Usage != nil {
		state.usage = responsesUsageFromChatUsage(chunk.Usage)
	}
	if len(chunk.Choices) == 0 {
		return events
	}
	choice := chunk.Choices[0]
	if choice.Delta.ReasoningContent != nil && *choice.Delta.ReasoningContent != "" {
		state.ensureReasoningOutput(&events)
		state.reasoning.WriteString(*choice.Delta.ReasoningContent)
		events = append(events, state.event(ResponsesStreamEvent{
			Type: "response.reasoning_summary_text.delta", OutputIndex: state.reasoningOutputIdx,
			ItemID: state.reasoningOutputID, SummaryIndex: 0, Delta: *choice.Delta.ReasoningContent,
		}))
	}
	if choice.Delta.Content != nil && *choice.Delta.Content != "" {
		state.ensureMessageOutput(&events)
		state.text.WriteString(*choice.Delta.Content)
		events = append(events, state.event(ResponsesStreamEvent{
			Type: "response.output_text.delta", OutputIndex: state.messageOutputIdx,
			ContentIndex: 0, ItemID: state.messageOutputID, Delta: *choice.Delta.Content,
		}))
	}
	for _, call := range choice.Delta.ToolCalls {
		idx := 0
		if call.Index != nil {
			idx = *call.Index
		}
		tool := state.ensureToolOutput(idx, call.ID, call.Function.Name, &events)
		if call.ID != "" {
			tool.callID = call.ID
		}
		if call.Function.Name != "" {
			tool.name = call.Function.Name
		}
		if call.Function.Arguments != "" {
			tool.arguments.WriteString(call.Function.Arguments)
			events = append(events, state.event(ResponsesStreamEvent{
				Type: "response.function_call_arguments.delta", OutputIndex: tool.outputIndex,
				ItemID: tool.itemID, CallID: tool.callID, Delta: call.Function.Arguments,
			}))
		}
	}
	if choice.FinishReason != nil && *choice.FinishReason != "" {
		state.finishReason = *choice.FinishReason
	}
	return events
}

func FinalizeChatResponsesStream(state *ChatChunkToResponsesState) []ResponsesStreamEvent {
	if state == nil || state.Finalized {
		return nil
	}
	state.Finalized = true
	var events []ResponsesStreamEvent
	state.ensureCreated(&events)
	for _, ref := range state.outputOrder {
		switch ref.kind {
		case "reasoning":
			text := state.reasoning.String()
			events = append(events, state.event(ResponsesStreamEvent{
				Type: "response.reasoning_summary_text.done", OutputIndex: ref.index,
				ItemID: state.reasoningOutputID, SummaryIndex: 0, Text: text,
			}))
			item := state.outputForRef(ref)
			events = append(events, state.event(ResponsesStreamEvent{Type: "response.output_item.done", OutputIndex: ref.index, Item: &item}))
		case "message":
			text := state.text.String()
			events = append(events, state.event(ResponsesStreamEvent{
				Type: "response.output_text.done", OutputIndex: ref.index,
				ContentIndex: 0, ItemID: state.messageOutputID, Text: text,
			}))
			part := ResponsesContentPart{Type: "output_text", Text: text}
			events = append(events, state.event(ResponsesStreamEvent{
				Type: "response.content_part.done", OutputIndex: ref.index,
				ContentIndex: 0, ItemID: state.messageOutputID, Part: &part,
			}))
			item := state.outputForRef(ref)
			events = append(events, state.event(ResponsesStreamEvent{Type: "response.output_item.done", OutputIndex: ref.index, Item: &item}))
		case "tool":
			tool := state.tools[ref.toolIndex]
			events = append(events, state.event(ResponsesStreamEvent{
				Type: "response.function_call_arguments.done", OutputIndex: ref.index,
				ItemID: tool.itemID, CallID: tool.callID, Name: tool.name, Arguments: tool.arguments.String(),
			}))
			item := state.outputForRef(ref)
			events = append(events, state.event(ResponsesStreamEvent{Type: "response.output_item.done", OutputIndex: ref.index, Item: &item}))
		}
	}
	response := state.finalResponse()
	eventType := "response.completed"
	if response.Status == "incomplete" {
		eventType = "response.incomplete"
	}
	events = append(events, state.event(ResponsesStreamEvent{Type: eventType, Response: response}))
	return events
}

func (s *ChatChunkToResponsesState) ensureCreated(events *[]ResponsesStreamEvent) {
	if s.CreatedEmitted {
		return
	}
	s.CreatedEmitted = true
	response := &ResponsesResponse{ID: s.ID, Object: "response", Model: s.Model, Status: "in_progress", Output: []ResponsesOutput{}}
	*events = append(*events, s.event(ResponsesStreamEvent{Type: "response.created", Response: response}))
}

func (s *ChatChunkToResponsesState) ensureReasoningOutput(events *[]ResponsesStreamEvent) {
	if s.reasoningOutputIdx >= 0 {
		return
	}
	s.reasoningOutputID = generateResponsesID("rs")
	s.reasoningOutputIdx = len(s.outputOrder)
	ref := chatResponsesOutputRef{kind: "reasoning", index: s.reasoningOutputIdx}
	s.outputOrder = append(s.outputOrder, ref)
	if events != nil {
		item := s.addedOutputForRef(ref)
		*events = append(*events, s.event(ResponsesStreamEvent{Type: "response.output_item.added", OutputIndex: ref.index, Item: &item}))
		part := ResponsesContentPart{Type: "output_text"}
		*events = append(*events, s.event(ResponsesStreamEvent{
			Type: "response.content_part.added", OutputIndex: ref.index,
			ContentIndex: 0, ItemID: s.messageOutputID, Part: &part,
		}))
	}
}

func (s *ChatChunkToResponsesState) ensureMessageOutput(events *[]ResponsesStreamEvent) {
	if s.messageOutputIdx >= 0 {
		return
	}
	s.messageOutputID = generateResponsesID("msg")
	s.messageOutputIdx = len(s.outputOrder)
	ref := chatResponsesOutputRef{kind: "message", index: s.messageOutputIdx}
	s.outputOrder = append(s.outputOrder, ref)
	if events != nil {
		item := s.addedOutputForRef(ref)
		*events = append(*events, s.event(ResponsesStreamEvent{Type: "response.output_item.added", OutputIndex: ref.index, Item: &item}))
	}
}

func (s *ChatChunkToResponsesState) ensureToolOutput(toolIndex int, callID, name string, events *[]ResponsesStreamEvent) *chatResponsesToolState {
	if tool := s.tools[toolIndex]; tool != nil {
		return tool
	}
	if callID == "" {
		callID = generateResponsesID("call")
	}
	tool := &chatResponsesToolState{
		outputIndex: len(s.outputOrder), itemID: generateResponsesID("fc"), callID: callID, name: name,
	}
	s.tools[toolIndex] = tool
	ref := chatResponsesOutputRef{kind: "tool", toolIndex: toolIndex, index: tool.outputIndex}
	s.outputOrder = append(s.outputOrder, ref)
	if events != nil {
		item := s.addedOutputForRef(ref)
		*events = append(*events, s.event(ResponsesStreamEvent{Type: "response.output_item.added", OutputIndex: ref.index, Item: &item}))
	}
	return tool
}

func (s *ChatChunkToResponsesState) addedOutputForRef(ref chatResponsesOutputRef) ResponsesOutput {
	item := s.outputForRef(ref)
	item.Status = "in_progress"
	item.Content = nil
	item.Summary = nil
	item.Arguments = ""
	return item
}

func (s *ChatChunkToResponsesState) outputForRef(ref chatResponsesOutputRef) ResponsesOutput {
	switch ref.kind {
	case "reasoning":
		item := ResponsesOutput{Type: "reasoning", ID: s.reasoningOutputID, Status: "completed"}
		if text := s.reasoning.String(); text != "" {
			item.Summary = []ResponsesSummary{{Type: "summary_text", Text: text}}
		}
		return item
	case "message":
		item := ResponsesOutput{Type: "message", ID: s.messageOutputID, Role: "assistant", Status: "completed"}
		if text := s.text.String(); text != "" {
			item.Content = []ResponsesContentPart{{Type: "output_text", Text: text}}
		}
		return item
	default:
		tool := s.tools[ref.toolIndex]
		return ResponsesOutput{Type: "function_call", ID: tool.itemID, Status: "completed", CallID: tool.callID, Name: tool.name, Arguments: tool.arguments.String()}
	}
}

func (s *ChatChunkToResponsesState) finalResponse() *ResponsesResponse {
	status := "completed"
	var incomplete *ResponsesIncompleteDetails
	if s.finishReason == "length" {
		status = "incomplete"
		incomplete = &ResponsesIncompleteDetails{Reason: "max_output_tokens"}
	}
	outputs := make([]ResponsesOutput, 0, len(s.outputOrder))
	for _, ref := range s.outputOrder {
		outputs = append(outputs, s.outputForRef(ref))
	}
	return &ResponsesResponse{
		ID: s.ID, Object: "response", Model: s.Model, Status: status,
		Output: outputs, Usage: s.usage, IncompleteDetails: incomplete,
	}
}

func (s *ChatChunkToResponsesState) event(event ResponsesStreamEvent) ResponsesStreamEvent {
	event.SequenceNumber = s.Sequence
	s.Sequence++
	return event
}

func responsesUsageFromChatUsage(usage *ChatUsage) *ResponsesUsage {
	if usage == nil {
		return nil
	}
	out := &ResponsesUsage{InputTokens: usage.PromptTokens, OutputTokens: usage.CompletionTokens, TotalTokens: usage.TotalTokens}
	if out.TotalTokens == 0 {
		out.TotalTokens = out.InputTokens + out.OutputTokens
	}
	if usage.PromptTokensDetails != nil {
		out.InputTokensDetails = &ResponsesInputTokensDetails{
			CachedTokens: usage.PromptTokensDetails.CachedTokens,
			AudioTokens:  usage.PromptTokensDetails.AudioTokens,
		}
	}
	if usage.CompletionTokensDetails != nil {
		out.OutputTokensDetails = &ResponsesOutputTokensDetails{
			ReasoningTokens:          usage.CompletionTokensDetails.ReasoningTokens,
			AudioTokens:              usage.CompletionTokensDetails.AudioTokens,
			AcceptedPredictionTokens: usage.CompletionTokensDetails.AcceptedPredictionTokens,
			RejectedPredictionTokens: usage.CompletionTokensDetails.RejectedPredictionTokens,
		}
	}
	return out
}

func ResponsesEventToSSE(event ResponsesStreamEvent) (string, error) {
	payload, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, payload), nil
}

func responsesIDFromChatID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return generateResponsesID("resp")
	}
	id = strings.TrimPrefix(id, "chatcmpl-")
	return "resp_" + id
}

func generateResponsesID(prefix string) string {
	id := strings.TrimPrefix(generateChatCmplID(), "chatcmpl-")
	return prefix + "_" + id
}
