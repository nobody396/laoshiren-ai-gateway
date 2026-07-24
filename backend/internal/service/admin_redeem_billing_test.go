//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRedeemBatchCreatedBy(t *testing.T) {
	t.Parallel()

	require.Nil(t, normalizeRedeemBatchCreatedBy(nil))

	neg := int64(-1)
	require.Nil(t, normalizeRedeemBatchCreatedBy(&neg))

	zero := int64(0)
	require.Nil(t, normalizeRedeemBatchCreatedBy(&zero))

	adminID := int64(1)
	got := normalizeRedeemBatchCreatedBy(&adminID)
	require.NotNil(t, got)
	require.Equal(t, int64(1), *got)
}
