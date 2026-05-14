package service

import (
	"context"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

// CommissionRecord 分佣记录
type CommissionRecord struct {
	ID            int64     `json:"id"`
	BeneficiaryID int64     `json:"beneficiary_id"` // 获得分佣/奖励的用户
	UserID        int64     `json:"user_id"`        // 触发分佣的用户（消费方/充值方）
	Amount        float64   `json:"amount"`         // 分佣/奖励金额
	SourceAmount  float64   `json:"source_amount"`  // 原始触发金额
	Type          string    `json:"type"`           // 分佣类型，见 domain.CommissionType* 常量
	Rate          float64   `json:"rate,omitempty"` // 生成该流水时使用的比例快照
	RateSource    string    `json:"rate_source,omitempty"`
	SourceID      *int64    `json:"source_id,omitempty"` // 关联来源 ID（usage_log.id 或 redeem_code.id）
	Note          *string   `json:"note,omitempty"`      // 备注
	CreatedAt     time.Time `json:"created_at"`
}

// InvitedUserStat 代理商邀请的用户消费统计（用于代理商后台）
type InvitedUserStat struct {
	UserID       int64     `json:"user_id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	RegisteredAt time.Time `json:"joined_at"`
	// 指定日期范围内的消费总额（usage_logs.actual_cost 汇总）
	ConsumedAmount float64 `json:"total_consumption"`
	// 产生的分佣总额（commission_records.amount 汇总，兼容 consumption / consumption_commission）
	CommissionAmount float64 `json:"total_commission"`
}

// AgentDashboard 代理商总览统计
type AgentDashboard struct {
	InvitedUserCount         int64   `json:"invited_user_count"`          // 邀请的用户总数
	TotalCommission          float64 `json:"total_commission"`            // 累计分佣总额（所有类型）
	SettledCommission        float64 `json:"settled_commission"`          // 已结算金额
	UnsettledCommission      float64 `json:"unsettled_commission"`        // 未结算金额
	PeriodCommission         float64 `json:"period_commission"`           // 指定周期内的分佣总额
	PeriodConsumed           float64 `json:"period_consumed"`             // 指定周期内旗下用户消费总额
	ThisMonthCommission      float64 `json:"this_month_commission"`       // 本月分佣
	ConsumptionRate          float64 `json:"consumption_rate"`            // 当前代理商消耗分润比例
	FirstRechargeInviteeRate float64 `json:"first_recharge_invitee_rate"` // 被邀请用户首充奖励比例
	RateSource               string  `json:"rate_source"`                 // global 或 agent_override
}

// UserReferralDashboard 普通用户邀请看板统计
type UserReferralDashboard struct {
	InvitedUserCount    int64   `json:"invited_user_count"`
	TotalCommission     float64 `json:"total_commission"`
	ThisMonthCommission float64 `json:"this_month_commission"`
}

// CommissionRates 全局分佣/奖励比例配置。比例以小数表示：0.06 = 6%。
type CommissionRates struct {
	ConsumptionRate           float64   `json:"consumption_rate"`
	FirstRechargeInviteeRate  float64   `json:"first_recharge_invitee_rate"`
	FirstRechargeReferralRate float64   `json:"first_recharge_referral_rate"`
	UpdatedAt                 time.Time `json:"updated_at,omitempty"`
}

// AgentRateConfig 单代理商比例覆盖配置。
type AgentRateConfig struct {
	AgentID                  int64      `json:"agent_id"`
	ConsumptionRate          float64    `json:"consumption_rate"`
	Enabled                  bool       `json:"enabled"`
	EffectiveConsumptionRate float64    `json:"effective_consumption_rate"`
	RateSource               string     `json:"rate_source"`
	CreatedAt                *time.Time `json:"created_at,omitempty"`
	UpdatedAt                *time.Time `json:"updated_at,omitempty"`
}

// AgentSettlement 代理商结算流水。结算只扣减未结算佣金，不影响 users.balance。
type AgentSettlement struct {
	ID         int64     `json:"id"`
	AgentID    int64     `json:"agent_id"`
	Amount     float64   `json:"amount"`
	OperatorID int64     `json:"operator_id"`
	Note       string    `json:"note"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type AdminAgentListFilters struct {
	Search string
	Start  *time.Time
	End    *time.Time
}

type AdminAgentSummary struct {
	AgentID                 int64      `json:"agent_id"`
	Email                   string     `json:"email"`
	Username                string     `json:"username"`
	Status                  string     `json:"status"`
	InviteCode              *string    `json:"invite_code,omitempty"`
	CreatedAt               time.Time  `json:"created_at"`
	LastActiveAt            *time.Time `json:"last_active_at,omitempty"`
	InvitedUserCount        int64      `json:"invited_user_count"`
	TotalConsumption        float64    `json:"total_consumption"`
	PeriodConsumption       float64    `json:"period_consumption"`
	TotalCommission         float64    `json:"total_commission"`
	PeriodCommission        float64    `json:"period_commission"`
	ThisMonthCommission     float64    `json:"this_month_commission"`
	SettledCommission       float64    `json:"settled_commission"`
	UnsettledCommission     float64    `json:"unsettled_commission"`
	ConsumptionRate         float64    `json:"consumption_rate"`
	RateSource              string     `json:"rate_source"`
	OverrideEnabled         bool       `json:"override_enabled"`
	OverrideConsumptionRate *float64   `json:"override_consumption_rate,omitempty"`
}

type AdminAgentUserStat struct {
	UserID                int64      `json:"user_id"`
	Email                 string     `json:"email"`
	Username              string     `json:"username"`
	JoinedAt              time.Time  `json:"joined_at"`
	FirstInvitedTopupAt   *time.Time `json:"first_invited_topup_at,omitempty"`
	TotalRecharged        float64    `json:"total_recharged"`
	TotalConsumption      float64    `json:"total_consumption"`
	PeriodConsumption     float64    `json:"period_consumption"`
	TotalCommission       float64    `json:"total_commission"`
	PeriodCommission      float64    `json:"period_commission"`
	CommissionRecordCount int64      `json:"commission_record_count"`
	LastCommissionAt      *time.Time `json:"last_commission_at,omitempty"`
}

type BindAgentUserInput struct {
	Email          string `json:"email"`
	OverwriteAgent bool   `json:"overwrite_agent"`
}

type BindAgentUserResult struct {
	UserID           int64  `json:"user_id"`
	Email            string `json:"email"`
	Username         string `json:"username"`
	AgentID          int64  `json:"agent_id"`
	PreviousAgentID  *int64 `json:"previous_agent_id,omitempty"`
	InviterID        *int64 `json:"inviter_id,omitempty"`
	InviterIDChanged bool   `json:"inviter_id_changed"`
	AlreadyBound     bool   `json:"already_bound"`
	Overwritten      bool   `json:"overwritten"`
}

type AdminAgentCommissionRecord struct {
	CommissionRecord
	UserEmail    string `json:"user_email"`
	UserUsername string `json:"user_username"`
}

// CommissionRateRepository 提供比例配置读取和写入。
type CommissionRateRepository interface {
	GetCommissionRates(ctx context.Context) (*CommissionRates, error)
	UpdateCommissionRates(ctx context.Context, rates *CommissionRates) error
	GetAgentRateConfig(ctx context.Context, agentID int64) (*AgentRateConfig, error)
	UpsertAgentRateConfig(ctx context.Context, config *AgentRateConfig) error
	ResolveAgentConsumptionRate(ctx context.Context, agentID int64) (rate float64, source string, err error)
}

// AgentCommissionAdminRepository 提供管理员代理商面板所需的聚合和结算数据。
type AgentCommissionAdminRepository interface {
	ListAdminAgents(ctx context.Context, params pagination.PaginationParams, filters AdminAgentListFilters) ([]AdminAgentSummary, *pagination.PaginationResult, error)
	GetAdminAgent(ctx context.Context, agentID int64, start, end *time.Time) (*AdminAgentSummary, error)
	ListAdminAgentUsers(ctx context.Context, agentID int64, params pagination.PaginationParams, start, end *time.Time) ([]AdminAgentUserStat, *pagination.PaginationResult, error)
	ListAdminAgentCommissions(ctx context.Context, agentID int64, params pagination.PaginationParams, typeFilter string, start, end *time.Time) ([]AdminAgentCommissionRecord, *pagination.PaginationResult, error)
	ListAgentSettlements(ctx context.Context, agentID int64, params pagination.PaginationParams) ([]AgentSettlement, *pagination.PaginationResult, error)
	SumAgentSettlements(ctx context.Context, agentID int64) (float64, error)
	CreateAgentSettlement(ctx context.Context, settlement *AgentSettlement) error
}

// CommissionRepository 分佣记录数据访问接口
type CommissionRepository interface {
	Create(ctx context.Context, record *CommissionRecord) error

	// ListByBeneficiary 查询某收益人的分佣记录（支持类型过滤和日期范围）
	ListByBeneficiary(
		ctx context.Context,
		beneficiaryID int64,
		params pagination.PaginationParams,
		typeFilter string,
		start, end *time.Time,
	) ([]CommissionRecord, *pagination.PaginationResult, error)

	// SumByBeneficiaryAndPeriod 统计收益人在指定日期范围内的分佣总额
	SumByBeneficiaryAndPeriod(ctx context.Context, beneficiaryID int64, start, end *time.Time) (float64, error)

	// SumByBeneficiaryTypeAndPeriod 统计收益人在指定类型和日期范围内的分佣总额
	SumByBeneficiaryTypeAndPeriod(ctx context.Context, beneficiaryID int64, commType string, start, end *time.Time) (float64, error)

	// ListInvitedUsersWithStats 查询代理商旗下用户列表及其消费/分佣统计
	ListInvitedUsersWithStats(
		ctx context.Context,
		agentID int64,
		params pagination.PaginationParams,
		start, end *time.Time,
	) ([]InvitedUserStat, *pagination.PaginationResult, error)
}
