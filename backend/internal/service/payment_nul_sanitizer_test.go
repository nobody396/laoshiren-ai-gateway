package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRemovePostgresTextNUL(t *testing.T) {
	value := "trade\x00-no\x00"

	got := removePostgresTextNUL(value)

	require.Equal(t, "trade-no", got)
	require.False(t, strings.ContainsRune(got, 0))
	require.Equal(t, "unchanged", removePostgresTextNUL("unchanged"))
}
