package service

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestNormalizeOpenAIResponsesRejectedStatusClearsWholeItemType(t *testing.T) {
	items := make([]string, 0, 11)
	for i := 0; i < 10; i++ {
		items = append(items, `{"type":"tool_search_output","status":"completed","call_id":"call_`+strconv.Itoa(i)+`"}`)
	}
	items = append(items, `{"type":"message","status":"completed","role":"user","content":"hi"}`)
	body := []byte(`{"input":[` + strings.Join(items, ",") + `]}`)
	errorBody := []byte(`{"error":{"code":"unknown_parameter","param":"input[7].status","message":"Unknown parameter"}}`)

	retry, changed, err := normalizeOpenAIResponsesRejectedStatusRetryBody(http.StatusBadRequest, body, errorBody)

	require.NoError(t, err)
	require.True(t, changed)
	for i := 0; i < 10; i++ {
		require.False(t, gjson.GetBytes(retry, "input."+strconv.Itoa(i)+".status").Exists())
	}
	require.Equal(t, "completed", gjson.GetBytes(retry, "input.10.status").String())
}

func TestOpenAIResponsesRejectedStatusRetryIsBoundedAndDeduplicated(t *testing.T) {
	initial := []byte(`{"input":[]}`)
	state := newOpenAIResponsesRejectedStatusRetryState(initial)
	require.False(t, state.Allow(initial))
	for i := 0; i < maxOpenAIResponsesRejectedStatusRetries; i++ {
		require.True(t, state.Allow([]byte(`{"attempt":`+strconv.Itoa(i)+`}`)))
	}
	require.False(t, state.Allow([]byte(`{"attempt":"overflow"}`)))
}

func TestNormalizeOpenAIResponsesRejectedStatusIgnoresAmbiguousErrors(t *testing.T) {
	body := []byte(`{"input":[{"type":"message","status":"completed"}]}`)
	for _, errorBody := range [][]byte{
		[]byte(`{"error":{"code":"invalid_request_error","param":"input[0].status"}}`),
		[]byte(`{"error":{"code":"unknown_parameter","param":"input[0].content"}}`),
	} {
		_, changed, err := normalizeOpenAIResponsesRejectedStatusRetryBody(http.StatusBadRequest, body, errorBody)
		require.NoError(t, err)
		require.False(t, changed)
	}
}
