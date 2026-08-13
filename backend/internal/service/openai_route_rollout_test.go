package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testOpenAIRouteRolloutAssignment(stage OpenAIRouteRolloutStage, class OpenAIRouteRequestClass) OpenAIRouteRolloutAssignmentRequest {
	return OpenAIRouteRolloutAssignmentRequest{
		ActivationID:    "activation-20260813-gpt-text-v4",
		PolicyVersion:   4,
		GroupID:         7,
		Model:           "gpt-5.6-sol",
		RequestClass:    class,
		Stage:           stage,
		TextAffinityKey: "session-stable-123",
		ImageRequestID:  "request-image-456",
	}
}

func TestAssignOpenAIRouteRolloutCohortReleaseGuardAlwaysFailsClosed(t *testing.T) {
	req := testOpenAIRouteRolloutAssignment(OpenAIRouteRolloutCanary100, OpenAIRouteRequestClassText)
	decision := AssignOpenAIRouteRolloutCohort(req)

	require.True(t, decision.CohortEligible)
	require.False(t, decision.UseAdaptive)
	require.False(t, decision.EnforceCodeAvailable)
	require.Equal(t, "enforce_release_guard_disabled", decision.Reason)
	require.Equal(t, "text_affinity", decision.BucketBasis)
	require.GreaterOrEqual(t, decision.Bucket, 0)
	require.Less(t, decision.Bucket, 10_000)
}

func TestAssignOpenAIRouteRolloutCohortIsMonotonicAcrossStages(t *testing.T) {
	stages := []OpenAIRouteRolloutStage{
		OpenAIRouteRolloutCanary1,
		OpenAIRouteRolloutCanary5,
		OpenAIRouteRolloutCanary20,
		OpenAIRouteRolloutCanary50,
		OpenAIRouteRolloutCanary100,
	}
	lastEligible := false
	bucket := -1
	for _, stage := range stages {
		decision := assignOpenAIRouteRolloutCohort(testOpenAIRouteRolloutAssignment(stage, OpenAIRouteRequestClassText), true)
		if bucket < 0 {
			bucket = decision.Bucket
		}
		require.Equal(t, bucket, decision.Bucket, stage)
		if lastEligible {
			require.True(t, decision.CohortEligible, "a cohort admitted at a lower stage must remain admitted")
		}
		lastEligible = decision.CohortEligible
		require.Equal(t, decision.CohortEligible, decision.UseAdaptive)
	}
	require.True(t, lastEligible, "100 percent stage must admit every valid bucket")
}

func TestAssignOpenAIRouteRolloutCohortPreservesTextAffinityAndImageStatelessness(t *testing.T) {
	text := testOpenAIRouteRolloutAssignment(OpenAIRouteRolloutCanary50, OpenAIRouteRequestClassText)
	firstText := assignOpenAIRouteRolloutCohort(text, true)
	text.ImageRequestID = "ignored-other-request"
	secondText := assignOpenAIRouteRolloutCohort(text, true)
	require.Equal(t, firstText.Bucket, secondText.Bucket)
	require.Equal(t, "text_affinity", firstText.BucketBasis)

	image := testOpenAIRouteRolloutAssignment(OpenAIRouteRolloutCanary50, OpenAIRouteRequestClassImage)
	firstImage := assignOpenAIRouteRolloutCohort(image, true)
	image.TextAffinityKey = "ignored-other-session"
	secondImage := assignOpenAIRouteRolloutCohort(image, true)
	require.Equal(t, firstImage.Bucket, secondImage.Bucket)
	require.Equal(t, "image_request", firstImage.BucketBasis)

	// Image requests are single-request cohorts. Use several deterministic IDs
	// rather than assuming any one pair cannot collide in 10,000 buckets.
	different := false
	for idx := 0; idx < 16; idx++ {
		image.ImageRequestID = fmt.Sprintf("another-image-request-%d", idx)
		if assignOpenAIRouteRolloutCohort(image, true).Bucket != firstImage.Bucket {
			different = true
			break
		}
	}
	require.True(t, different)
}

func TestAssignOpenAIRouteRolloutCohortMissingIdentityFailsClosed(t *testing.T) {
	text := testOpenAIRouteRolloutAssignment(OpenAIRouteRolloutCanary100, OpenAIRouteRequestClassText)
	text.TextAffinityKey = ""
	decision := assignOpenAIRouteRolloutCohort(text, true)
	require.False(t, decision.UseAdaptive)
	require.Equal(t, "text_affinity_missing", decision.Reason)

	image := testOpenAIRouteRolloutAssignment(OpenAIRouteRolloutCanary100, OpenAIRouteRequestClassImage)
	image.ImageRequestID = ""
	decision = assignOpenAIRouteRolloutCohort(image, true)
	require.False(t, decision.UseAdaptive)
	require.Equal(t, "image_request_id_missing", decision.Reason)
}

func testOpenAIRouteRolloutTransition(now time.Time) OpenAIRouteRolloutTransitionRequest {
	windowStart := now.Add(-73 * time.Hour)
	windowEnd := windowStart.Add(72 * time.Hour)
	assessedAt := windowEnd.Add(10 * time.Minute)
	approvedAt := assessedAt.Add(10 * time.Minute)
	return OpenAIRouteRolloutTransitionRequest{
		ActivationID:          "activation-20260813-gpt-text-v4",
		PolicyVersion:         4,
		RequestClass:          OpenAIRouteRequestClassText,
		CurrentStage:          OpenAIRouteRolloutShadow,
		RequestedStage:        OpenAIRouteRolloutCanary1,
		CurrentStageStartedAt: windowStart,
		RequestedAt:           now,
		CodePathRevision:      "release-that-introduced-dormant-code",
		DeploymentRevision:    "later-enforce-capable-release",
		Evidence: OpenAIRouteRolloutEvidence{
			ActivationID:           "activation-20260813-gpt-text-v4",
			PolicyVersion:          4,
			RequestClass:           OpenAIRouteRequestClassText,
			ApprovedStage:          OpenAIRouteRolloutCanary1,
			WindowStart:            windowStart,
			WindowEnd:              windowEnd,
			AssessedAt:             assessedAt,
			AutomatedEvidenceReady: true,
			ManualApprovalID:       "owner-approval-canary-1",
			ManualApprovedAt:       approvedAt,
			SourceRevision:         "later-enforce-capable-release",
		},
	}
}

func TestAssessOpenAIRouteRolloutTransitionRequiresSeparateRelease(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	req := testOpenAIRouteRolloutTransition(now)

	// Public assessment is hard-disabled in the release that introduces this
	// contract even when all evidence and approvals are otherwise valid.
	assessment := AssessOpenAIRouteRolloutTransition(req)
	require.False(t, assessment.Allowed)
	require.Equal(t, "enforce_release_guard_disabled", assessment.Reason)
	require.False(t, assessment.EnforceCodeAvailable)

	assessment = assessOpenAIRouteRolloutTransition(req, true)
	require.True(t, assessment.Allowed)
	require.Equal(t, "explicit_stage_transition_authorized", assessment.Reason)
	require.True(t, assessment.ManualApprovalRequired)
	require.False(t, assessment.AutomaticPromotion)

	req.CodePathRevision = req.DeploymentRevision
	assessment = assessOpenAIRouteRolloutTransition(req, true)
	require.False(t, assessment.Allowed)
	require.Equal(t, "separate_release_required", assessment.Reason)
}

func TestAssessOpenAIRouteRolloutTransitionForbidsSkippingAndApprovalReplay(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	req := testOpenAIRouteRolloutTransition(now)

	req.RequestedStage = OpenAIRouteRolloutCanary5
	assessment := assessOpenAIRouteRolloutTransition(req, true)
	require.False(t, assessment.Allowed)
	require.Equal(t, "stage_skip_forbidden", assessment.Reason)

	req = testOpenAIRouteRolloutTransition(now)
	req.Evidence.ApprovedStage = OpenAIRouteRolloutCanary5
	assessment = assessOpenAIRouteRolloutTransition(req, true)
	require.False(t, assessment.Allowed)
	require.Equal(t, "evidence_scope_mismatch", assessment.Reason)
}

func TestAssessOpenAIRouteRolloutTransitionRequiresDwellAndOneClickRollback(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	req := testOpenAIRouteRolloutTransition(now)
	req.CurrentStageStartedAt = now.Add(-23*time.Hour - 59*time.Minute)
	assessment := assessOpenAIRouteRolloutTransition(req, true)
	require.False(t, assessment.Allowed)
	require.Equal(t, "minimum_stage_dwell_not_met", assessment.Reason)

	rollback := OpenAIRouteRolloutTransitionRequest{
		CurrentStage:   OpenAIRouteRolloutCanary50,
		RequestedStage: OpenAIRouteRolloutLegacy,
	}
	assessment = assessOpenAIRouteRolloutTransition(rollback, false)
	require.True(t, assessment.Allowed)
	require.Equal(t, "rollback_to_legacy", assessment.Reason)
}

func TestAssessOpenAIRouteRolloutTransitionRequiresExplicitShadowApproval(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	req := OpenAIRouteRolloutTransitionRequest{
		ActivationID:   "activation-20260813-gpt-text-v4",
		PolicyVersion:  4,
		RequestClass:   OpenAIRouteRequestClassText,
		CurrentStage:   OpenAIRouteRolloutLegacy,
		RequestedStage: OpenAIRouteRolloutShadow,
		RequestedAt:    now,
	}
	assessment := assessOpenAIRouteRolloutTransition(req, false)
	require.False(t, assessment.Allowed)
	require.Equal(t, "shadow_activation_approval_missing", assessment.Reason)

	req.ActivationApprovalID = "owner-approval-shadow-t0"
	assessment = assessOpenAIRouteRolloutTransition(req, false)
	require.True(t, assessment.Allowed)
	require.Equal(t, "explicit_shadow_activation_authorized", assessment.Reason)
}

func TestAssessOpenAIRouteRolloutTransitionUsesDailyCanaryEvidenceAfterShadow(t *testing.T) {
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	req := testOpenAIRouteRolloutTransition(now)
	req.CurrentStage = OpenAIRouteRolloutCanary1
	req.RequestedStage = OpenAIRouteRolloutCanary5
	req.CurrentStageStartedAt = now.Add(-25 * time.Hour)
	req.Evidence.ApprovedStage = OpenAIRouteRolloutCanary5
	req.Evidence.WindowStart = now.Add(-25 * time.Hour)
	req.Evidence.WindowEnd = now.Add(-time.Hour)
	req.Evidence.AssessedAt = now.Add(-50 * time.Minute)
	req.Evidence.ManualApprovedAt = now.Add(-30 * time.Minute)
	req.Evidence.ManualApprovalID = "owner-approval-canary-5"

	assessment := assessOpenAIRouteRolloutTransition(req, true)
	require.True(t, assessment.Allowed)
	require.Equal(t, "explicit_stage_transition_authorized", assessment.Reason)
}

func TestBuildOpenAIRouteReviewHeartbeatLifecycle(t *testing.T) {
	start := time.Date(2026, 8, 13, 0, 0, 0, 0, time.UTC)

	heartbeat, err := BuildOpenAIRouteReviewHeartbeat(OpenAIRouteReviewHeartbeatInput{Now: start})
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteReviewWaitingForT0, heartbeat.Phase)
	require.True(t, heartbeat.ReadOnly)
	require.False(t, heartbeat.AutomaticPromotion)

	heartbeat, err = BuildOpenAIRouteReviewHeartbeat(OpenAIRouteReviewHeartbeatInput{
		Now:             start.Add(24 * time.Hour),
		EvidenceStartAt: start,
	})
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteReviewCheckpoint24H, heartbeat.Phase)
	require.True(t, heartbeat.Due)
	require.Equal(t, int64(0), heartbeat.OverdueSeconds)

	heartbeat, err = BuildOpenAIRouteReviewHeartbeat(OpenAIRouteReviewHeartbeatInput{
		Now:              start.Add(71 * time.Hour),
		EvidenceStartAt:  start,
		LastCheckpointAt: start.Add(24 * time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteReviewPrimary72H, heartbeat.Phase)
	require.False(t, heartbeat.Due)
	require.Equal(t, start.Add(72*time.Hour), heartbeat.DueAt)

	heartbeat, err = BuildOpenAIRouteReviewHeartbeat(OpenAIRouteReviewHeartbeatInput{
		Now:                 start.Add(96 * time.Hour),
		EvidenceStartAt:     start,
		LastCheckpointAt:    start.Add(24 * time.Hour),
		LastAssessmentAt:    start.Add(72 * time.Hour),
		LastAssessmentReady: false,
	})
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteReviewRetry24H, heartbeat.Phase)
	require.True(t, heartbeat.Due)
	require.Equal(t, start.Add(96*time.Hour), heartbeat.DueAt)

	heartbeat, err = BuildOpenAIRouteReviewHeartbeat(OpenAIRouteReviewHeartbeatInput{
		Now:                 start.Add(73 * time.Hour),
		EvidenceStartAt:     start,
		LastCheckpointAt:    start.Add(24 * time.Hour),
		LastAssessmentAt:    start.Add(72 * time.Hour),
		LastAssessmentReady: true,
	})
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteReviewManual, heartbeat.Phase)
	require.True(t, heartbeat.ManualReviewRequired)
	require.False(t, heartbeat.Due)
}

func TestBuildOpenAIRouteReviewHeartbeatStoppedNeverSchedulesWork(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	heartbeat, err := BuildOpenAIRouteReviewHeartbeat(OpenAIRouteReviewHeartbeatInput{
		Now:             now,
		EvidenceStartAt: now.Add(-72 * time.Hour),
		StoppedAt:       now.Add(-time.Hour),
	})
	require.NoError(t, err)
	require.Equal(t, OpenAIRouteReviewStopped, heartbeat.Phase)
	require.False(t, heartbeat.Due)
	require.True(t, heartbeat.DueAt.IsZero())
}

func TestBuildOpenAIRouteReviewHeartbeatRejectsFutureHistory(t *testing.T) {
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	_, err := BuildOpenAIRouteReviewHeartbeat(OpenAIRouteReviewHeartbeatInput{
		Now:                 now,
		EvidenceStartAt:     now.Add(-72 * time.Hour),
		LastAssessmentAt:    now.Add(time.Minute),
		LastAssessmentReady: true,
	})
	require.ErrorIs(t, err, ErrOpenAIRouteInvalidRollout)
}
