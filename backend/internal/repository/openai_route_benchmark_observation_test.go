package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestOpenAIRouteBenchmarkObservationRepositoryMapsEndpointAndBeijingHour(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repository := NewOpenAIRouteBenchmarkObservationRepository(db)
	now := time.Date(2026, 8, 15, 8, 30, 0, 0, time.UTC) // Beijing Saturday 16:30.
	endpoint := service.OpenAIRouteEndpointForBaseURL("https://hk.pomoai.xyz", "/v1/responses")
	key := service.OpenAIRouteKey{
		GroupID: 6, AccountID: 23, Model: "gpt-5.6-sol",
		RequestClass: service.OpenAIRouteRequestClassText,
		EndpointHash: service.OpenAIRouteEndpointHash(endpoint),
		Transport:    string(service.OpenAIUpstreamTransportHTTPSSE), FailureDomain: "pomoai:gpt-pro-0.2",
	}
	rows := sqlmock.NewRows([]string{
		"account_id", "model", "base_url", "sampled_at", "passed",
		"first_text_ms", "total_ms", "error_type", "stream_complete",
	})
	for i := 0; i < 20; i++ {
		rows.AddRow(int64(23), "gpt-5.6-sol", "https://hk.pomoai.xyz", now.Add(-time.Duration(i)*time.Minute), true, int64(500), int64(2_000), "", true)
	}
	mock.ExpectQuery("(?s)"+regexp.QuoteMeta("FROM base_url_benchmarks")).
		WithArgs(
			now.Add(-28*24*time.Hour), sqlmock.AnyArg(), sqlmock.AnyArg(),
			now.Add(-72*time.Hour), int(now.In(beijingFixedZone).Weekday()), now.In(beijingFixedZone).Hour(),
			openAIRouteBenchmarkMaximumRows+1,
		).
		WillReturnRows(rows)

	profiles, err := repository.GetBenchmarkBatch(context.Background(), []service.OpenAIRouteKey{key}, now)

	require.NoError(t, err)
	profile := profiles[service.OpenAIRouteObservationFingerprint(key)]
	require.Equal(t, uint64(20), profile.Global.ReliabilityCount)
	require.Equal(t, uint64(20), profile.Recent.ReliabilityCount)
	require.Equal(t, uint64(20), profile.HourOfWeek.ReliabilityCount)
	require.Equal(t, uint64(20), profile.Evidence.RawSamples)
	require.Equal(t, uint64(1), profile.Evidence.EffectiveSamples)
	require.Greater(t, profile.Evidence.Confidence, 0.0)
	require.LessOrEqual(t, profile.Evidence.Confidence, service.OpenAIRouteBenchmarkMaxConfidence)
	require.Equal(t, now, profile.Evidence.LastObservedAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAIRouteBenchmarkObservationRepositoryKeepsVariantsAndRequestClassesSeparate(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repository := NewOpenAIRouteBenchmarkObservationRepository(db)
	now := time.Date(2026, 8, 15, 8, 30, 0, 0, time.UTC)
	textKey := service.OpenAIRouteKey{
		GroupID: 6, AccountID: 23, Model: "gpt-5.6-sol", RequestClass: service.OpenAIRouteRequestClassText,
		EndpointHash: service.OpenAIRouteEndpointHash(service.OpenAIRouteEndpointForBaseURL("https://jp.pomoai.xyz", "/v1/responses")),
		Transport:    string(service.OpenAIUpstreamTransportHTTPSSE), FailureDomain: "pomoai:gpt-pro-0.2",
	}
	imageKey := textKey
	imageKey.RequestClass = service.OpenAIRouteRequestClassImage
	rows := sqlmock.NewRows([]string{
		"account_id", "model", "base_url", "sampled_at", "passed",
		"first_text_ms", "total_ms", "error_type", "stream_complete",
	}).AddRow(int64(23), "gpt-5.6-sol", "https://hk.pomoai.xyz", now, true, int64(400), int64(1_000), "", true)
	mock.ExpectQuery("(?s)" + regexp.QuoteMeta("FROM base_url_benchmarks")).WillReturnRows(rows)

	profiles, err := repository.GetBenchmarkBatch(context.Background(), []service.OpenAIRouteKey{textKey, imageKey}, now)

	require.NoError(t, err)
	require.Zero(t, profiles[service.OpenAIRouteObservationFingerprint(textKey)].Evidence.RawSamples, "HK evidence must not train the JP variant")
	require.Zero(t, profiles[service.OpenAIRouteObservationFingerprint(imageKey)].Evidence.RawSamples, "text probes must not train image routing")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAIRouteBenchmarkFailureClassIsBounded(t *testing.T) {
	tests := map[string]service.OpenAIRouteFailureClass{
		"rate_limited":      service.OpenAIRouteFailureRateLimit,
		"http_5xx":          service.OpenAIRouteFailureUpstream5xx,
		"authentication":    service.OpenAIRouteFailureAuthentication,
		"stream_incomplete": service.OpenAIRouteFailurePartialStream,
		"wrong_response":    service.OpenAIRouteFailureMalformedStream,
		"timeout":           service.OpenAIRouteFailureLocalTransport,
		"unknown":           service.OpenAIRouteFailureCapacity,
	}
	for raw, expected := range tests {
		t.Run(raw, func(t *testing.T) {
			require.Equal(t, expected, openAIRouteBenchmarkFailureClass(raw))
		})
	}

	failed := openAIRouteBenchmarkRowAggregate(time.Now(), false, "stream_incomplete", sql.NullInt64{}, sql.NullInt64{})
	require.Equal(t, uint64(1), failed.PartialStreams)
	require.Equal(t, uint64(1), failed.FailureCounts[service.OpenAIRouteFailurePartialStream])
}
