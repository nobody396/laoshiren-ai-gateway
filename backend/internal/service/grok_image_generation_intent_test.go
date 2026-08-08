//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsExplicitGrokImageGenerationIntent(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "native image tool",
			body: `{"model":"grok-4.5","tools":[{"type":"image_generation"}],"input":"draw"}`,
			want: true,
		},
		{
			name: "explicit namespace tool choice",
			body: `{"model":"grok-4.5","tools":[{"type":"namespace","name":"image_gen"}],"tool_choice":{"type":"namespace","name":"image_gen"}}`,
			want: true,
		},
		{
			name: "passive namespace catalog",
			body: `{"model":"grok-4.5","tools":[{"type":"namespace","name":"image_gen","tools":[{"type":"function","name":"imagegen"}]}],"tool_choice":"auto"}`,
			want: false,
		},
		{
			name: "responses lite passive additional tools",
			body: `{"model":"grok-4.5","tool_choice":"auto","input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"image_gen"}]}]}`,
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsExplicitGrokImageGenerationIntent("grok-4.5", []byte(tt.body)))
		})
	}
	require.True(t, IsExplicitGrokImageGenerationIntent("grok-imagine-image", []byte(`{"model":"grok-imagine-image"}`)))
}
