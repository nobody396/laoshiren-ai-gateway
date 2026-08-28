//go:build unit

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestChannelPricingTimeRoundTrip(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &channelRepository{db: db}
	created := time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC)
	columns := []string{
		"id", "channel_id", "platform", "models", "billing_mode", "input_price", "output_price", "cache_write_price", "cache_read_price",
		"fast_multiplier", "flex_multiplier", "fast_supported", "flex_supported", "fast_verified_at", "flex_verified_at",
		"image_output_price", "per_request_price", "time_pricing", "created_at", "updated_at",
	}
	mock.ExpectQuery(`(?s)SELECT .*per_request_price, time_pricing, created_at, updated_at.*FROM channel_model_pricing.*channel_id = \$1`).
		WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows(columns).AddRow(
			int64(11), int64(7), "openai", `["gpt-5"]`, service.BillingModeToken,
			nil, nil, nil, nil, nil, nil, false, false, nil, nil, nil, nil,
			`{"timezone":"Asia/Shanghai","periods":[{"start_time":"09:00","end_time":"12:00","multiplier":2}]}`,
			created, created,
		))
	mock.ExpectQuery(`SELECT id, pricing_id, min_tokens, max_tokens, tier_label`).
		WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{
		"id", "pricing_id", "min_tokens", "max_tokens", "tier_label", "input_price", "output_price", "cache_write_price", "cache_read_price", "per_request_price", "sort_order", "created_at", "updated_at",
	}))

	pricing, err := repo.ListModelPricing(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, pricing, 1)
	require.NotNil(t, pricing[0].TimePricing)
	require.Equal(t, "Asia/Shanghai", pricing[0].TimePricing.Timezone)
	require.Equal(t, 2.0, pricing[0].TimePricing.Periods[0].Multiplier)
	require.NoError(t, mock.ExpectationsWereMet())
}
