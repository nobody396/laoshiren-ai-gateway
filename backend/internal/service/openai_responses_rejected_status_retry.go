package service

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const maxOpenAIResponsesRejectedStatusRetries = 6

var openAIResponsesRejectedStatusParamPattern = regexp.MustCompile(`(?i)^input\[(\d+)\]\.status$`)

type openAIResponsesRejectedStatusRetryState struct {
	budget *openAIResponsesRejectedStatusRetryBudget
	seen   map[[sha256.Size]byte]struct{}
}

type openAIResponsesRejectedStatusRetryBudget struct {
	mu       sync.Mutex
	attempts int
}

const openAIResponsesRejectedStatusRetryBudgetContextKey = "openai_responses_rejected_status_retry_budget"

func newOpenAIResponsesRejectedStatusRetryState(initial []byte) *openAIResponsesRejectedStatusRetryState {
	return newOpenAIResponsesRejectedStatusRetryStateWithBudget(initial, &openAIResponsesRejectedStatusRetryBudget{})
}

func openAIResponsesRejectedStatusRetryStateForRequest(c *gin.Context, initial []byte) *openAIResponsesRejectedStatusRetryState {
	var budget *openAIResponsesRejectedStatusRetryBudget
	if c != nil {
		if existing, ok := c.Get(openAIResponsesRejectedStatusRetryBudgetContextKey); ok {
			budget, _ = existing.(*openAIResponsesRejectedStatusRetryBudget)
		}
	}
	if budget == nil {
		budget = &openAIResponsesRejectedStatusRetryBudget{}
		if c != nil {
			c.Set(openAIResponsesRejectedStatusRetryBudgetContextKey, budget)
		}
	}
	return newOpenAIResponsesRejectedStatusRetryStateWithBudget(initial, budget)
}

func newOpenAIResponsesRejectedStatusRetryStateWithBudget(initial []byte, budget *openAIResponsesRejectedStatusRetryBudget) *openAIResponsesRejectedStatusRetryState {
	s := &openAIResponsesRejectedStatusRetryState{budget: budget, seen: make(map[[sha256.Size]byte]struct{}, maxOpenAIResponsesRejectedStatusRetries+1)}
	s.seen[sha256.Sum256(initial)] = struct{}{}
	return s
}

func (s *openAIResponsesRejectedStatusRetryState) Allow(body []byte) bool {
	if s == nil || s.budget == nil || len(body) == 0 {
		return false
	}
	hash := sha256.Sum256(body)
	if _, exists := s.seen[hash]; exists {
		return false
	}
	s.budget.mu.Lock()
	defer s.budget.mu.Unlock()
	if s.budget.attempts >= maxOpenAIResponsesRejectedStatusRetries {
		return false
	}
	s.seen[hash] = struct{}{}
	s.budget.attempts++
	return true
}

// normalizeOpenAIResponsesRejectedStatusRetryBody handles only an explicit
// upstream rejection of input[n].status. It removes status from every item of
// that same type in one bounded retry, while preserving other item types.
func normalizeOpenAIResponsesRejectedStatusRetryBody(statusCode int, body, responseBody []byte) ([]byte, bool, error) {
	if statusCode != http.StatusBadRequest || len(body) == 0 || len(responseBody) == 0 {
		return nil, false, nil
	}
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(responseBody)))
	if code != "unknown_parameter" && code != "unsupported_parameter" {
		return nil, false, nil
	}
	param := strings.ToLower(strings.TrimSpace(gjson.GetBytes(responseBody, "error.param").String()))
	match := openAIResponsesRejectedStatusParamPattern.FindStringSubmatch(param)
	if len(match) != 2 {
		return nil, false, nil
	}
	index, err := strconv.Atoi(match[1])
	if err != nil || index < 0 {
		return nil, false, nil
	}

	itemPath := fmt.Sprintf("input.%d", index)
	rejected := gjson.GetBytes(body, itemPath)
	if !rejected.IsObject() || !rejected.Get("status").Exists() {
		return nil, false, nil
	}

	retryBody := body
	rejectedType := strings.TrimSpace(rejected.Get("type").String())
	cleared := 0
	if input := gjson.GetBytes(body, "input"); rejectedType != "" && input.IsArray() {
		for itemIndex, item := range input.Array() {
			if !item.IsObject() || strings.TrimSpace(item.Get("type").String()) != rejectedType || !item.Get("status").Exists() {
				continue
			}
			next, deleteErr := sjson.DeleteBytes(retryBody, fmt.Sprintf("input.%d.status", itemIndex))
			if deleteErr != nil {
				return nil, false, fmt.Errorf("delete rejected status at input[%d]: %w", itemIndex, deleteErr)
			}
			retryBody = next
			cleared++
		}
	}
	if cleared == 0 {
		next, deleteErr := sjson.DeleteBytes(retryBody, itemPath+".status")
		if deleteErr != nil {
			return nil, false, fmt.Errorf("delete rejected status at input[%d]: %w", index, deleteErr)
		}
		retryBody = next
	}
	return retryBody, true, nil
}
