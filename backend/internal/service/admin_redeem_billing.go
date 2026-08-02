package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/accountchangerecord"
	"github.com/bozhouDev/DragonCode-sub2api/ent/predicate"
	"github.com/bozhouDev/DragonCode-sub2api/ent/redeemcode"
	"github.com/bozhouDev/DragonCode-sub2api/ent/redeemcodebatch"
	"github.com/bozhouDev/DragonCode-sub2api/ent/user"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
)

func (s *adminServiceImpl) ListRedeemCodeBilling(ctx context.Context, page, pageSize int, filters RedeemCodeBillingFilters) (*RedeemCodeBillingResult, error) {
	if s.entClient == nil {
		return nil, errors.New("entClient is nil")
	}

	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	query := s.applyRedeemCodeBillingFilters(s.entClient.RedeemCode.Query(), filters)

	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}

	allForSummary, err := query.Clone().All(ctx)
	if err != nil {
		return nil, err
	}
	ledgerBySourceID, err := s.redeemCodeLedgerMap(ctx, collectRedeemCodeIDs(allForSummary))
	if err != nil {
		return nil, err
	}

	codes, err := query.
		WithUser().
		WithBatch().
		Order(dbent.Desc(redeemcode.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]RedeemCodeBillingItem, 0, len(codes))
	for _, code := range codes {
		items = append(items, redeemCodeBillingItemFromEntity(code, ledgerBySourceID[code.ID]))
	}

	return &RedeemCodeBillingResult{
		Items:    items,
		Summary:  summarizeRedeemCodeBilling(allForSummary, ledgerBySourceID),
		Total:    int64(total),
		Page:     normalizePage(params.Page),
		PageSize: params.Limit(),
		Pages:    calcPages(total, params.Limit()),
	}, nil
}

// ListRedeemCodeClassificationAnomalies returns fail-closed records that were
// redeemed while still classified as unsold inventory. These records require
// evidence review; callers must never infer "sold" from redemption alone.
func (s *adminServiceImpl) ListRedeemCodeClassificationAnomalies(
	ctx context.Context,
) (*RedeemCodeClassificationAnomalyResult, error) {
	if s.entClient == nil {
		return nil, errors.New("entClient is nil")
	}
	query := s.entClient.RedeemCode.Query().Where(
		redeemcode.StatusEQ(StatusUsed),
		redeemcode.PurposeEQ(RedeemCodePurposeSaleRecharge),
		redeemcode.SalesStatusEQ(RedeemCodeSalesStatusInventory),
	)
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, err
	}
	codes, err := query.
		WithUser().
		WithBatch().
		Order(dbent.Desc(redeemcode.FieldID)).
		Limit(200).
		All(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]RedeemCodeBillingItem, 0, len(codes))
	for _, code := range codes {
		items = append(items, redeemCodeBillingItemFromEntity(code, nil))
	}
	return &RedeemCodeClassificationAnomalyResult{Items: items, Total: int64(total)}, nil
}

func (s *adminServiceImpl) applyRedeemCodeBillingFilters(query *dbent.RedeemCodeQuery, filters RedeemCodeBillingFilters) *dbent.RedeemCodeQuery {
	query = query.Where(redeemcode.TypeEQ(RedeemTypeBalance))

	if filters.Purpose != "" {
		query = query.Where(redeemcode.PurposeEQ(filters.Purpose))
	}
	if filters.SalesStatus != "" {
		query = query.Where(redeemcode.SalesStatusEQ(filters.SalesStatus))
	}
	if filters.RedeemStatus != "" {
		query = query.Where(redeemcode.StatusEQ(filters.RedeemStatus))
	}
	if filters.BatchID != nil && *filters.BatchID > 0 {
		query = query.Where(redeemcode.BatchIDEQ(*filters.BatchID))
	}
	if filters.AmountMin != nil {
		query = query.Where(redeemcode.ValueGTE(*filters.AmountMin))
	}
	if filters.AmountMax != nil {
		query = query.Where(redeemcode.ValueLTE(*filters.AmountMax))
	}
	if filters.UsedStartTime != nil {
		query = query.Where(redeemcode.UsedAtGTE(*filters.UsedStartTime))
	}
	if filters.UsedEndTime != nil {
		query = query.Where(redeemcode.UsedAtLT(*filters.UsedEndTime))
	}
	if filters.CreatedStartTime != nil {
		query = query.Where(redeemcode.CreatedAtGTE(*filters.CreatedStartTime))
	}
	if filters.CreatedEndTime != nil {
		query = query.Where(redeemcode.CreatedAtLT(*filters.CreatedEndTime))
	}

	search := strings.TrimSpace(filters.Search)
	if search != "" {
		preds := []predicate.RedeemCode{
			redeemcode.CodeContainsFold(search),
			redeemcode.NotesContainsFold(search),
			redeemcode.SoldToNoteContainsFold(search),
			redeemcode.ExternalOrderNoContainsFold(search),
			redeemcode.ExternalOrderURLContainsFold(search),
			redeemcode.InternalNotesContainsFold(search),
			redeemcode.HasUserWith(user.EmailContainsFold(search)),
			redeemcode.HasUserWith(user.UsernameContainsFold(search)),
			redeemcode.HasBatchWith(redeemcodebatch.NameContainsFold(search)),
		}
		query = query.Where(redeemcode.Or(preds...))
	}

	return query
}

func (s *adminServiceImpl) redeemCodeLedgerMap(ctx context.Context, codeIDs []int64) (map[int64]*dbent.AccountChangeRecord, error) {
	out := make(map[int64]*dbent.AccountChangeRecord, len(codeIDs))
	if len(codeIDs) == 0 {
		return out, nil
	}

	records, err := s.entClient.AccountChangeRecord.Query().
		Where(
			accountchangerecord.SourceTypeEQ(AccountChangeSourceRedeemCode),
			accountchangerecord.SourceIDIn(codeIDs...),
		).
		Order(dbent.Desc(accountchangerecord.FieldCreatedAt), dbent.Desc(accountchangerecord.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	for _, record := range records {
		if record.SourceID == nil {
			continue
		}
		if _, exists := out[*record.SourceID]; !exists {
			out[*record.SourceID] = record
		}
	}
	return out, nil
}

func collectRedeemCodeIDs(codes []*dbent.RedeemCode) []int64 {
	ids := make([]int64, 0, len(codes))
	for _, code := range codes {
		if code != nil {
			ids = append(ids, code.ID)
		}
	}
	return ids
}

func redeemCodeBillingItemFromEntity(code *dbent.RedeemCode, ledger *dbent.AccountChangeRecord) RedeemCodeBillingItem {
	item := RedeemCodeBillingItem{
		ID:               code.ID,
		Code:             code.Code,
		Type:             code.Type,
		Value:            code.Value,
		PaidValue:        redeemPaidValueFromEntity(code),
		Purpose:          code.Purpose,
		SalesStatus:      code.SalesStatus,
		RedeemStatus:     code.Status,
		BatchID:          code.BatchID,
		SoldAt:           code.SoldAt,
		SoldToNote:       stringFromPtr(code.SoldToNote),
		ExternalOrderNo:  stringFromPtr(code.ExternalOrderNo),
		ExternalOrderURL: stringFromPtr(code.ExternalOrderURL),
		InternalNotes:    stringFromPtr(code.InternalNotes),
		Notes:            stringFromPtr(code.Notes),
		UsedBy:           code.UsedBy,
		UsedAt:           code.UsedAt,
		CreatedAt:        code.CreatedAt,
		UpdatedAt:        code.UpdatedAt,
	}
	if code.Edges.Batch != nil {
		item.BatchName = code.Edges.Batch.Name
	}
	if code.Edges.User != nil {
		item.UsedByEmail = code.Edges.User.Email
		item.UsedByUsername = code.Edges.User.Username
		balance := code.Edges.User.Balance
		item.CurrentUserBalance = &balance
	}
	if ledger != nil {
		item.LedgerID = &ledger.ID
		item.LedgerMatched = true
		delta := ledger.Delta
		item.LedgerDelta = &delta
	}
	return item
}

func summarizeRedeemCodeBilling(codes []*dbent.RedeemCode, ledgerBySourceID map[int64]*dbent.AccountChangeRecord) RedeemCodeBillingSummary {
	var summary RedeemCodeBillingSummary
	for _, code := range codes {
		if code == nil {
			continue
		}
		summary.TotalCodes++
		switch code.Status {
		case StatusUsed:
			summary.UsedCodes++
		case StatusUnused:
			summary.UnusedCodes++
		}

		if code.Purpose == RedeemCodePurposeSaleRecharge {
			summary.SaleFaceValue += code.Value
		}
		if code.Purpose == RedeemCodePurposeSaleRecharge && code.SalesStatus == RedeemCodeSalesStatusSold {
			paidValue := redeemPaidValueFromEntity(code)
			summary.SoldFaceValue += paidValue
			if code.Status == StatusUnused {
				summary.SoldUnredeemedFaceValue += paidValue
			}
		}
		if code.Status != StatusUsed {
			continue
		}
		switch code.Purpose {
		case RedeemCodePurposeSaleRecharge:
			summary.RedeemedSaleAmount += redeemPaidValueFromEntity(code)
		case RedeemCodePurposeGift:
			summary.GiftRedeemedAmount += code.Value
		case RedeemCodePurposeCompensation:
			summary.CompensationRedeemedAmount += code.Value
		case RedeemCodePurposeInternalTest:
			summary.InternalTestRedeemedAmount += code.Value
		}
		if ledgerBySourceID[code.ID] == nil {
			summary.LedgerMissingCount++
		}
	}
	return summary
}

func redeemPaidValueFromEntity(code *dbent.RedeemCode) float64 {
	if code == nil || code.Purpose != RedeemCodePurposeSaleRecharge {
		return 0
	}
	if code.PaidValue > 0 && code.PaidValue <= code.Value {
		return code.PaidValue
	}
	return code.Value
}

func normalizeRedeemCodePurposeForService(codeType, purpose string) string {
	trimmed := strings.TrimSpace(purpose)
	switch trimmed {
	case RedeemCodePurposeSaleRecharge,
		RedeemCodePurposeGift,
		RedeemCodePurposeCompensation,
		RedeemCodePurposeInternalTest,
		RedeemCodePurposeMigration:
		return trimmed
	}
	if codeType == RedeemTypeBalance {
		return RedeemCodePurposeSaleRecharge
	}
	return RedeemCodePurposeMigration
}

func normalizeRedeemCodeSalesStatusForService(purpose, status string) string {
	trimmed := strings.TrimSpace(status)
	switch trimmed {
	case RedeemCodeSalesStatusInventory,
		RedeemCodeSalesStatusSold,
		RedeemCodeSalesStatusGifted,
		RedeemCodeSalesStatusVoid:
		return trimmed
	}
	switch strings.TrimSpace(purpose) {
	case RedeemCodePurposeGift, RedeemCodePurposeCompensation:
		return RedeemCodeSalesStatusGifted
	default:
		return RedeemCodeSalesStatusInventory
	}
}

func shouldCreateRedeemCodeBatch(input *GenerateRedeemCodesInput) bool {
	if input == nil {
		return false
	}
	return strings.TrimSpace(input.BatchName) != "" ||
		strings.TrimSpace(input.Purpose) != "" ||
		strings.TrimSpace(input.SalesChannel) != "" ||
		strings.TrimSpace(input.ExternalURL) != "" ||
		strings.TrimSpace(input.SoldToNote) != "" ||
		strings.TrimSpace(input.ExternalOrderNo) != "" ||
		strings.TrimSpace(input.ExternalOrderURL) != "" ||
		strings.TrimSpace(input.InternalNotes) != ""
}

// normalizeRedeemBatchCreatedBy drops synthetic/invalid principals that are not
// real users rows (e.g. Admin API Key service principal UserID=-1).
func normalizeRedeemBatchCreatedBy(createdBy *int64) *int64 {
	if createdBy == nil || *createdBy <= 0 {
		return nil
	}
	return createdBy
}

func defaultRedeemCodeBatchName(name, codeType string, value float64) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("%s %.2f redeem codes", strings.TrimSpace(codeType), value)
}

func defaultRedeemSalesChannel(value string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return "manual"
}

func trimStringPointerForService(v string) *string {
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringFromPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
