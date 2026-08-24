package service

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

func (s *CompensationControlService) ReviseDraft(ctx context.Context, command CompensationRevisionCommand) (*CompensationDraft, error) {
	command.Reason = strings.TrimSpace(command.Reason)
	if s == nil || s.db == nil || command.DraftID <= 0 || command.Reason == "" || len(command.Reason) > 500 || len(command.Adjustments) == 0 {
		return nil, fmt.Errorf("draft, reason, and adjustments are required")
	}
	prior, err := s.GetDraft(ctx, command.DraftID)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('compensation-draft'),$1)`, prior.IncidentID); err != nil {
		return nil, err
	}
	var latestID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM compensation_drafts WHERE series_id=$1 ORDER BY revision_number DESC LIMIT 1`, prior.SeriesID).Scan(&latestID); err != nil {
		return nil, err
	}
	if latestID != prior.ID {
		return nil, fmt.Errorf("only the latest immutable draft revision can be revised")
	}
	var approved bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM compensation_draft_approvals a JOIN compensation_drafts d ON d.id=a.draft_id WHERE d.series_id=$1)`, prior.SeriesID).Scan(&approved); err != nil {
		return nil, err
	}
	if approved {
		return nil, fmt.Errorf("an approved compensation series cannot be revised")
	}
	policy, err := loadCompensationPolicyVersion(ctx, tx, prior.PolicyVersion)
	if err != nil {
		return nil, err
	}
	adjustments := map[int64]CompensationRevisionAdjustment{}
	for _, adjustment := range command.Adjustments {
		if adjustment.UserID <= 0 || adjustment.ProposedCNYFen < 0 {
			return nil, fmt.Errorf("revision adjustment is invalid")
		}
		if _, exists := adjustments[adjustment.UserID]; exists {
			return nil, fmt.Errorf("duplicate revision adjustment")
		}
		adjustments[adjustment.UserID] = adjustment
	}
	build := compensationDraftBuild{policy: policy, incidentID: prior.IncidentID, periodID: prior.ShadowPeriodID, productRollingPaid: prior.AffectedProductRollingPaidValueCNYFen}
	var changed bool
	appliedAdjustments := map[int64]bool{}
	for _, previous := range prior.Users {
		next := previous
		next.ID = 0
		next.Email = ""
		next.Items = nil
		next.EvidenceHash = ""
		adjustment, ok := adjustments[previous.UserID]
		if ok {
			appliedAdjustments[previous.UserID] = true
			maxAllowed := roundMicrosToFen(previous.RawValueCNYMicros)
			maxAllowed = minCompensationInt64(maxAllowed, previous.TierCapCNYFen)
			maxAllowed = minCompensationInt64(maxAllowed, previous.RelationshipCapCNYFen)
			if previous.RollingRemainingCNYFen != nil {
				maxAllowed = minCompensationInt64(maxAllowed, *previous.RollingRemainingCNYFen)
			}
			if adjustment.Included && !previous.Eligible {
				return nil, fmt.Errorf("ineligible user %d cannot be included by revision", previous.UserID)
			}
			if adjustment.Included && !previous.Included && previous.ExclusionReason != "operator_excluded" {
				return nil, fmt.Errorf("user %d has a frozen evidence exclusion and cannot be manually included", previous.UserID)
			}
			if adjustment.ProposedCNYFen > maxAllowed {
				return nil, fmt.Errorf("revision for user %d exceeds frozen policy caps", previous.UserID)
			}
			if !adjustment.Included {
				next.Included = false
				if previous.Included {
					next.ExclusionReason = "operator_excluded"
				}
				adjustment.ProposedCNYFen = 0
			} else {
				next.Included = true
				if next.ExclusionReason == "operator_excluded" {
					next.ExclusionReason = ""
				}
			}
			if adjustment.ProposedCNYFen != previous.ProposedTotalCNYFen || next.Included != previous.Included {
				changed = true
			}
			next.ProposedTotalCNYFen = adjustment.ProposedCNYFen
		}
		items := make([]compensationItemBuild, len(previous.Items))
		raws := make([]int64, len(previous.Items))
		for i, item := range previous.Items {
			item.ID = 0
			raws[i] = item.RawValueCNYMicros
		}
		allocation := AllocateCompensationFen(next.ProposedTotalCNYFen, raws)
		next.BalanceBenefitCNYFen = 0
		next.BuilderPassBenefitCNYFen = 0
		for i, item := range previous.Items {
			item.ID = 0
			item.ProposedCNYFen = allocation[i]
			if item.BenefitChannel == "builder_pass" {
				next.BuilderPassBenefitCNYFen, err = checkedAddCompensation(next.BuilderPassBenefitCNYFen, allocation[i])
				if err != nil {
					return nil, err
				}
			} else {
				next.BalanceBenefitCNYFen, err = checkedAddCompensation(next.BalanceBenefitCNYFen, allocation[i])
				if err != nil {
					return nil, err
				}
			}
			evidence, _ := json.Marshal(item)
			var weight any
			if item.GroupWeightVersionID != nil {
				weight = *item.GroupWeightVersionID
			}
			items[i] = compensationItemBuild{item: item, rateID: item.ProductRateVersionID, weightID: weight, evidence: evidence}
		}
		evidenceItems := make([]CompensationDraftItem, len(items))
		for i := range items {
			evidenceItems[i] = items[i].item
		}
		evidence, _ := canonicalCompensationJSON(compensationUserEvidence{User: next, Items: evidenceItems, Policy: policy})
		hash := sha256.Sum256(evidence)
		next.EvidenceHash = hex.EncodeToString(hash[:])
		build.users = append(build.users, compensationUserBuild{user: next, evidence: evidence, items: items})
		build.total, err = checkedAddCompensation(build.total, next.ProposedTotalCNYFen)
		if err != nil {
			return nil, err
		}
	}
	if len(appliedAdjustments) != len(adjustments) {
		return nil, fmt.Errorf("revision adjustment references a user outside the draft")
	}
	if !changed {
		return nil, fmt.Errorf("revision must change at least one frozen proposal")
	}
	evidenceUsers := make([]compensationUserEvidence, 0, len(build.users))
	for _, user := range build.users {
		items := make([]CompensationDraftItem, len(user.items))
		for i := range user.items {
			items[i] = user.items[i].item
		}
		evidenceUsers = append(evidenceUsers, compensationUserEvidence{User: user.user, Items: items, Policy: policy})
	}
	draftPayload := struct {
		PriorDraftID int64                      `json:"prior_draft_id"`
		Reason       string                     `json:"reason"`
		Actor        int64                      `json:"actor_user_id"`
		Users        []compensationUserEvidence `json:"users"`
	}{prior.ID, command.Reason, command.ActorUserID, evidenceUsers}
	build.payload, err = canonicalCompensationJSON(draftPayload)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(build.payload)
	build.hash = hex.EncodeToString(sum[:])
	replaces := prior.ID
	draftID, err := persistCompensationDraftTx(ctx, tx, build, prior.SeriesID, prior.RevisionNumber+1, &replaces, command.Reason, command.ActorUserID, true)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return s.GetDraft(ctx, draftID)
}

func (s *CompensationControlService) ReviewShadowDraft(ctx context.Context, command CompensationShadowReviewCommand) (*CompensationShadowReview, error) {
	command.Notes = strings.TrimSpace(command.Notes)
	if s == nil || s.db == nil || command.DraftID <= 0 || command.OwnerJudgementTotalCNYFen < 0 || command.Notes == "" || len(command.Notes) > 1000 || (command.Result != "aligned" && command.Result != "redesign_required") {
		return nil, fmt.Errorf("draft, judgement, result, and review notes are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var incidentID, proposed int64
	var series string
	var high bool
	if err = tx.QueryRowContext(ctx, `SELECT incident_id,series_id::text,proposed_total_cny_fen,high_value FROM compensation_drafts WHERE id=$1`, command.DraftID).Scan(&incidentID, &series, &proposed, &high); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('compensation-draft'),$1)`, incidentID); err != nil {
		return nil, err
	}
	var latestID int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM compensation_drafts WHERE series_id=$1 ORDER BY revision_number DESC LIMIT 1`, series).Scan(&latestID); err != nil {
		return nil, err
	}
	if latestID != command.DraftID {
		return nil, fmt.Errorf("only the latest draft revision can be reviewed")
	}
	if high && command.Result == "aligned" {
		return nil, fmt.Errorf("high-value draft must be redesigned before alignment review")
	}
	item := &CompensationShadowReview{DraftID: command.DraftID, OwnerJudgementTotalCNYFen: command.OwnerJudgementTotalCNYFen, VarianceCNYFen: proposed - command.OwnerJudgementTotalCNYFen, Result: command.Result, Notes: command.Notes}
	var actor any
	if command.ActorUserID > 0 {
		v := command.ActorUserID
		item.CreatedByUserID = &v
		actor = v
	}
	var existingActor sql.NullInt64
	err = tx.QueryRowContext(ctx, `INSERT INTO compensation_shadow_reviews(draft_id,owner_judgement_total_cny_fen,variance_cny_fen,result,notes,created_by_user_id) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(draft_id) DO NOTHING RETURNING id,created_at`, item.DraftID, item.OwnerJudgementTotalCNYFen, item.VarianceCNYFen, item.Result, item.Notes, actor).Scan(&item.ID, &item.CreatedAt)
	if err == sql.ErrNoRows {
		err = tx.QueryRowContext(ctx, `SELECT id,owner_judgement_total_cny_fen,variance_cny_fen,result,notes,created_by_user_id,created_at FROM compensation_shadow_reviews WHERE draft_id=$1`, item.DraftID).Scan(&item.ID, &item.OwnerJudgementTotalCNYFen, &item.VarianceCNYFen, &item.Result, &item.Notes, &existingActor, &item.CreatedAt)
		if err != nil {
			return nil, err
		}
		if existingActor.Valid {
			v := existingActor.Int64
			item.CreatedByUserID = &v
		}
		if item.OwnerJudgementTotalCNYFen != command.OwnerJudgementTotalCNYFen || item.Result != command.Result || item.Notes != command.Notes {
			return nil, fmt.Errorf("shadow review already exists with different input")
		}
	} else if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return item, nil
}
