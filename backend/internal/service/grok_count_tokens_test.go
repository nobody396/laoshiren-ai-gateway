package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEstimateGrokCountTokens(t *testing.T) {
	for _, body := range []string{
		`{"model":"grok-4.5","messages":[{"role":"user","content":"hello"}]}`,
		`{"model":"grok-4.5","system":"be concise","messages":[{"role":"user","content":[{"type":"text","text":"hello"}]}],"tools":[{"name":"lookup","description":"lookup","input_schema":{"type":"object"}}]}`,
	} {
		got, err := EstimateGrokCountTokens([]byte(body))
		require.NoError(t, err)
		require.Positive(t, got)
	}
}

func TestEstimateGrokCountTokensRejectsInvalidRequest(t *testing.T) {
	for _, body := range []string{
		`{`,
		`{"messages":[{"role":"user","content":"hello"}]}`,
		`{"model":"grok-4.5","messages":[{"role":"user","content":{"unexpected":true}}]}`,
	} {
		_, err := EstimateGrokCountTokens([]byte(body))
		require.Error(t, err)
	}
}
