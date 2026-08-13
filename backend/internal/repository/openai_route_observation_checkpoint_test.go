package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type openAIRouteObservationStoreTestStub struct {
	checkErr      error
	recordErr     error
	recordCostErr error
	getErr        error
	records       int
	costs         int
	profiles      map[string]service.OpenAIRouteObservationProfile
}

func (s *openAIRouteObservationStoreTestStub) Check(context.Context) error { return s.checkErr }
func (s *openAIRouteObservationStoreTestStub) Record(context.Context, service.OpenAIRouteObservation) error {
	s.records++
	return s.recordErr
}
func (s *openAIRouteObservationStoreTestStub) RecordCost(context.Context, service.OpenAIRouteActualCostObservation) error {
	s.costs++
	return s.recordCostErr
}
func (s *openAIRouteObservationStoreTestStub) GetBatch(context.Context, []service.OpenAIRouteKey, time.Time) (map[string]service.OpenAIRouteObservationProfile, error) {
	return s.profiles, s.getErr
}

func TestOpenAIRouteObservationCheckpoint_RecordAggregatesSparseMetrics(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repository := NewOpenAIRouteObservationCheckpointRepository(db)
	key := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText)
	now := time.Date(2026, 8, 12, 13, 37, 0, 0, time.UTC)
	mock.ExpectExec("(?s)"+regexp.QuoteMeta("INSERT INTO openai_route_observation_hourly")).
		WithArgs(
			service.OpenAIRouteObservationFingerprint(key), now.Truncate(time.Hour),
			key.GroupID, key.AccountID, key.FailureDomain, key.Model, string(key.RequestClass), key.EndpointHash, key.Transport,
			sqlmock.AnyArg(), 0.12, 0.024, now,
		).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = repository.Record(context.Background(), service.OpenAIRouteObservation{
		Key: key, ObservedAt: now, Success: true, FailureClass: service.OpenAIRouteFailureNone,
		TTFTMilliseconds: 820, CompletionLatencyMS: 4_100,
		ActualBaseCostUSD: 0.12, ActualAccountCostUSD: 0.024, ActualCostAuthoritative: true,
	})

	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAIRouteObservationStore_AttemptsDurableWriteWhenRedisFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	cacheErr := errors.New("redis unavailable")
	cache := &openAIRouteObservationStoreTestStub{recordErr: cacheErr}
	store := NewOpenAIRouteObservationStore(cache, db)
	observation := service.OpenAIRouteObservation{
		Key:        testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText),
		ObservedAt: time.Date(2026, 8, 12, 13, 37, 0, 0, time.UTC),
		Success:    true, FailureClass: service.OpenAIRouteFailureNone,
	}
	mock.ExpectExec("(?s)" + regexp.QuoteMeta("INSERT INTO openai_route_observation_hourly")).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = store.Record(context.Background(), observation)

	require.ErrorIs(t, err, cacheErr)
	require.Equal(t, 1, cache.records)
	require.NoError(t, mock.ExpectationsWereMet(), "PostgreSQL must still receive the checkpoint")
}

func TestOpenAIRouteObservationStore_ReplacesLongWindowsWithoutDoubleCounting(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	key := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText)
	fingerprint := service.OpenAIRouteObservationFingerprint(key)
	now := time.Date(2026, 8, 12, 13, 30, 0, 0, time.UTC)
	cacheProfile := newOpenAIRouteObservationProfile()
	cacheProfile.Recent.AttemptCount = 3
	cacheProfile.Global.AttemptCount = 99
	cacheProfile.HourOfWeek.AttemptCount = 99
	cache := &openAIRouteObservationStoreTestStub{profiles: map[string]service.OpenAIRouteObservationProfile{fingerprint: cacheProfile}}
	store := NewOpenAIRouteObservationStore(cache, db)
	rows := sqlmock.NewRows([]string{
		"route_fingerprint", "hour_start", "metrics", "actual_base_cost_usd", "actual_account_cost_usd", "last_observed_at",
	}).AddRow(fingerprint, now.Truncate(time.Hour), []byte(`{"attempt_count":2,"reliability_count":2,"success_count":2}`), "0", "0", now)
	mock.ExpectQuery("(?s)" + regexp.QuoteMeta("FROM openai_route_observation_hourly")).WillReturnRows(rows)

	profiles, err := store.GetBatch(context.Background(), []service.OpenAIRouteKey{key}, now)

	require.NoError(t, err)
	profile := profiles[fingerprint]
	require.Equal(t, uint64(3), profile.Recent.AttemptCount)
	require.Equal(t, uint64(2), profile.Global.AttemptCount)
	require.Equal(t, uint64(2), profile.HourOfWeek.AttemptCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAIRouteObservationStore_RecoversLongWindowsWhenRedisReadFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	key := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText)
	fingerprint := service.OpenAIRouteObservationFingerprint(key)
	now := time.Date(2026, 8, 12, 13, 30, 0, 0, time.UTC)
	cache := &openAIRouteObservationStoreTestStub{getErr: errors.New("redis unavailable")}
	store := NewOpenAIRouteObservationStore(cache, db)
	rows := sqlmock.NewRows([]string{
		"route_fingerprint", "hour_start", "metrics", "actual_base_cost_usd", "actual_account_cost_usd", "last_observed_at",
	}).AddRow(fingerprint, now.Truncate(time.Hour), []byte(`{"attempt_count":2,"reliability_count":2,"success_count":2}`), "0", "0", now)
	mock.ExpectQuery("(?s)" + regexp.QuoteMeta("FROM openai_route_observation_hourly")).WillReturnRows(rows)

	profiles, err := store.GetBatch(context.Background(), []service.OpenAIRouteKey{key}, now)

	require.NoError(t, err)
	profile := profiles[fingerprint]
	require.Zero(t, profile.Recent.AttemptCount)
	require.Equal(t, uint64(2), profile.Global.AttemptCount)
	require.Equal(t, uint64(2), profile.HourOfWeek.AttemptCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOpenAIRouteObservationCheckpoint_GetBatchReturnsRowsCloseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repository := NewOpenAIRouteObservationCheckpointRepository(db)
	key := testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText)
	now := time.Date(2026, 8, 12, 13, 30, 0, 0, time.UTC)
	closeErr := errors.New("rows close failed")
	rows := sqlmock.NewRows([]string{
		"route_fingerprint", "hour_start", "metrics", "actual_base_cost_usd", "actual_account_cost_usd", "last_observed_at",
	}).AddRow(
		"unexpected-fingerprint",
		now.Truncate(time.Hour),
		[]byte(`{"attempt_count":1,"reliability_count":1,"success_count":1}`),
		"0",
		"0",
		now,
	).CloseError(closeErr)
	mock.ExpectQuery("(?s)" + regexp.QuoteMeta("FROM openai_route_observation_hourly")).WillReturnRows(rows)

	_, err = repository.GetBatch(context.Background(), []service.OpenAIRouteKey{key}, now)

	require.ErrorContains(t, err, "unexpected OpenAI route checkpoint fingerprint")
	require.ErrorIs(t, err, closeErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDecodeOpenAIRouteCheckpointAggregateRejectsFractionalCounter(t *testing.T) {
	_, err := decodeOpenAIRouteCheckpointAggregate([]byte(`{"attempt_count":1.5}`), "0", "0", time.Now())
	require.Error(t, err)
}

func TestOpenAIRouteObservationMetricsRejectsUnboundedFailureKey(t *testing.T) {
	observation := service.OpenAIRouteObservation{
		Key:          testOpenAIRouteObservationKey(service.OpenAIRouteRequestClassText),
		FailureClass: service.OpenAIRouteFailureClass("invalid:key"),
	}
	require.NoError(t, observation.Validate(), "service validation intentionally accepts future classified values")
	_, err := openAIRouteObservationMetrics(observation)
	require.ErrorContains(t, err, "invalid OpenAI route checkpoint failure class")
}
