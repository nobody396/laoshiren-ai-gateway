package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type affiliateAgentRepoStub struct {
	qualification *AffiliateAgentQualification
	autoResult    *AffiliateAgentReviewResult
	autoResults   map[int64]*AffiliateAgentReviewResult
	autoErrors    map[int64]error
	candidates    []AffiliateQualifiedCandidate
	autoCalls     int
	reviewResult  *AffiliateAgentReviewResult
	reviewCalls   int
}

func (r *affiliateAgentRepoStub) GetAgentQualification(context.Context, int64) (*AffiliateAgentQualification, error) {
	return r.qualification, nil
}

func (r *affiliateAgentRepoStub) SubmitAgentApplication(context.Context, int64, string) (*AffiliateAgentApplication, error) {
	return nil, ErrAffiliateQualificationNotMet
}

func (r *affiliateAgentRepoStub) ListAgentApplications(context.Context, string, int) ([]AffiliateAgentApplication, error) {
	return nil, nil
}

func (r *affiliateAgentRepoStub) ListQualifiedCandidates(context.Context, int) ([]AffiliateQualifiedCandidate, error) {
	return r.candidates, nil
}

func (r *affiliateAgentRepoStub) GetOperationsSummary(context.Context) (*AffiliateOperationsSummary, error) {
	return nil, nil
}

func (r *affiliateAgentRepoStub) ListPartnerPerformance(context.Context, int) ([]AffiliatePartnerPerformance, error) {
	return nil, nil
}

func (r *affiliateAgentRepoStub) GetPartnerPerformance(context.Context, int64, time.Time, time.Time) (*AffiliatePartnerPerformanceDetail, error) {
	return nil, nil
}

func (r *affiliateAgentRepoStub) ReviewAgentApplication(context.Context, int64, bool, string, int64, string, int32) (*AffiliateAgentReviewResult, error) {
	r.reviewCalls++
	return r.reviewResult, nil
}

func (r *affiliateAgentRepoStub) AutoActivateAgent(_ context.Context, userID int64, _ string, _ string, _ int32) (*AffiliateAgentReviewResult, error) {
	r.autoCalls++
	if err, ok := r.autoErrors[userID]; ok {
		return nil, err
	}
	if result, ok := r.autoResults[userID]; ok {
		return result, nil
	}
	return r.autoResult, nil
}

type activationUserLookupStub struct {
	user *User
}

func (s activationUserLookupStub) GetByID(context.Context, int64) (*User, error) {
	return s.user, nil
}

type activationSettingsStub struct{}

func (activationSettingsStub) GetFrontendURL(context.Context) string {
	return "https://laoshirenai.com/"
}
func (activationSettingsStub) GetSiteName(context.Context) string { return "老实人AI" }

type activationEmailQueueStub struct {
	emails []string
	urls   []string
	sites  []string
}

func (q *activationEmailQueueStub) EnqueueAffiliateAgentActivated(email, siteName, partnerCenterURL string) error {
	q.emails = append(q.emails, email)
	q.sites = append(q.sites, siteName)
	q.urls = append(q.urls, partnerCenterURL)
	return nil
}

func qualifiedAffiliateSnapshot() *AffiliateAgentQualification {
	startedAt := time.Now().Add(-time.Hour)
	return &AffiliateAgentQualification{
		UserID:             42,
		ProgramMode:        AffiliateProgramModeLive,
		ProgramStartedAt:   &startedAt,
		AgentStatus:        "qualified",
		RiskStatus:         "clear",
		Qualified:          true,
		QualificationRoute: "self_consumption",
	}
}

func newActivationTestService(repo *affiliateAgentRepoStub, queue *activationEmailQueueStub) *AffiliateAgentService {
	svc := NewAffiliateAgentService(repo)
	svc.SetActivationNotificationDeps(
		activationUserLookupStub{user: &User{ID: 42, Email: "partner@example.com"}},
		activationSettingsStub{},
		queue,
	)
	return svc
}

func TestAffiliateAgentService_AutoActivationEnqueuesEmailOnce(t *testing.T) {
	repo := &affiliateAgentRepoStub{
		qualification: qualifiedAffiliateSnapshot(),
		autoResult: &AffiliateAgentReviewResult{
			NewlyActivated: true,
			Application:    AffiliateAgentApplication{ID: 7, UserID: 42, Status: "approved"},
		},
	}
	queue := &activationEmailQueueStub{}
	svc := newActivationTestService(repo, queue)

	qualification, err := svc.GetQualification(context.Background(), 42)
	require.NoError(t, err)
	require.NotNil(t, qualification)
	require.Equal(t, 1, repo.autoCalls)
	require.Equal(t, []string{"partner@example.com"}, queue.emails)
	require.Equal(t, []string{"老实人AI"}, queue.sites)
	require.Equal(t, []string{"https://laoshirenai.com/affiliate"}, queue.urls)
}

func TestAffiliateAgentService_IdempotentActivationSkipsEmail(t *testing.T) {
	repo := &affiliateAgentRepoStub{
		qualification: qualifiedAffiliateSnapshot(),
		autoResult: &AffiliateAgentReviewResult{
			NewlyActivated: false,
			Application:    AffiliateAgentApplication{ID: 7, UserID: 42, Status: "approved"},
		},
	}
	queue := &activationEmailQueueStub{}
	svc := newActivationTestService(repo, queue)

	_, err := svc.GetQualification(context.Background(), 42)
	require.NoError(t, err)
	require.Equal(t, 1, repo.autoCalls)
	require.Empty(t, queue.emails, "idempotent repair must not resend the activation email")
}

func TestAffiliateAgentService_AdminApprovalEnqueuesActivationEmail(t *testing.T) {
	repo := &affiliateAgentRepoStub{
		reviewResult: &AffiliateAgentReviewResult{
			NewlyActivated: true,
			Application:    AffiliateAgentApplication{ID: 9, UserID: 42, Status: "approved"},
		},
	}
	queue := &activationEmailQueueStub{}
	svc := newActivationTestService(repo, queue)

	_, err := svc.ReviewApplication(context.Background(), 9, true, "资料无误", 1)
	require.NoError(t, err)
	require.Equal(t, 1, repo.reviewCalls)
	require.Equal(t, []string{"partner@example.com"}, queue.emails)
}

func TestAffiliateAgentService_NotQualifiedSkipsActivationAndEmail(t *testing.T) {
	repo := &affiliateAgentRepoStub{
		qualification: &AffiliateAgentQualification{
			UserID:      42,
			ProgramMode: AffiliateProgramModeLive,
			AgentStatus: "not_qualified",
			RiskStatus:  "clear",
			Qualified:   false,
		},
	}
	queue := &activationEmailQueueStub{}
	svc := newActivationTestService(repo, queue)

	_, err := svc.GetQualification(context.Background(), 42)
	require.NoError(t, err)
	require.Zero(t, repo.autoCalls)
	require.Empty(t, queue.emails)
}

func TestBuildAffiliateAgentActivatedEmailBody(t *testing.T) {
	body := buildAffiliateAgentActivatedEmailBody("老实人AI", "https://laoshirenai.com/affiliate")
	require.Contains(t, body, "合伙人已开通")
	require.Contains(t, body, "https://laoshirenai.com/affiliate")
	require.Contains(t, body, "5%")
	require.Contains(t, body, "老实人AI")
}
