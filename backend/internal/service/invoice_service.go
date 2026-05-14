package service

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bozhouDev/DragonCode-sub2api/internal/domain"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/xuri/excelize/v2"
)

// Official tax-bureau (V260401) batch-invoice import template. The exported
// workbook is produced by filling in this template directly so the result can
// be imported into 电子税务局 - 数电发票批量导入开具 without any post-processing.
//
//go:embed templates/batch_invoice_template.xlsx
var batchInvoiceTemplate []byte

// Seller-side identity and item metadata used when filling the template.
// Adjust these constants if the issuing company / billing item changes.
const (
	invoiceTemplateSheetBasic  = "1-发票基本信息"
	invoiceTemplateSheetDetail = "2-发票明细信息"
	invoiceTemplateDataStartRow = 4

	invoiceTypeRegular = "普通发票"

	invoiceItemName    = "技术服务费"
	invoiceItemTaxCode = "3040203000000000000"
	invoiceItemUnit    = "元"
	invoiceTaxRate     = "0.01" // 1% 征收率（小规模纳税人）

	invoiceSellerBank    = "" // 通过配置或环境变量注入
	invoiceSellerAccount = "" // 通过配置或环境变量注入
)

type InvoiceService struct {
	repo InvoiceRepository
}

func NewInvoiceService(repo InvoiceRepository) *InvoiceService {
	return &InvoiceService{repo: repo}
}

func (s *InvoiceService) ListUserTopupOrders(ctx context.Context, userID int64, params pagination.PaginationParams, filters InvoiceTopupOrderListFilters) (*InvoiceTopupOrderListResult, error) {
	items, total, err := s.repo.ListUserTopupOrders(ctx, userID, params, filters)
	if err != nil {
		return nil, err
	}

	selectableAmountFen, err := s.repo.SumSelectableAmountFen(ctx, userID, filters)
	if err != nil {
		return nil, err
	}

	return &InvoiceTopupOrderListResult{
		Items:               items,
		Total:               int64(total),
		Page:                normalizePage(params.Page),
		PageSize:            params.Limit(),
		Pages:               calcPages(total, params.Limit()),
		SelectableAmountFen: selectableAmountFen,
	}, nil
}

func (s *InvoiceService) ListAdminTopupOrders(ctx context.Context, params pagination.PaginationParams, filters InvoiceTopupOrderListFilters) (*InvoiceTopupOrderListResult, error) {
	items, total, err := s.repo.ListAdminTopupOrders(ctx, params, filters)
	if err != nil {
		return nil, err
	}

	return &InvoiceTopupOrderListResult{
		Items:    items,
		Total:    int64(total),
		Page:     normalizePage(params.Page),
		PageSize: params.Limit(),
		Pages:    calcPages(total, params.Limit()),
	}, nil
}

func (s *InvoiceService) ListProfiles(ctx context.Context, userID int64) ([]InvoiceProfile, error) {
	return s.repo.ListProfiles(ctx, userID)
}

func (s *InvoiceService) CreateProfile(ctx context.Context, userID int64, input InvoiceProfileInput) (*InvoiceProfile, error) {
	if err := validateInvoiceProfileInput(input); err != nil {
		return nil, err
	}
	profileCount, err := s.repo.CountProfiles(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateProfile(ctx, userID, input, profileCount == 0)
}

func (s *InvoiceService) UpdateProfile(ctx context.Context, userID, profileID int64, input InvoiceProfileInput) (*InvoiceProfile, error) {
	// verify ownership
	if _, err := s.repo.GetProfile(ctx, userID, profileID); err != nil {
		return nil, err
	}
	if err := validateInvoiceProfileInput(input); err != nil {
		return nil, err
	}
	return s.repo.UpdateProfile(ctx, profileID, input)
}

func (s *InvoiceService) DeleteProfile(ctx context.Context, userID, profileID int64) error {
	profile, err := s.repo.GetProfile(ctx, userID, profileID)
	if err != nil {
		return err
	}

	inUse, err := s.repo.ProfileInUse(ctx, profileID)
	if err != nil {
		return err
	}
	if inUse {
		return ErrInvoiceProfileInUse
	}

	if err := s.repo.DeleteProfile(ctx, profileID); err != nil {
		return err
	}

	if profile.IsDefault {
		next, findErr := s.repo.FindLatestProfileForUser(ctx, userID)
		if findErr != nil {
			return findErr
		}
		if next != nil {
			if err := s.repo.ClearDefaultProfiles(ctx, userID); err != nil {
				return fmt.Errorf("clear previous default invoice profile: %w", err)
			}
			if _, err := s.repo.SetProfileDefault(ctx, next.ID); err != nil {
				return fmt.Errorf("set fallback default invoice profile: %w", err)
			}
		}
	}
	return nil
}

func (s *InvoiceService) SetDefaultProfile(ctx context.Context, userID, profileID int64) (*InvoiceProfile, error) {
	profile, err := s.repo.GetProfile(ctx, userID, profileID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.ClearDefaultProfiles(ctx, userID); err != nil {
		return nil, fmt.Errorf("clear existing default invoice profile: %w", err)
	}

	updated, err := s.repo.SetProfileDefault(ctx, profile.ID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func validateInvoiceProfileInput(input InvoiceProfileInput) error {
	if err := validateRequiredInvoiceProfileField("title", "抬头名称", input.Title, InvoiceProfileTitleMaxRunes); err != nil {
		return err
	}
	if err := validateRequiredInvoiceProfileField("tax_number", "纳税人识别号", input.TaxNumber, InvoiceProfileTaxNumberMaxRunes); err != nil {
		return err
	}
	if err := validateRequiredInvoiceProfileField("email", "邮箱", input.Email, InvoiceProfileEmailMaxRunes); err != nil {
		return err
	}
	if err := validateOptionalInvoiceProfileField("address", "地址", input.Address, InvoiceProfileAddressMaxRunes); err != nil {
		return err
	}
	if err := validateOptionalInvoiceProfileField("phone", "电话", input.Phone, InvoiceProfilePhoneMaxRunes); err != nil {
		return err
	}
	if err := validateOptionalInvoiceProfileField("bank_name", "开户银行", input.BankName, InvoiceProfileBankNameMaxRunes); err != nil {
		return err
	}
	return validateOptionalInvoiceProfileField("bank_account", "银行账号", input.BankAccount, InvoiceProfileBankAccountMaxRunes)
}

func validateRequiredInvoiceProfileField(field, label, value string, maxRunes int) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return infraerrors.BadRequest("INVOICE_PROFILE_FIELD_REQUIRED", fmt.Sprintf("%s不能为空", label)).
			WithMetadata(map[string]string{"field": field})
	}
	return validateInvoiceProfileFieldLength(field, label, trimmed, maxRunes)
}

func validateOptionalInvoiceProfileField(field, label string, value *string, maxRunes int) error {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return validateInvoiceProfileFieldLength(field, label, trimmed, maxRunes)
}

func validateInvoiceProfileFieldLength(field, label, value string, maxRunes int) error {
	if utf8.RuneCountInString(value) <= maxRunes {
		return nil
	}
	return infraerrors.BadRequest("INVOICE_PROFILE_FIELD_TOO_LONG", fmt.Sprintf("%s过长，最多 %d 个字符", label, maxRunes)).
		WithMetadata(map[string]string{
			"field":      field,
			"max_length": fmt.Sprintf("%d", maxRunes),
		})
}

func (s *InvoiceService) CreateRequest(ctx context.Context, userID int64, input CreateInvoiceRequestInput) (*CreateInvoiceRequestResult, error) {
	orderIDs, err := uniquePositiveIDs(input.OrderIDs)
	if err != nil {
		return nil, err
	}

	profile, err := s.repo.GetProfile(ctx, userID, input.ProfileID)
	if err != nil {
		return nil, err
	}

	orders, err := s.repo.GetTopupOrdersByIDs(ctx, orderIDs)
	if err != nil {
		return nil, err
	}
	if len(orders) != len(orderIDs) {
		return nil, ErrInvoiceOrderNotFound
	}

	var totalAmountFen int64
	for _, order := range orders {
		if order.UserID != userID {
			return nil, ErrInvoiceOrderOwnership
		}
		if order.Status != TopupStatusCompleted {
			return nil, ErrInvoiceOrderStatusInvalid
		}
		if order.InvoiceStatus != TopupInvoiceStatusNone {
			return nil, ErrInvoiceOrderAlreadyApplied
		}
		totalAmountFen += int64(order.AmountCNYFen)
	}
	if totalAmountFen < InvoiceRequestMinimumAmountFen {
		return nil, ErrInvoiceAmountBelowMinimum
	}

	updatedCount, err := s.repo.ClaimTopupOrdersForInvoicing(ctx, userID, orderIDs)
	if err != nil {
		return nil, err
	}
	if updatedCount != len(orderIDs) {
		return nil, ErrInvoiceOrderAlreadyApplied
	}

	remark := trimNullableString(input.Remark)
	requestID, _, err := s.repo.CreateRequest(ctx, CreateInvoiceRequestRepoInput{
		TempSerialNo: tempInvoiceSerialNo(),
		UserID:       userID,
		ProfileID:    profile.ID,
		ProfileSnapshotJSON: domain.InvoiceProfileSnapshot{
			Title:       profile.Title,
			TaxNumber:   profile.TaxNumber,
			Email:       profile.Email,
			Address:     profile.Address,
			Phone:       profile.Phone,
			BankName:    profile.BankName,
			BankAccount: profile.BankAccount,
		},
		TotalAmountFen: totalAmountFen,
		Status:         InvoiceRequestStatusPending,
		Remark:         remark,
	})
	if err != nil {
		return nil, err
	}

	// We need the createdAt to build the final serial; fetch it via GetRequestWithOrders
	// which also returns the entity with its timestamps.
	requestEntity, err := s.repo.GetRequestWithOrders(ctx, requestID)
	if err != nil {
		return nil, err
	}

	finalSerial := formatInvoiceSerialNo(requestEntity.ID, requestEntity.CreatedAt)
	if err := s.repo.UpdateRequestSerialNo(ctx, requestID, finalSerial); err != nil {
		return nil, err
	}

	if err := s.repo.CreateRequestOrderLinks(ctx, requestID, orderIDs); err != nil {
		return nil, err
	}

	return &CreateInvoiceRequestResult{
		InvoiceRequestID: requestID,
		SerialNo:         finalSerial,
		TotalAmountFen:   totalAmountFen,
		Status:           InvoiceRequestStatusPending,
	}, nil
}

func (s *InvoiceService) ListUserRequests(ctx context.Context, userID int64, params pagination.PaginationParams, filters InvoiceRequestListFilters) (*InvoiceRequestListResult, error) {
	items, total, err := s.repo.ListUserRequests(ctx, userID, params, filters)
	if err != nil {
		return nil, err
	}

	return &InvoiceRequestListResult{
		Items:    items,
		Total:    int64(total),
		Page:     normalizePage(params.Page),
		PageSize: params.Limit(),
		Pages:    calcPages(total, params.Limit()),
	}, nil
}

func (s *InvoiceService) ListAdminRequests(ctx context.Context, params pagination.PaginationParams, filters InvoiceRequestListFilters) (*InvoiceRequestListResult, error) {
	items, total, err := s.repo.ListAdminRequests(ctx, params, filters)
	if err != nil {
		return nil, err
	}

	pendingCount, err := s.repo.CountPendingRequests(ctx)
	if err != nil {
		return nil, err
	}

	return &InvoiceRequestListResult{
		Items:        items,
		Total:        int64(total),
		Page:         normalizePage(params.Page),
		PageSize:     params.Limit(),
		Pages:        calcPages(total, params.Limit()),
		PendingCount: int64(pendingCount),
	}, nil
}

func (s *InvoiceService) CompleteRequest(ctx context.Context, actorID, requestID int64) error {
	return s.transitionRequestStatus(ctx, actorID, requestID, InvoiceRequestStatusCompleted, nil)
}

func (s *InvoiceService) RejectRequest(ctx context.Context, actorID, requestID int64, input RejectInvoiceRequestInput) error {
	reason := strings.TrimSpace(input.RejectReason)
	if reason == "" {
		return infraerrors.BadRequest("INVOICE_REJECT_REASON_REQUIRED", "reject_reason is required")
	}
	return s.transitionRequestStatus(ctx, actorID, requestID, InvoiceRequestStatusRejected, &reason)
}

func (s *InvoiceService) ExportRequests(ctx context.Context, actorID int64, input ExportInvoiceRequestsInput) (*ExportInvoiceRequestsResult, error) {
	filters := InvoiceRequestListFilters{
		Status:          strings.TrimSpace(input.Status),
		Search:          strings.TrimSpace(input.Search),
		StartTime:       input.StartTime,
		EndTime:         input.EndTime,
		IncludeExported: input.IncludeExported,
	}

	requests, err := s.repo.ListRequestsForExport(ctx, filters)
	if err != nil {
		return nil, err
	}
	if len(requests) == 0 {
		return nil, ErrInvoiceExportEmpty
	}

	now := time.Now()
	batchNo := fmt.Sprintf("INVBATCH%s", now.Format("20060102150405"))
	fileName := fmt.Sprintf("batch_invoice_%s.xlsx", now.Format("20060102_150405"))

	content, err := buildInvoiceRequestWorkbook(requests)
	if err != nil {
		return nil, err
	}

	pendingIDs := make([]int64, 0, len(requests))
	for _, req := range requests {
		if req.Status == InvoiceRequestStatusPending {
			pendingIDs = append(pendingIDs, req.ID)
		}
	}
	if len(pendingIDs) > 0 {
		updated, err := s.repo.MarkRequestsExported(ctx, pendingIDs, batchNo, actorID)
		if err != nil {
			return nil, err
		}
		if updated != len(pendingIDs) {
			return nil, ErrInvoiceRequestStatusInvalid
		}
	}

	return &ExportInvoiceRequestsResult{
		FileName: fileName,
		Content:  content,
		BatchNo:  batchNo,
		Exported: len(requests),
	}, nil
}

func (s *InvoiceService) transitionRequestStatus(ctx context.Context, actorID, requestID int64, targetStatus string, rejectReason *string) error {
	requestEntity, err := s.repo.GetRequestWithOrders(ctx, requestID)
	if err != nil {
		return err
	}

	if requestEntity.Status != InvoiceRequestStatusPending && requestEntity.Status != InvoiceRequestStatusExported {
		return ErrInvoiceRequestStatusInvalid
	}

	switch targetStatus {
	case InvoiceRequestStatusCompleted, InvoiceRequestStatusRejected:
	default:
		return ErrInvoiceRequestStatusInvalid
	}

	if err := s.repo.UpdateRequestStatus(ctx, requestID, UpdateRequestStatusInput{
		Status:       targetStatus,
		ActorID:      actorID,
		RejectReason: rejectReason,
	}); err != nil {
		return err
	}

	orderIDs := make([]int64, 0, len(requestEntity.Orders))
	for _, o := range requestEntity.Orders {
		orderIDs = append(orderIDs, o.TopupOrderID)
	}

	if len(orderIDs) > 0 {
		switch targetStatus {
		case InvoiceRequestStatusCompleted:
			updated, err := s.repo.MarkTopupOrdersInvoiced(ctx, orderIDs)
			if err != nil {
				return err
			}
			if updated != len(orderIDs) {
				return ErrInvoiceRequestStatusInvalid
			}
		case InvoiceRequestStatusRejected:
			if err := s.repo.DeleteRequestOrderLinks(ctx, requestID); err != nil {
				return err
			}
			updated, err := s.repo.ReleaseTopupOrdersFromInvoicing(ctx, orderIDs)
			if err != nil {
				return err
			}
			if updated != len(orderIDs) {
				return ErrInvoiceRequestStatusInvalid
			}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Excel workbook builder (business logic – stays in service layer)
// ---------------------------------------------------------------------------

// buildInvoiceRequestWorkbook fills the official 电子税务局 batch-invoice import
// template with the given invoice requests and returns the resulting xlsx
// bytes. The output can be imported directly into 电子税务局 → 蓝字数电发票开具
// → 批量导入开具 without any further conversion.
func buildInvoiceRequestWorkbook(requests []InvoiceRequest) ([]byte, error) {
	f, err := excelize.OpenReader(bytes.NewReader(batchInvoiceTemplate))
	if err != nil {
		return nil, fmt.Errorf("open batch-invoice template: %w", err)
	}
	defer func() { _ = f.Close() }()

	if err := fillInvoiceBasicInfoSheet(f, requests); err != nil {
		return nil, err
	}
	if err := fillInvoiceDetailSheet(f, requests); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("write invoice export workbook: %w", err)
	}
	return buf.Bytes(), nil
}

// fillInvoiceBasicInfoSheet populates sheet "1-发票基本信息". Column indexes
// follow the V260401 template; the tax bureau requires every cell value to be
// stored as text (hence SetCellStr).
func fillInvoiceBasicInfoSheet(f *excelize.File, requests []InvoiceRequest) error {
	sheet := invoiceTemplateSheetBasic
	for i, req := range requests {
		row := invoiceTemplateDataStartRow + i
		snapshot := req.ProfileSnapshot

		cells := map[int]string{
			1:  req.SerialNo,           // 发票流水号
			2:  invoiceTypeRegular,     // 发票类型
			4:  "是",                    // 是否含税
			5:  "否",                    // 受票方自然人标识
			6:  snapshot.Title,         // 购买方名称
			7:  snapshot.TaxNumber,     // 购买方纳税人识别号
			11: valueOrEmpty(snapshot.Address),     // 购买方地址
			17: valueOrEmpty(snapshot.Phone),       // 购买方电话
			18: valueOrEmpty(snapshot.BankName),    // 购买方开户银行
			19: valueOrEmpty(snapshot.BankAccount), // 购买方银行账号
			28: invoiceSellerBank,     // 销售方开户行
			29: invoiceSellerAccount,  // 销售方银行账号
			31: snapshot.Email,        // 购买方邮箱
		}

		for col, value := range cells {
			if value == "" {
				continue
			}
			if err := setSheetCellStr(f, sheet, col, row, value); err != nil {
				return err
			}
		}
	}
	return nil
}

// fillInvoiceDetailSheet populates sheet "2-发票明细信息". Each invoice request
// becomes a single line item ("技术服务费") whose unit-price equals the total
// (since 数量 = 1). 含税标志=是, so 单价/金额 are gross (含税) amounts.
func fillInvoiceDetailSheet(f *excelize.File, requests []InvoiceRequest) error {
	sheet := invoiceTemplateSheetDetail
	for i, req := range requests {
		row := invoiceTemplateDataStartRow + i
		amountStr := fmt.Sprintf("%.2f", fenToYuan(req.TotalAmountFen))

		cells := map[int]string{
			1: req.SerialNo,        // 发票流水号
			2: invoiceItemName,     // 项目名称
			3: invoiceItemTaxCode,  // 商品和服务税收编码
			5: invoiceItemUnit,     // 单位
			6: "1",                 // 数量
			7: amountStr,           // 单价
			8: amountStr,           // 金额
			9: invoiceTaxRate,      // 税率
		}

		for col, value := range cells {
			if err := setSheetCellStr(f, sheet, col, row, value); err != nil {
				return err
			}
		}
	}
	return nil
}

func setSheetCellStr(f *excelize.File, sheet string, col, row int, value string) error {
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return fmt.Errorf("compute cell coordinates (%s col=%d row=%d): %w", sheet, col, row, err)
	}
	if err := f.SetCellStr(sheet, cell, value); err != nil {
		return fmt.Errorf("set %s!%s: %w", sheet, cell, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Pure utility helpers (no DB access)
// ---------------------------------------------------------------------------

func uniquePositiveIDs(ids []int64) ([]int64, error) {
	if len(ids) == 0 {
		return nil, ErrInvoiceEmptyOrderSelection
	}
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, ErrInvoiceOrderNotFound
		}
		if _, ok := seen[id]; ok {
			return nil, infraerrors.BadRequest("INVOICE_DUPLICATE_ORDER_ID", "duplicate topup order id in request")
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func formatInvoiceSerialNo(id int64, createdAt time.Time) string {
	return fmt.Sprintf("INV%s%09d", createdAt.Format("20060102"), id)
}

func tempInvoiceSerialNo() string {
	return fmt.Sprintf("TMP%017d", time.Now().UnixNano()%100000000000000000)
}

func calcPages(total, pageSize int) int {
	if pageSize <= 0 {
		pageSize = 20
	}
	if total <= 0 {
		return 1
	}
	pages := total / pageSize
	if total%pageSize != 0 {
		pages++
	}
	if pages < 1 {
		return 1
	}
	return pages
}

func normalizePage(page int) int {
	if page <= 0 {
		return 1
	}
	return page
}

func trimNullableString(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func valueOrEmpty(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func fenToYuan(amountFen int64) float64 {
	return float64(amountFen) / 100
}
