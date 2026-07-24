package admin

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOptionalLimitField_OmittedVsNullVsNumber(t *testing.T) {
	t.Parallel()

	type payload struct {
		Daily optionalLimitField `json:"daily_limit_usd"`
	}

	// Omitted: not set, service input nil (preserve on update).
	var omitted payload
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.False(t, omitted.Daily.IsSet())
	require.Nil(t, omitted.Daily.ToServiceInput())

	// Explicit null: unlimited sentinel (-1).
	var asNull payload
	require.NoError(t, json.Unmarshal([]byte(`{"daily_limit_usd":null}`), &asNull))
	require.True(t, asNull.Daily.IsSet())
	require.NotNil(t, asNull.Daily.ToServiceInput())
	require.InDelta(t, -1.0, *asNull.Daily.ToServiceInput(), 0.0001)

	// Positive limit.
	var positive payload
	require.NoError(t, json.Unmarshal([]byte(`{"daily_limit_usd":450}`), &positive))
	require.True(t, positive.Daily.IsSet())
	require.NotNil(t, positive.Daily.ToServiceInput())
	require.InDelta(t, 450.0, *positive.Daily.ToServiceInput(), 0.0001)

	// Zero quota.
	var zero payload
	require.NoError(t, json.Unmarshal([]byte(`{"daily_limit_usd":0}`), &zero))
	require.True(t, zero.Daily.IsSet())
	require.NotNil(t, zero.Daily.ToServiceInput())
	require.InDelta(t, 0.0, *zero.Daily.ToServiceInput(), 0.0001)
}

func TestOptionalStringField_OmittedVsEmptyVsText(t *testing.T) {
	t.Parallel()

	type payload struct {
		Description optionalStringField `json:"description"`
	}

	var omitted payload
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.False(t, omitted.Description.IsSet())
	require.Nil(t, omitted.Description.ToServicePointer())

	var empty payload
	require.NoError(t, json.Unmarshal([]byte(`{"description":""}`), &empty))
	require.True(t, empty.Description.IsSet())
	require.NotNil(t, empty.Description.ToServicePointer())
	require.Equal(t, "", *empty.Description.ToServicePointer())

	var text payload
	require.NoError(t, json.Unmarshal([]byte(`{"description":"hello"}`), &text))
	require.True(t, text.Description.IsSet())
	require.NotNil(t, text.Description.ToServicePointer())
	require.Equal(t, "hello", *text.Description.ToServicePointer())

	var asNull payload
	require.NoError(t, json.Unmarshal([]byte(`{"description":null}`), &asNull))
	require.True(t, asNull.Description.IsSet())
	require.NotNil(t, asNull.Description.ToServicePointer())
	require.Equal(t, "", *asNull.Description.ToServicePointer())
}
