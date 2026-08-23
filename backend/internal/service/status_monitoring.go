package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

const (
	channelMonitoringDefaultWindow = 15 * time.Minute
	channelMonitoringMaxWindow     = 2 * time.Hour
)

type ChannelMonitoringEvidence struct {
	FactType            ReliabilityFactType `json:"fact_type"`
	Platform            string              `json:"platform"`
	Model               string              `json:"model"`
	RequestClass        string              `json:"request_class"`
	Protocol            string              `json:"protocol"`
	GroupID             *int64              `json:"group_id,omitempty"`
	GroupName           string              `json:"group_name,omitempty"`
	AccountID           *int64              `json:"account_id,omitempty"`
	AccountName         string              `json:"account_name,omitempty"`
	RouteFingerprint    string              `json:"route_fingerprint,omitempty"`
	SampleCount         int64               `json:"sample_count"`
	SuccessCount        int64               `json:"success_count"`
	FailureCount        int64               `json:"failure_count"`
	RecoveredCount      int64               `json:"recovered_count"`
	CustomerImpactCount int64               `json:"customer_impact_count"`
	Availability        *float64            `json:"availability,omitempty"`
	AverageLatencyMs    float64             `json:"average_latency_ms"`
	P95LatencyMs        float64             `json:"p95_latency_ms"`
	SamplesPerMinute    float64             `json:"samples_per_minute"`
	LastObservedAt      time.Time           `json:"last_observed_at"`
	LastFailureAt       *time.Time          `json:"last_failure_at,omitempty"`
	LastSuccessAt       *time.Time          `json:"last_success_at,omitempty"`
	LastRecoveryAt      *time.Time          `json:"last_recovery_at,omitempty"`
	ProductCodes        []string            `json:"product_codes"`
	ComponentCodes      []string            `json:"component_codes"`
}

type ChannelMonitoringTotals struct {
	SampleCount          int64 `json:"sample_count"`
	FailureCount         int64 `json:"failure_count"`
	CustomerRequestCount int64 `json:"customer_request_count"`
	CustomerSuccessCount int64 `json:"customer_success_count"`
}

type ChannelMonitoringSnapshot struct {
	GeneratedAt         time.Time                       `json:"generated_at"`
	WindowStart         time.Time                       `json:"window_start"`
	WindowEnd           time.Time                       `json:"window_end"`
	WindowMinutes       int                             `json:"window_minutes"`
	Completeness        ReliabilityEvidenceCompleteness `json:"completeness"`
	Status              *AdminStatusSnapshot            `json:"status"`
	Evidence            []ChannelMonitoringEvidence     `json:"evidence"`
	EvidenceBucketTotal int64                           `json:"evidence_bucket_total"`
	EvidenceTruncated   bool                            `json:"evidence_truncated"`
	Totals              ChannelMonitoringTotals         `json:"totals"`
}

func (s *StatusControlService) MonitoringSnapshot(ctx context.Context, window time.Duration) (*ChannelMonitoringSnapshot, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("channel monitoring database is unavailable")
	}
	if window <= 0 {
		window = channelMonitoringDefaultWindow
	}
	if window > channelMonitoringMaxWindow {
		return nil, fmt.Errorf("channel monitoring window exceeds two hours")
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	end := time.Now()
	start := end.Add(-window)
	status, err := s.AdminSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	definitions, err := s.loadStatusDefinitions(ctx)
	if err != nil {
		return nil, err
	}
	evidence, totals, bucketTotal, err := s.loadChannelMonitoringEvidence(ctx, start, end)
	if err != nil {
		return nil, err
	}
	projectChannelMonitoringEvidence(definitions, evidence)
	completeness := ReliabilityEvidenceCompleteness{}
	if s.evidence != nil {
		completeness = s.evidence.Completeness()
	}
	return &ChannelMonitoringSnapshot{
		GeneratedAt: end, WindowStart: start, WindowEnd: end, WindowMinutes: int(window.Minutes()),
		Completeness: completeness, Status: status, Evidence: evidence, Totals: totals,
		EvidenceBucketTotal: bucketTotal, EvidenceTruncated: bucketTotal > int64(len(evidence)),
	}, nil
}

func (s *StatusControlService) loadChannelMonitoringEvidence(ctx context.Context, start, end time.Time) ([]ChannelMonitoringEvidence, ChannelMonitoringTotals, int64, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, ChannelMonitoringTotals{}, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SET LOCAL statement_timeout = '5s'`); err != nil {
		return nil, ChannelMonitoringTotals{}, 0, err
	}
	rows, err := tx.QueryContext(ctx, `
WITH buckets AS (
 SELECT o.fact_type,o.group_id,COALESCE(g.name,'') AS group_name,o.account_id,COALESCE(a.name,'') AS account_name,
  o.route_fingerprint,o.platform,o.model,o.request_class,o.protocol,
  COUNT(*)::bigint AS sample_count,
  COUNT(*) FILTER (WHERE o.outcome='success')::bigint AS success_count,
  COUNT(*) FILTER (WHERE o.outcome='failure')::bigint AS failure_count,
  COUNT(*) FILTER (WHERE o.outcome='recovered')::bigint AS recovered_count,
  COUNT(*) FILTER (WHERE o.customer_impact=TRUE)::bigint AS customer_impact_count,
  COALESCE(AVG(o.latency_ms),0)::double precision AS average_latency_ms,
  COALESCE(percentile_cont(0.95) WITHIN GROUP (ORDER BY o.latency_ms),0)::double precision AS p95_latency_ms,
  MAX(o.observed_at) AS last_observed_at,
  MAX(o.observed_at) FILTER (WHERE o.outcome='failure') AS last_failure_at,
  MAX(o.observed_at) FILTER (WHERE o.outcome IN ('success','recovered')) AS last_success_at,
  MAX(o.observed_at) FILTER (WHERE o.outcome='recovered') AS last_recovery_at
 FROM reliability_observations o
 LEFT JOIN groups g ON g.id=o.group_id
 LEFT JOIN accounts a ON a.id=o.account_id
 WHERE o.observed_at >= $1 AND o.observed_at < $2 AND o.outcome <> 'excluded'
 GROUP BY o.fact_type,o.group_id,g.name,o.account_id,a.name,o.route_fingerprint,o.platform,o.model,o.request_class,o.protocol
)
SELECT buckets.*,
 COUNT(*) OVER()::bigint AS total_buckets,
 COALESCE(SUM(sample_count) OVER(),0)::bigint AS total_samples,
 COALESCE(SUM(failure_count) OVER(),0)::bigint AS total_failures,
 COALESCE(SUM(sample_count) FILTER (WHERE fact_type='customer_request') OVER(),0)::bigint AS total_customer_requests,
 COALESCE(SUM(success_count+recovered_count) FILTER (WHERE fact_type='customer_request') OVER(),0)::bigint AS total_customer_successes
FROM buckets
ORDER BY last_observed_at DESC
LIMIT 2000`, start, end)
	if err != nil {
		return nil, ChannelMonitoringTotals{}, 0, err
	}
	defer func() { _ = rows.Close() }()
	result := make([]ChannelMonitoringEvidence, 0)
	totals := ChannelMonitoringTotals{}
	var bucketTotal int64
	minutes := end.Sub(start).Minutes()
	for rows.Next() {
		var item ChannelMonitoringEvidence
		var factType string
		var groupID, accountID sql.NullInt64
		var lastFailure, lastSuccess, lastRecovery sql.NullTime
		if err := rows.Scan(
			&factType, &groupID, &item.GroupName, &accountID, &item.AccountName,
			&item.RouteFingerprint, &item.Platform, &item.Model, &item.RequestClass, &item.Protocol,
			&item.SampleCount, &item.SuccessCount, &item.FailureCount, &item.RecoveredCount, &item.CustomerImpactCount,
			&item.AverageLatencyMs, &item.P95LatencyMs, &item.LastObservedAt, &lastFailure, &lastSuccess, &lastRecovery,
			&bucketTotal, &totals.SampleCount, &totals.FailureCount, &totals.CustomerRequestCount, &totals.CustomerSuccessCount,
		); err != nil {
			return nil, ChannelMonitoringTotals{}, 0, err
		}
		item.FactType = ReliabilityFactType(factType)
		if groupID.Valid {
			value := groupID.Int64
			item.GroupID = &value
		}
		if accountID.Valid {
			value := accountID.Int64
			item.AccountID = &value
		}
		if lastFailure.Valid {
			value := lastFailure.Time
			item.LastFailureAt = &value
		}
		if lastSuccess.Valid {
			value := lastSuccess.Time
			item.LastSuccessAt = &value
		}
		if lastRecovery.Valid {
			value := lastRecovery.Time
			item.LastRecoveryAt = &value
		}
		if item.SampleCount > 0 {
			value := float64(item.SuccessCount+item.RecoveredCount) / float64(item.SampleCount)
			item.Availability = &value
		}
		if minutes > 0 {
			item.SamplesPerMinute = float64(item.SampleCount) / minutes
		}
		item.ProductCodes = []string{}
		item.ComponentCodes = []string{}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, ChannelMonitoringTotals{}, 0, err
	}
	if err := tx.Commit(); err != nil {
		return nil, ChannelMonitoringTotals{}, 0, err
	}
	return result, totals, bucketTotal, nil
}

func projectChannelMonitoringEvidence(products []statusProductDefinition, evidence []ChannelMonitoringEvidence) {
	componentCodes := map[int64]string{}
	productCodes := map[int64]string{}
	componentProducts := map[int64]int64{}
	for _, product := range products {
		productCodes[product.ID] = product.Code
		for _, component := range product.Components {
			componentCodes[component.ID] = component.Code
			componentProducts[component.ID] = product.ID
		}
	}
	for index := range evidence {
		item := &evidence[index]
		observation := &ReliabilityObservation{
			FactType: item.FactType, GroupID: item.GroupID, AccountID: item.AccountID,
			Platform: item.Platform, Model: item.Model, RequestClass: item.RequestClass,
			Protocol: item.Protocol, RouteFingerprint: item.RouteFingerprint,
			Outcome: ReliabilityOutcomeSuccess, ObservedAt: item.LastObservedAt,
		}
		targets := matchingStatusComponentIDs(products, observation, true)
		productSet := map[string]struct{}{}
		for _, componentID := range targets {
			if code := componentCodes[componentID]; code != "" {
				item.ComponentCodes = append(item.ComponentCodes, code)
			}
			if code := productCodes[componentProducts[componentID]]; code != "" {
				productSet[code] = struct{}{}
			}
		}
		for code := range productSet {
			item.ProductCodes = append(item.ProductCodes, code)
		}
		sort.Strings(item.ComponentCodes)
		sort.Strings(item.ProductCodes)
	}
}
