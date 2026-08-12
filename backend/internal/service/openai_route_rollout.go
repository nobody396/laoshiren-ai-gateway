package service

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math/bits"
	"strings"
	"time"
)

const (
	// OpenAIRouteEnforceCodeAvailable deliberately remains false in the release
	// that introduces the rollout contract. A later, separately reviewed
	// release must change this constant and wire the adaptive choice into the
	// scheduler; policy data alone can never activate traffic in this release.
	OpenAIRouteEnforceCodeAvailable = false

	openAIRouteRolloutBucketCount = uint64(10_000)
	openAIRouteMinimumStageDwell  = 24 * time.Hour
)

var ErrOpenAIRouteInvalidRollout = errors.New("invalid OpenAI route rollout")

// OpenAIRouteRolloutStage is separate from OpenAIRoutePolicyMode. Shadow can
// produce evidence without serving traffic, while every canary stage is a
// distinct, explicitly approved traffic state.
type OpenAIRouteRolloutStage string

const (
	OpenAIRouteRolloutLegacy    OpenAIRouteRolloutStage = "legacy"
	OpenAIRouteRolloutShadow    OpenAIRouteRolloutStage = "shadow"
	OpenAIRouteRolloutCanary1   OpenAIRouteRolloutStage = "canary_1"
	OpenAIRouteRolloutCanary5   OpenAIRouteRolloutStage = "canary_5"
	OpenAIRouteRolloutCanary20  OpenAIRouteRolloutStage = "canary_20"
	OpenAIRouteRolloutCanary50  OpenAIRouteRolloutStage = "canary_50"
	OpenAIRouteRolloutCanary100 OpenAIRouteRolloutStage = "canary_100"
)

func (s OpenAIRouteRolloutStage) TrafficBasisPoints() (int, bool) {
	switch s {
	case OpenAIRouteRolloutCanary1:
		return 100, true
	case OpenAIRouteRolloutCanary5:
		return 500, true
	case OpenAIRouteRolloutCanary20:
		return 2_000, true
	case OpenAIRouteRolloutCanary50:
		return 5_000, true
	case OpenAIRouteRolloutCanary100:
		return 10_000, true
	default:
		return 0, false
	}
}

func (s OpenAIRouteRolloutStage) Valid() bool {
	if s == OpenAIRouteRolloutLegacy || s == OpenAIRouteRolloutShadow {
		return true
	}
	_, ok := s.TrafficBasisPoints()
	return ok
}

func nextOpenAIRouteRolloutStage(stage OpenAIRouteRolloutStage) (OpenAIRouteRolloutStage, bool) {
	switch stage {
	case OpenAIRouteRolloutLegacy:
		return OpenAIRouteRolloutShadow, true
	case OpenAIRouteRolloutShadow:
		return OpenAIRouteRolloutCanary1, true
	case OpenAIRouteRolloutCanary1:
		return OpenAIRouteRolloutCanary5, true
	case OpenAIRouteRolloutCanary5:
		return OpenAIRouteRolloutCanary20, true
	case OpenAIRouteRolloutCanary20:
		return OpenAIRouteRolloutCanary50, true
	case OpenAIRouteRolloutCanary50:
		return OpenAIRouteRolloutCanary100, true
	default:
		return "", false
	}
}

// OpenAIRouteRolloutEvidence is the immutable handoff from a read-only
// assessment plus an owner approval. It contains no credentials or request
// content. ApprovedStage prevents one approval from being replayed for a later
// percentage increase.
type OpenAIRouteRolloutEvidence struct {
	ActivationID           string
	PolicyVersion          int
	RequestClass           OpenAIRouteRequestClass
	ApprovedStage          OpenAIRouteRolloutStage
	WindowStart            time.Time
	WindowEnd              time.Time
	AssessedAt             time.Time
	AutomatedEvidenceReady bool
	ManualApprovalID       string
	ManualApprovedAt       time.Time
	SourceRevision         string
}

type OpenAIRouteRolloutTransitionRequest struct {
	ActivationID         string
	ActivationApprovalID string
	PolicyVersion        int
	RequestClass         OpenAIRouteRequestClass

	CurrentStage          OpenAIRouteRolloutStage
	RequestedStage        OpenAIRouteRolloutStage
	CurrentStageStartedAt time.Time
	RequestedAt           time.Time

	// CodePathRevision identifies the release that first introduced the
	// dormant enforce path. DeploymentRevision is the later release being
	// considered for activation. They must differ for any real traffic stage.
	CodePathRevision   string
	DeploymentRevision string

	Evidence OpenAIRouteRolloutEvidence
}

type OpenAIRouteRolloutTransitionAssessment struct {
	Allowed                bool
	Reason                 string
	CurrentStage           OpenAIRouteRolloutStage
	RequestedStage         OpenAIRouteRolloutStage
	ManualApprovalRequired bool
	AutomaticPromotion     bool
	EnforceCodeAvailable   bool
}

// AssessOpenAIRouteRolloutTransition always applies this release's hard
// enforce guard. Unit tests exercise the lower-level contract independently,
// but production callers cannot supply a boolean that bypasses the constant.
func AssessOpenAIRouteRolloutTransition(req OpenAIRouteRolloutTransitionRequest) OpenAIRouteRolloutTransitionAssessment {
	return assessOpenAIRouteRolloutTransition(req, OpenAIRouteEnforceCodeAvailable)
}

func assessOpenAIRouteRolloutTransition(
	req OpenAIRouteRolloutTransitionRequest,
	enforceCodeAvailable bool,
) OpenAIRouteRolloutTransitionAssessment {
	result := OpenAIRouteRolloutTransitionAssessment{
		Reason:                 "invalid_transition",
		CurrentStage:           req.CurrentStage,
		RequestedStage:         req.RequestedStage,
		ManualApprovalRequired: true,
		AutomaticPromotion:     false,
		EnforceCodeAvailable:   enforceCodeAvailable,
	}
	if !req.CurrentStage.Valid() || !req.RequestedStage.Valid() {
		return result
	}
	// One-click rollback is deliberately independent of telemetry, storage,
	// approval, or the adaptive runtime. It only stops future adaptive choices;
	// it must not delete historical evidence.
	if req.RequestedStage == OpenAIRouteRolloutLegacy {
		result.Allowed = true
		result.Reason = "rollback_to_legacy"
		return result
	}
	if req.RequestedStage == req.CurrentStage {
		result.Allowed = true
		result.Reason = "no_change"
		return result
	}
	next, ok := nextOpenAIRouteRolloutStage(req.CurrentStage)
	if !ok || next != req.RequestedStage {
		result.Reason = "stage_skip_forbidden"
		return result
	}
	if strings.TrimSpace(req.ActivationID) == "" || req.PolicyVersion <= 0 || !req.RequestClass.Valid() || req.RequestedAt.IsZero() {
		result.Reason = "invalid_scope"
		return result
	}
	// Entering Shadow does not serve adaptive traffic. Its T0 is created only
	// by a separately authorized control-plane action, outside this evaluator.
	if req.RequestedStage == OpenAIRouteRolloutShadow {
		if strings.TrimSpace(req.ActivationApprovalID) == "" {
			result.Reason = "shadow_activation_approval_missing"
			return result
		}
		result.Allowed = true
		result.Reason = "explicit_shadow_activation_authorized"
		return result
	}
	if !enforceCodeAvailable {
		result.Reason = "enforce_release_guard_disabled"
		return result
	}
	if strings.TrimSpace(req.CodePathRevision) == "" || strings.TrimSpace(req.DeploymentRevision) == "" ||
		strings.TrimSpace(req.CodePathRevision) == strings.TrimSpace(req.DeploymentRevision) {
		result.Reason = "separate_release_required"
		return result
	}
	if req.CurrentStageStartedAt.IsZero() || req.RequestedAt.Before(req.CurrentStageStartedAt) ||
		req.RequestedAt.Sub(req.CurrentStageStartedAt) < openAIRouteMinimumStageDwell {
		result.Reason = "minimum_stage_dwell_not_met"
		return result
	}
	evidence := req.Evidence
	if !evidence.AutomatedEvidenceReady || strings.TrimSpace(evidence.ManualApprovalID) == "" || evidence.ManualApprovedAt.IsZero() {
		result.Reason = "evidence_or_manual_approval_missing"
		return result
	}
	if strings.TrimSpace(evidence.ActivationID) != strings.TrimSpace(req.ActivationID) ||
		evidence.PolicyVersion != req.PolicyVersion || evidence.RequestClass != req.RequestClass ||
		evidence.ApprovedStage != req.RequestedStage {
		result.Reason = "evidence_scope_mismatch"
		return result
	}
	minimumEvidenceWindow := openAIRoutePromotionInitialCheckpoint
	if req.CurrentStage == OpenAIRouteRolloutShadow {
		minimumEvidenceWindow = openAIRoutePromotionMinimumWindow
	}
	if evidence.WindowStart.IsZero() || evidence.WindowEnd.IsZero() || !evidence.WindowStart.Before(evidence.WindowEnd) ||
		evidence.WindowEnd.Sub(evidence.WindowStart) < minimumEvidenceWindow {
		result.Reason = "evidence_window_too_short"
		return result
	}
	if evidence.AssessedAt.Before(evidence.WindowEnd) || evidence.ManualApprovedAt.Before(evidence.AssessedAt) ||
		req.RequestedAt.Before(evidence.ManualApprovedAt) {
		result.Reason = "evidence_time_order_invalid"
		return result
	}
	if strings.TrimSpace(evidence.SourceRevision) == "" || strings.TrimSpace(evidence.SourceRevision) != strings.TrimSpace(req.DeploymentRevision) {
		result.Reason = "evidence_revision_mismatch"
		return result
	}
	result.Allowed = true
	result.Reason = "explicit_stage_transition_authorized"
	return result
}

type OpenAIRouteRolloutAssignmentRequest struct {
	ActivationID  string
	PolicyVersion int
	GroupID       int64
	Model         string
	RequestClass  OpenAIRouteRequestClass
	Stage         OpenAIRouteRolloutStage

	// TextAffinityKey is the existing sticky-session/previous-response
	// identity. ImageRequestID is scoped to one logical image request and keeps
	// retries in the same cohort without creating cross-request stickiness.
	TextAffinityKey string
	ImageRequestID  string
}

type OpenAIRouteRolloutAssignment struct {
	UseAdaptive          bool
	CohortEligible       bool
	Reason               string
	Stage                OpenAIRouteRolloutStage
	TrafficBasisPoints   int
	Bucket               int
	BucketBasis          string
	EnforceCodeAvailable bool
}

// AssignOpenAIRouteRolloutCohort is fail-closed. It may compute the stable
// cohort for audit, but this release's constant prevents it from authorizing
// adaptive traffic.
func AssignOpenAIRouteRolloutCohort(req OpenAIRouteRolloutAssignmentRequest) OpenAIRouteRolloutAssignment {
	return assignOpenAIRouteRolloutCohort(req, OpenAIRouteEnforceCodeAvailable)
}

func assignOpenAIRouteRolloutCohort(
	req OpenAIRouteRolloutAssignmentRequest,
	enforceCodeAvailable bool,
) OpenAIRouteRolloutAssignment {
	result := OpenAIRouteRolloutAssignment{
		Reason:               "invalid_scope",
		Stage:                req.Stage,
		Bucket:               -1,
		EnforceCodeAvailable: enforceCodeAvailable,
	}
	if !req.Stage.Valid() || strings.TrimSpace(req.ActivationID) == "" || req.PolicyVersion <= 0 || req.GroupID <= 0 ||
		strings.TrimSpace(req.Model) == "" || !req.RequestClass.Valid() {
		return result
	}
	basisPoints, trafficStage := req.Stage.TrafficBasisPoints()
	result.TrafficBasisPoints = basisPoints
	if !trafficStage {
		result.Reason = "legacy_or_shadow"
		return result
	}
	cohortKey := ""
	switch req.RequestClass {
	case OpenAIRouteRequestClassText:
		result.BucketBasis = "text_affinity"
		cohortKey = strings.TrimSpace(req.TextAffinityKey)
		if cohortKey == "" {
			result.Reason = "text_affinity_missing"
			return result
		}
	case OpenAIRouteRequestClassImage:
		result.BucketBasis = "image_request"
		cohortKey = strings.TrimSpace(req.ImageRequestID)
		if cohortKey == "" {
			result.Reason = "image_request_id_missing"
			return result
		}
	default:
		return result
	}
	result.Bucket = openAIRouteRolloutBucket(req, result.BucketBasis, cohortKey)
	result.CohortEligible = result.Bucket < basisPoints
	if !result.CohortEligible {
		result.Reason = "outside_canary_cohort"
		return result
	}
	if !enforceCodeAvailable {
		result.Reason = "enforce_release_guard_disabled"
		return result
	}
	result.UseAdaptive = true
	result.Reason = "inside_authorized_canary_cohort"
	return result
}

func openAIRouteRolloutBucket(req OpenAIRouteRolloutAssignmentRequest, basis, cohortKey string) int {
	canonical := fmt.Sprintf("route-rollout-v1|%s|%d|%d|%s|%s|%s|%s",
		strings.TrimSpace(req.ActivationID),
		req.PolicyVersion,
		req.GroupID,
		strings.TrimSpace(req.Model),
		req.RequestClass,
		basis,
		cohortKey,
	)
	sum := sha256.Sum256([]byte(canonical))
	value := binary.BigEndian.Uint64(sum[:8])
	// Multiplication-high maps the complete uint64 range monotonically into
	// 10,000 buckets without modulo bias.
	bucket, _ := bits.Mul64(value, openAIRouteRolloutBucketCount)
	return int(bucket)
}

type OpenAIRouteReviewPhase string

const (
	OpenAIRouteReviewWaitingForT0  OpenAIRouteReviewPhase = "waiting_for_t0"
	OpenAIRouteReviewCheckpoint24H OpenAIRouteReviewPhase = "checkpoint_24h"
	OpenAIRouteReviewPrimary72H    OpenAIRouteReviewPhase = "primary_review_72h"
	OpenAIRouteReviewRetry24H      OpenAIRouteReviewPhase = "retry_review_24h"
	OpenAIRouteReviewManual        OpenAIRouteReviewPhase = "manual_review"
	OpenAIRouteReviewStopped       OpenAIRouteReviewPhase = "stopped"
)

type OpenAIRouteReviewHeartbeatInput struct {
	Now                 time.Time
	EvidenceStartAt     time.Time
	StoppedAt           time.Time
	LastCheckpointAt    time.Time
	LastAssessmentAt    time.Time
	LastAssessmentReady bool
}

type OpenAIRouteReviewHeartbeat struct {
	Phase                OpenAIRouteReviewPhase `json:"phase"`
	DueAt                time.Time              `json:"due_at,omitempty"`
	Due                  bool                   `json:"due"`
	OverdueSeconds       int64                  `json:"overdue_seconds"`
	ReadOnly             bool                   `json:"read_only"`
	AutomaticPromotion   bool                   `json:"automatic_promotion"`
	ManualReviewRequired bool                   `json:"manual_review_required"`
	Timezone             string                 `json:"timezone"`
}

// BuildOpenAIRouteReviewHeartbeat deterministically tells an external
// one-shot wakeup what read-only action is due. It does not create a timer,
// mutate policy, or promote traffic.
func BuildOpenAIRouteReviewHeartbeat(input OpenAIRouteReviewHeartbeatInput) (OpenAIRouteReviewHeartbeat, error) {
	heartbeat := OpenAIRouteReviewHeartbeat{
		ReadOnly:           true,
		AutomaticPromotion: false,
		Timezone:           "Asia/Shanghai",
	}
	if input.Now.IsZero() {
		return heartbeat, fmt.Errorf("%w: heartbeat now is required", ErrOpenAIRouteInvalidRollout)
	}
	now := input.Now.UTC()
	for _, timestamp := range []time.Time{input.StoppedAt, input.LastCheckpointAt, input.LastAssessmentAt} {
		if !timestamp.IsZero() && timestamp.After(now) {
			return heartbeat, fmt.Errorf("%w: heartbeat history cannot be in the future", ErrOpenAIRouteInvalidRollout)
		}
	}
	if input.LastAssessmentReady && input.LastAssessmentAt.IsZero() {
		return heartbeat, fmt.Errorf("%w: ready assessment timestamp is required", ErrOpenAIRouteInvalidRollout)
	}
	if !input.StoppedAt.IsZero() {
		heartbeat.Phase = OpenAIRouteReviewStopped
		return heartbeat, nil
	}
	if input.EvidenceStartAt.IsZero() {
		heartbeat.Phase = OpenAIRouteReviewWaitingForT0
		return heartbeat, nil
	}
	start := input.EvidenceStartAt.UTC()
	if (!input.LastCheckpointAt.IsZero() && input.LastCheckpointAt.Before(start)) ||
		(!input.LastAssessmentAt.IsZero() && input.LastAssessmentAt.Before(start)) {
		return heartbeat, fmt.Errorf("%w: heartbeat history predates T0", ErrOpenAIRouteInvalidRollout)
	}
	if now.Before(start) {
		heartbeat.Phase = OpenAIRouteReviewWaitingForT0
		heartbeat.DueAt = start
		return heartbeat, nil
	}
	checkpointDue := start.Add(openAIRoutePromotionInitialCheckpoint)
	if input.LastCheckpointAt.IsZero() || input.LastCheckpointAt.Before(checkpointDue) {
		heartbeat.Phase = OpenAIRouteReviewCheckpoint24H
		heartbeat.DueAt = checkpointDue
		setOpenAIRouteHeartbeatDue(&heartbeat, now)
		return heartbeat, nil
	}
	primaryDue := start.Add(openAIRoutePromotionMinimumWindow)
	if input.LastAssessmentAt.IsZero() || input.LastAssessmentAt.Before(primaryDue) {
		heartbeat.Phase = OpenAIRouteReviewPrimary72H
		heartbeat.DueAt = primaryDue
		setOpenAIRouteHeartbeatDue(&heartbeat, now)
		return heartbeat, nil
	}
	if input.LastAssessmentReady {
		heartbeat.Phase = OpenAIRouteReviewManual
		heartbeat.ManualReviewRequired = true
		return heartbeat, nil
	}
	heartbeat.Phase = OpenAIRouteReviewRetry24H
	heartbeat.DueAt = input.LastAssessmentAt.UTC().Add(openAIRoutePromotionRetryInterval)
	setOpenAIRouteHeartbeatDue(&heartbeat, now)
	return heartbeat, nil
}

func setOpenAIRouteHeartbeatDue(heartbeat *OpenAIRouteReviewHeartbeat, now time.Time) {
	if heartbeat == nil || heartbeat.DueAt.IsZero() || now.Before(heartbeat.DueAt) {
		return
	}
	heartbeat.Due = true
	heartbeat.OverdueSeconds = int64(now.Sub(heartbeat.DueAt) / time.Second)
}
