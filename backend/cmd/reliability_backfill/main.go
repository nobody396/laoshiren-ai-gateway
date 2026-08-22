package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	_ "github.com/lib/pq"
)

const maxAuditWindow = 7 * 24 * time.Hour

type auditCandidate struct {
	Source     string    `json:"source"`
	SourceID   int64     `json:"source_id"`
	FactType   string    `json:"fact_type"`
	Outcome    string    `json:"outcome"`
	ObservedAt time.Time `json:"observed_at"`
}

type auditSourceRow struct {
	Source          string
	SourceID        int64
	RequestID       string
	ClientRequestID string
	ErrorOwner      string
	BusinessLimited bool
	CountTokens     bool
	ObservedAt      time.Time
}

type auditReport struct {
	Mode       string           `json:"mode"`
	Start      time.Time        `json:"start"`
	End        time.Time        `json:"end"`
	Candidates []auditCandidate `json:"candidates"`
	Counts     map[string]int   `json:"counts"`
}

func main() {
	startRaw := flag.String("start", "", "inclusive start time (RFC3339 or YYYY-MM-DD)")
	endRaw := flag.String("end", "", "exclusive end time (RFC3339 or YYYY-MM-DD; default now)")
	limit := flag.Int("limit", 1000, "maximum candidate source rows (1..5000)")
	flag.Parse()
	if *limit <= 0 || *limit > 5000 {
		log.Fatal("--limit must be between 1 and 5000")
	}
	cfg, err := config.LoadForBootstrap()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.Fatalf("load timezone: %v", err)
	}
	start, end, err := parseAuditWindow(*startRaw, *endRaw, location, time.Now())
	if err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("postgres", cfg.Database.DSNWithTimezone(cfg.Timezone))
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(0)
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	report, err := runAudit(ctx, db, start, end, *limit)
	if err != nil {
		log.Fatalf("run read-only audit: %v", err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		log.Fatalf("encode report: %v", err)
	}
}

func runAudit(ctx context.Context, db *sql.DB, start, end time.Time, limit int) (*auditReport, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `SET LOCAL statement_timeout = '60s'`); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, reliabilityBackfillAuditSQL(), start, end, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	sources := make([]auditSourceRow, 0, limit)
	for rows.Next() {
		source := auditSourceRow{}
		if err := rows.Scan(
			&source.Source, &source.SourceID, &source.RequestID, &source.ClientRequestID,
			&source.ErrorOwner, &source.BusinessLimited, &source.CountTokens, &source.ObservedAt,
		); err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	report := &auditReport{Mode: "read-only", Start: start, End: end, Counts: map[string]int{}}
	report.Candidates = classifyAuditCandidates(sources)
	for _, candidate := range report.Candidates {
		report.Counts[candidate.FactType+":"+candidate.Outcome]++
	}
	return report, nil
}

func reliabilityBackfillAuditSQL() string {
	return `
WITH source_rows AS (
  SELECT
    'usage_logs'::text AS source,
    ul.id AS source_id,
    COALESCE(ul.request_id, '') AS request_id,
    ''::text AS client_request_id,
    ''::text AS error_owner,
    FALSE AS is_business_limited,
    FALSE AS is_count_tokens,
    ul.created_at AS observed_at
  FROM usage_logs ul
  WHERE ul.created_at >= $1 AND ul.created_at < $2
    AND COALESCE(ul.request_id, '') <> ''

  UNION ALL

  SELECT
    'ops_error_logs'::text AS source,
    e.id AS source_id,
    COALESCE(e.request_id, '') AS request_id,
    COALESCE(e.client_request_id, '') AS client_request_id,
    COALESCE(e.error_owner, '') AS error_owner,
    COALESCE(e.is_business_limited, FALSE) AS is_business_limited,
    COALESCE(e.is_count_tokens, FALSE) AS is_count_tokens,
    e.created_at AS observed_at
  FROM ops_error_logs e
  WHERE e.created_at >= $1 AND e.created_at < $2
    AND COALESCE(e.request_id, e.client_request_id, '') <> ''
    AND COALESCE(e.error_source, '') <> 'monthly_upstream_probe'
)
SELECT source, source_id, request_id, client_request_id, error_owner,
       is_business_limited, is_count_tokens, observed_at
FROM source_rows
ORDER BY observed_at, source, source_id
LIMIT $3`
}

func classifyAuditCandidates(rows []auditSourceRow) []auditCandidate {
	type finalState struct {
		candidate auditCandidate
		recovered bool
		failure   bool
	}
	finals := make(map[string]*finalState)
	errorsByRequestID := make(map[string][]auditSourceRow)
	for _, row := range rows {
		row.RequestID = strings.TrimSpace(row.RequestID)
		row.ClientRequestID = strings.TrimSpace(row.ClientRequestID)
		if row.Source == "usage_logs" && row.RequestID != "" {
			if _, exists := finals[row.RequestID]; !exists {
				finals[row.RequestID] = &finalState{candidate: auditCandidate{
					Source: row.Source, SourceID: row.SourceID, FactType: "customer_request", Outcome: "success", ObservedAt: row.ObservedAt,
				}}
			}
			continue
		}
		if row.Source == "ops_error_logs" {
			identity := row.ClientRequestID
			if identity == "" {
				identity = row.RequestID
			}
			if identity == "" {
				continue
			}
			errorsByRequestID[identity] = append(errorsByRequestID[identity], row)
		}
	}

	candidates := make([]auditCandidate, 0, len(rows))
	for identity, errorRows := range errorsByRequestID {
		matchedFinal := finals[identity]
		if matchedFinal == nil {
			for _, row := range errorRows {
				if final := finals[row.RequestID]; final != nil {
					matchedFinal = final
					break
				}
			}
		}
		for _, row := range errorRows {
			providerOwned := row.ErrorOwner == "provider" || row.ErrorOwner == "platform"
			excluded := row.BusinessLimited || row.CountTokens || !providerOwned
			if matchedFinal != nil && providerOwned && !excluded {
				matchedFinal.recovered = true
			}
			if providerOwned && !excluded {
				candidates = append(candidates, auditCandidate{
					Source: row.Source, SourceID: row.SourceID, FactType: "upstream_attempt", Outcome: "failure", ObservedAt: row.ObservedAt,
				})
			}
			if matchedFinal == nil && providerOwned && !excluded {
				if state := finals[identity]; state == nil {
					finals[identity] = &finalState{candidate: auditCandidate{
						Source: row.Source, SourceID: row.SourceID, FactType: "customer_request", Outcome: "failure", ObservedAt: row.ObservedAt,
					}, failure: true}
				} else {
					state.failure = true
				}
			} else if matchedFinal == nil {
				if _, exists := finals[identity]; !exists {
					finals[identity] = &finalState{candidate: auditCandidate{
						Source: row.Source, SourceID: row.SourceID, FactType: "customer_request", Outcome: "excluded", ObservedAt: row.ObservedAt,
					}}
				}
			}
		}
	}
	for _, state := range finals {
		if state.recovered {
			state.candidate.Outcome = "recovered"
		} else if state.failure {
			state.candidate.Outcome = "failure"
		}
		candidates = append(candidates, state.candidate)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ObservedAt.Equal(candidates[j].ObservedAt) {
			if candidates[i].Source == candidates[j].Source {
				return candidates[i].SourceID < candidates[j].SourceID
			}
			return candidates[i].Source < candidates[j].Source
		}
		return candidates[i].ObservedAt.Before(candidates[j].ObservedAt)
	})
	return candidates
}

func parseAuditWindow(startRaw, endRaw string, location *time.Location, now time.Time) (time.Time, time.Time, error) {
	if strings.TrimSpace(startRaw) == "" {
		return time.Time{}, time.Time{}, fmt.Errorf("--start is required")
	}
	start, err := parseAuditTime(startRaw, location)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --start: %w", err)
	}
	end := now
	if strings.TrimSpace(endRaw) != "" {
		end, err = parseAuditTime(endRaw, location)
		if err != nil {
			return time.Time{}, time.Time{}, fmt.Errorf("invalid --end: %w", err)
		}
	}
	if !start.Before(end) {
		return time.Time{}, time.Time{}, fmt.Errorf("--start must be before --end")
	}
	if end.Sub(start) > maxAuditWindow {
		return time.Time{}, time.Time{}, fmt.Errorf("audit window cannot exceed seven days")
	}
	return start, end, nil
}

func parseAuditTime(raw string, location *time.Location) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02 15:04:05", value, location); err == nil {
		return parsed, nil
	}
	if parsed, err := time.ParseInLocation("2006-01-02", value, location); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("unsupported time %q", raw)
}
