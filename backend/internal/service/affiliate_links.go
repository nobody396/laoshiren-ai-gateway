package service

import (
	"context"
	"errors"
	"strings"
	"time"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

var (
	ErrAffiliateAgentNotActive      = infraerrors.Forbidden("AFFILIATE_AGENT_NOT_ACTIVE", "affiliate agent is not active")
	ErrAffiliateLinkNotFound        = infraerrors.NotFound("AFFILIATE_LINK_NOT_FOUND", "affiliate link not found")
	ErrAffiliateLinkLimit           = infraerrors.Conflict("AFFILIATE_LINK_LIMIT", "affiliate campaign link limit reached")
	ErrAffiliateLinkConflict        = infraerrors.Conflict("AFFILIATE_LINK_CONFLICT", "affiliate link code conflict")
	ErrAffiliateDefaultLinkRequired = infraerrors.Conflict(
		"AFFILIATE_DEFAULT_LINK_REQUIRED",
		"default affiliate link must remain active",
	)
)

type AffiliateLink struct {
	ID                     int64     `json:"id"`
	AgentID                int64     `json:"agent_id"`
	Code                   string    `json:"code"`
	Name                   string    `json:"name"`
	Channel                string    `json:"channel"`
	IsDefault              bool      `json:"is_default"`
	Status                 string    `json:"status"`
	RateVersion            int32     `json:"rate_version"`
	CustomerRebateRateBPS  int32     `json:"customer_rebate_rate_bps"`
	AgentCommissionRateBPS int32     `json:"agent_commission_rate_bps"`
	CreatedAt              time.Time `json:"created_at"`
	UpdatedAt              time.Time `json:"updated_at"`
}

type AffiliateLinkReferral struct {
	AgentID                int64
	LinkID                 int64
	RateVersion            int32
	CustomerRebateRateBPS  int32
	AgentCommissionRateBPS int32
}

type CreateAffiliateLinkInput struct {
	AgentID               int64
	Code                  string
	Name                  string
	Channel               string
	CustomerRebateRateBPS int32
	CreatedBy             int64
	IsDefault             bool
}

type AffiliateLinkRepository interface {
	ResolveActiveLink(ctx context.Context, code string) (*AffiliateLinkReferral, error)
	ResolveDefaultAgentLink(ctx context.Context, agentID int64) (*AffiliateLinkReferral, error)
	BindAgentReferral(ctx context.Context, customerUserID int64, referral AffiliateLinkReferral) error
	ListLinks(ctx context.Context, agentID int64) ([]AffiliateLink, error)
	CreateLink(ctx context.Context, input CreateAffiliateLinkInput) (*AffiliateLink, error)
	UpdateLinkRate(ctx context.Context, agentID, linkID int64, customerRebateRateBPS int32, changedBy int64) (*AffiliateLink, error)
	SetLinkStatus(ctx context.Context, agentID, linkID int64, status string) (*AffiliateLink, error)
}

type AffiliateLinkService struct {
	repo AffiliateLinkRepository
}

func NewAffiliateLinkService(repo AffiliateLinkRepository) *AffiliateLinkService {
	return &AffiliateLinkService{repo: repo}
}

func (s *AffiliateLinkService) List(ctx context.Context, agentID int64) ([]AffiliateLink, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate link repository is not configured")
	}
	return s.repo.ListLinks(ctx, agentID)
}

func (s *AffiliateLinkService) Create(
	ctx context.Context,
	agentID int64,
	name string,
	channel string,
	customerRebateRateBPS int32,
) (*AffiliateLink, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("affiliate link repository is not configured")
	}
	name = strings.TrimSpace(name)
	channel = strings.TrimSpace(channel)
	if name == "" || len(name) > 100 || len(channel) > 100 {
		return nil, ErrInvalidInput
	}
	if err := validateAffiliateCustomerRate(customerRebateRateBPS); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 10; attempt++ {
		code, err := generateInviteCode()
		if err != nil {
			return nil, err
		}
		link, err := s.repo.CreateLink(ctx, CreateAffiliateLinkInput{
			AgentID:               agentID,
			Code:                  "A" + code,
			Name:                  name,
			Channel:               channel,
			CustomerRebateRateBPS: customerRebateRateBPS,
			CreatedBy:             agentID,
		})
		if errors.Is(err, ErrAffiliateLinkConflict) {
			continue
		}
		return link, err
	}
	return nil, ErrAffiliateLinkConflict
}

func (s *AffiliateLinkService) UpdateRate(
	ctx context.Context,
	agentID int64,
	linkID int64,
	customerRebateRateBPS int32,
) (*AffiliateLink, error) {
	if err := validateAffiliateCustomerRate(customerRebateRateBPS); err != nil {
		return nil, err
	}
	return s.repo.UpdateLinkRate(ctx, agentID, linkID, customerRebateRateBPS, agentID)
}

func (s *AffiliateLinkService) SetStatus(
	ctx context.Context,
	agentID int64,
	linkID int64,
	status string,
) (*AffiliateLink, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "disabled" {
		status = "paused"
	}
	if status != "active" && status != "paused" {
		return nil, infraerrors.BadRequest("AFFILIATE_LINK_INVALID_STATUS", "合伙人链接状态只能是使用中或已停用")
	}
	return s.repo.SetLinkStatus(ctx, agentID, linkID, status)
}

func validateAffiliateCustomerRate(rateBPS int32) error {
	if rateBPS < 0 || rateBPS > AffiliateAgentPoolRateBPS || rateBPS%100 != 0 {
		return infraerrors.BadRequest("AFFILIATE_LINK_INVALID_RATE", "客户返利比例必须是 0% 到 10%，并按 1% 调整")
	}
	return nil
}
