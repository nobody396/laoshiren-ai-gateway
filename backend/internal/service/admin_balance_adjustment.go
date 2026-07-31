package service

import (
	"context"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
)

var (
	// ErrAdminBalanceSourceReversalRequired prevents a manual balance decrease
	// from leaving affiliate-eligible FIFO lots behind. Those lots must be
	// reversed through their original paid source so balance and attribution
	// remain consistent.
	ErrAdminBalanceSourceReversalRequired = infraerrors.Conflict(
		"ADMIN_BALANCE_SOURCE_REVERSAL_REQUIRED",
		"该笔减额无法安全匹配到足额的非分润余额（可能包含可分润额度），不能直接减少；请从原充值订单或卡密记录发起来源冲正。",
	)
	errAdminBalanceAdjustmentGuardUnavailable = infraerrors.InternalServer(
		"ADMIN_BALANCE_ADJUSTMENT_GUARD_UNAVAILABLE",
		"管理员余额调整安全校验暂不可用，请稍后重试。",
	)
)

type AdminBalanceAdjustmentResult struct {
	OldBalance float64
	NewBalance float64
}

// adminBalanceAdjustmentRepository is deliberately narrower than
// UserRepository. Production repositories must perform the user-row lock,
// affiliate-lot guard, and balance mutation in one transaction.
type adminBalanceAdjustmentRepository interface {
	ApplyAdminBalanceAdjustment(
		ctx context.Context,
		userID int64,
		amount float64,
		operation string,
	) (*AdminBalanceAdjustmentResult, error)
}
