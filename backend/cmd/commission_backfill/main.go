package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/bozhouDev/DragonCode-sub2api/ent/runtime"
	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/repository"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

const consumptionCommissionRate = 0.06

type candidate struct {
	UsageLogID        int64
	RequestID         string
	UserID            int64
	AgentID           int64
	ActualCost        float64
	CommissionAmount  float64
	UsageCreatedAtUTC time.Time
}

func main() {
	startRaw := flag.String("start", "", "inclusive start time, supports RFC3339 or YYYY-MM-DD")
	endRaw := flag.String("end", "", "exclusive end time, supports RFC3339 or YYYY-MM-DD")
	limit := flag.Int("limit", 500, "max number of usage logs to inspect")
	userID := flag.Int64("user-id", 0, "optional user filter")
	agentID := flag.Int64("agent-id", 0, "optional agent filter")
	printLimit := flag.Int("print-limit", 20, "how many candidate rows to print")
	apply := flag.Bool("apply", false, "apply changes instead of dry-run")
	flag.Parse()

	if strings.TrimSpace(*startRaw) == "" {
		log.Fatal("--start is required")
	}
	if *limit <= 0 {
		log.Fatal("--limit must be greater than 0")
	}
	if *printLimit < 0 {
		log.Fatal("--print-limit must be >= 0")
	}

	cfg, err := config.LoadForBootstrap()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	location, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.Fatalf("failed to load timezone %q: %v", cfg.Timezone, err)
	}

	start, err := parseTimeFlag(*startRaw, location, false)
	if err != nil {
		log.Fatalf("invalid --start: %v", err)
	}

	var end *time.Time
	if strings.TrimSpace(*endRaw) != "" {
		parsedEnd, err := parseTimeFlag(*endRaw, location, true)
		if err != nil {
			log.Fatalf("invalid --end: %v", err)
		}
		if !parsedEnd.After(start) {
			log.Fatal("--end must be after --start")
		}
		end = &parsedEnd
	}

	client, sqlDB, err := repository.InitEnt(cfg)
	if err != nil {
		log.Fatalf("failed to init db: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("failed to close db: %v", err)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	candidates, err := loadCandidates(ctx, sqlDB, start, end, *limit, *userID, *agentID)
	if err != nil {
		log.Fatalf("failed to load candidates: %v", err)
	}

	var totalActualCost float64
	var totalCommission float64
	for _, item := range candidates {
		totalActualCost += item.ActualCost
		totalCommission += item.CommissionAmount
	}

	mode := "dry-run"
	if *apply {
		mode = "apply"
	}
	fmt.Printf("mode=%s timezone=%s start=%s", mode, cfg.Timezone, start.Format(time.RFC3339))
	if end != nil {
		fmt.Printf(" end=%s", end.Format(time.RFC3339))
	}
	if *userID > 0 {
		fmt.Printf(" user_id=%d", *userID)
	}
	if *agentID > 0 {
		fmt.Printf(" agent_id=%d", *agentID)
	}
	fmt.Printf(" limit=%d\n", *limit)
	fmt.Printf("candidates=%d total_actual_cost=%.8f total_commission=%.8f\n", len(candidates), totalActualCost, totalCommission)

	printed := min(len(candidates), *printLimit)
	for i := 0; i < printed; i++ {
		item := candidates[i]
		fmt.Printf(
			"candidate[%d] usage_log_id=%d request_id=%q user_id=%d agent_id=%d actual_cost=%.8f commission=%.8f created_at=%s\n",
			i,
			item.UsageLogID,
			item.RequestID,
			item.UserID,
			item.AgentID,
			item.ActualCost,
			item.CommissionAmount,
			item.UsageCreatedAtUTC.Format(time.RFC3339),
		)
	}

	if !*apply {
		if len(candidates) > 0 {
			fmt.Println("dry-run only; rerun with --apply after confirming the bug window.")
		}
		return
	}

	appliedCount := 0
	skippedCount := 0
	for _, item := range candidates {
		rowCtx, rowCancel := context.WithTimeout(context.Background(), 10*time.Second)
		applied, err := applyCandidate(rowCtx, sqlDB, item)
		rowCancel()
		if err != nil {
			log.Fatalf("failed to backfill usage_log_id=%d: %v", item.UsageLogID, err)
		}
		if applied {
			appliedCount++
			continue
		}
		skippedCount++
	}

	fmt.Printf("applied=%d skipped=%d\n", appliedCount, skippedCount)
}

func parseTimeFlag(raw string, location *time.Location, endOfDay bool) (time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, errors.New("empty time")
	}

	if ts, err := time.Parse(time.RFC3339, value); err == nil {
		return ts, nil
	}
	if ts, err := time.ParseInLocation("2006-01-02 15:04:05", value, location); err == nil {
		return ts, nil
	}
	if ts, err := time.ParseInLocation("2006-01-02", value, location); err == nil {
		if endOfDay {
			return ts.Add(24 * time.Hour), nil
		}
		return ts, nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q", raw)
}

func loadCandidates(ctx context.Context, db *sql.DB, start time.Time, end *time.Time, limit int, userID, agentID int64) ([]candidate, error) {
	args := []any{
		service.CommissionTypeConsumption,
		service.BillingTypeBalance,
		start,
	}

	conditions := []string{
		"ul.actual_cost > 0",
		"ul.billing_type = $2",
		"ul.created_at >= $3",
		"invited.deleted_at IS NULL",
		"invited.agent_id IS NOT NULL",
		"agent.deleted_at IS NULL",
		"existing.id IS NULL",
	}

	argPos := 4
	if end != nil {
		conditions = append(conditions, fmt.Sprintf("ul.created_at < $%d", argPos))
		args = append(args, *end)
		argPos++
	}
	if userID > 0 {
		conditions = append(conditions, fmt.Sprintf("ul.user_id = $%d", argPos))
		args = append(args, userID)
		argPos++
	}
	if agentID > 0 {
		conditions = append(conditions, fmt.Sprintf("invited.agent_id = $%d", argPos))
		args = append(args, agentID)
		argPos++
	}

	args = append(args, limit)
	limitPos := argPos

	query := fmt.Sprintf(`
		SELECT
			ul.id,
			COALESCE(ul.request_id, ''),
			ul.user_id,
			invited.agent_id,
			ul.actual_cost,
			ul.created_at
		FROM usage_logs ul
		INNER JOIN users invited
			ON invited.id = ul.user_id
		INNER JOIN users agent
			ON agent.id = invited.agent_id
		LEFT JOIN commission_records existing
			ON existing.type = $1
			AND existing.source_id = ul.id
		WHERE %s
		ORDER BY ul.id ASC
		LIMIT $%d
	`, strings.Join(conditions, " AND "), limitPos)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]candidate, 0)
	for rows.Next() {
		var item candidate
		if err := rows.Scan(
			&item.UsageLogID,
			&item.RequestID,
			&item.UserID,
			&item.AgentID,
			&item.ActualCost,
			&item.UsageCreatedAtUTC,
		); err != nil {
			return nil, err
		}
		item.CommissionAmount = item.ActualCost * consumptionCommissionRate
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

func applyCandidate(ctx context.Context, db *sql.DB, item candidate) (bool, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var exists bool
	if err := tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM commission_records
			WHERE type = $1 AND source_id = $2
		)
	`, service.CommissionTypeConsumption, item.UsageLogID).Scan(&exists); err != nil {
		return false, err
	}
	if exists {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		return false, nil
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE users
		SET balance = balance + $2
		WHERE id = $1
	`, item.AgentID, item.CommissionAmount); err != nil {
		return false, err
	}

	note := fmt.Sprintf("消耗分佣，用户 #%d 消费 %.8f", item.UserID, item.ActualCost)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO commission_records (
			beneficiary_id,
			user_id,
			amount,
			source_amount,
			type,
			source_id,
			note,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		ON CONFLICT DO NOTHING
	`, item.AgentID, item.UserID, item.CommissionAmount, item.ActualCost, service.CommissionTypeConsumption, item.UsageLogID, note); err != nil {
		return false, err
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
