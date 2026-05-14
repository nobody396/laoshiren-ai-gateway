package service

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"strings"
	"testing"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/enttest"
	"github.com/bozhouDev/DragonCode-sub2api/ent/invoicerequest"
	"github.com/bozhouDev/DragonCode-sub2api/ent/invoicerequestorder"
	"github.com/bozhouDev/DragonCode-sub2api/ent/topuporder"
	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

// newInvoiceRepoForTest creates an InvoiceRepository backed by an in-memory
// SQLite database. It returns both the repository and the raw ent client so
// tests can set up fixtures and verify state directly.
func newInvoiceRepoForTest(t *testing.T) (InvoiceRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", "file:invoice_service?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	// Import the repository implementation via an adapter so the service
	// package does not depend on the repository package directly.
	repo := newInMemoryInvoiceRepo(client)
	return repo, client
}

func newInvoiceServiceForTest(t *testing.T) (*InvoiceService, *dbent.Client) {
	t.Helper()
	repo, client := newInvoiceRepoForTest(t)
	return NewInvoiceService(repo), client
}

func TestInvoiceServiceCreateProfileAcceptsChineseAddressByRuneLength(t *testing.T) {
	svc, client := newInvoiceServiceForTest(t)
	ctx := context.Background()

	testUser := client.User.Create().
		SetEmail("invoice-profile-address@example.com").
		SetPasswordHash("hash").
		SaveX(ctx)
	address := "深圳市示例区示例街道示例路1号示例楼101"

	profile, err := svc.CreateProfile(ctx, testUser.ID, InvoiceProfileInput{
		Title:     "深圳市测试科技有限公司",
		TaxNumber: "TAX001",
		Email:     "billing@example.com",
		Address:   &address,
	})

	require.NoError(t, err)
	require.NotNil(t, profile)
	require.NotNil(t, profile.Address)
	require.Equal(t, address, *profile.Address)
	require.True(t, profile.IsDefault)
}

func TestInvoiceServiceCreateProfileRejectsOverlongAddressWithBadRequest(t *testing.T) {
	svc, client := newInvoiceServiceForTest(t)
	ctx := context.Background()

	testUser := client.User.Create().
		SetEmail("invoice-profile-long-address@example.com").
		SetPasswordHash("hash").
		SaveX(ctx)
	address := strings.Repeat("深", InvoiceProfileAddressMaxRunes+1)

	profile, err := svc.CreateProfile(ctx, testUser.ID, InvoiceProfileInput{
		Title:     "深圳市测试科技有限公司",
		TaxNumber: "TAX001",
		Email:     "billing@example.com",
		Address:   &address,
	})

	require.Nil(t, profile)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	appErr := infraerrors.FromError(err)
	require.Equal(t, "INVOICE_PROFILE_FIELD_TOO_LONG", appErr.Reason)
	require.Equal(t, "address", appErr.Metadata["field"])
	require.Equal(t, "500", appErr.Metadata["max_length"])
}

func TestInvoiceServiceCreateRequest(t *testing.T) {
	svc, client := newInvoiceServiceForTest(t)
	ctx := context.Background()

	testUser := client.User.Create().
		SetEmail("invoice-user@example.com").
		SetPasswordHash("hash").
		SaveX(ctx)
	profile := client.InvoiceProfile.Create().
		SetUserID(testUser.ID).
		SetTitle("测试公司").
		SetTaxNumber("TAX001").
		SetEmail("billing@example.com").
		SetIsDefault(true).
		SaveX(ctx)
	order1 := client.TopupOrder.Create().
		SetOrderNo("TP00001").
		SetUserID(testUser.ID).
		SetAmountCnyFen(20000).
		SetPayType("alipay").
		SetStatus(TopupStatusCompleted).
		SaveX(ctx)
	order2 := client.TopupOrder.Create().
		SetOrderNo("TP00002").
		SetUserID(testUser.ID).
		SetAmountCnyFen(15000).
		SetPayType("wechat").
		SetStatus(TopupStatusCompleted).
		SaveX(ctx)

	result, err := svc.CreateRequest(ctx, testUser.ID, CreateInvoiceRequestInput{
		ProfileID: profile.ID,
		OrderIDs:  []int64{order1.ID, order2.ID},
	})
	require.NoError(t, err)
	require.Equal(t, int64(35000), result.TotalAmountFen)
	require.Equal(t, InvoiceRequestStatusPending, result.Status)
	require.NotZero(t, result.InvoiceRequestID)
	require.Contains(t, result.SerialNo, "INV")

	requestEntity := client.InvoiceRequest.Query().
		Where(invoicerequest.IDEQ(result.InvoiceRequestID)).
		OnlyX(ctx)
	require.Equal(t, InvoiceRequestStatusPending, requestEntity.Status)
	require.Equal(t, "测试公司", requestEntity.ProfileSnapshotJSON.Title)

	requestOrders := client.InvoiceRequestOrder.Query().
		Where(invoicerequestorder.InvoiceRequestIDEQ(result.InvoiceRequestID)).
		AllX(ctx)
	require.Len(t, requestOrders, 2)

	updatedOrders := client.TopupOrder.Query().
		Where(topuporder.IDIn(order1.ID, order2.ID)).
		Order(dbent.Asc(topuporder.FieldID)).
		AllX(ctx)
	require.Equal(t, TopupInvoiceStatusApplied, updatedOrders[0].InvoiceStatus)
	require.Equal(t, TopupInvoiceStatusApplied, updatedOrders[1].InvoiceStatus)
}

func TestInvoiceServiceCreateRequestRequiresMinimumAmount(t *testing.T) {
	svc, client := newInvoiceServiceForTest(t)
	ctx := context.Background()

	testUser := client.User.Create().
		SetEmail("invoice-minimum@example.com").
		SetPasswordHash("hash").
		SaveX(ctx)
	profile := client.InvoiceProfile.Create().
		SetUserID(testUser.ID).
		SetTitle("门槛公司").
		SetTaxNumber("TAX-MIN").
		SetEmail("minimum@example.com").
		SetIsDefault(true).
		SaveX(ctx)
	order1 := client.TopupOrder.Create().
		SetOrderNo("TPMIN01").
		SetUserID(testUser.ID).
		SetAmountCnyFen(20000).
		SetPayType("alipay").
		SetStatus(TopupStatusCompleted).
		SaveX(ctx)
	order2 := client.TopupOrder.Create().
		SetOrderNo("TPMIN02").
		SetUserID(testUser.ID).
		SetAmountCnyFen(9999).
		SetPayType("wechat").
		SetStatus(TopupStatusCompleted).
		SaveX(ctx)

	result, err := svc.CreateRequest(ctx, testUser.ID, CreateInvoiceRequestInput{
		ProfileID: profile.ID,
		OrderIDs:  []int64{order1.ID, order2.ID},
	})
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrInvoiceAmountBelowMinimum)
	require.Equal(t, 0, client.InvoiceRequest.Query().CountX(ctx))

	updatedOrders := client.TopupOrder.Query().
		Where(topuporder.IDIn(order1.ID, order2.ID)).
		Order(dbent.Asc(topuporder.FieldID)).
		AllX(ctx)
	require.Equal(t, TopupInvoiceStatusNone, updatedOrders[0].InvoiceStatus)
	require.Equal(t, TopupInvoiceStatusNone, updatedOrders[1].InvoiceStatus)
}

func TestInvoiceServiceRejectAndCompleteRequest(t *testing.T) {
	svc, client := newInvoiceServiceForTest(t)
	ctx := context.Background()

	testUser := client.User.Create().
		SetEmail("invoice-flow@example.com").
		SetPasswordHash("hash").
		SaveX(ctx)
	profile := client.InvoiceProfile.Create().
		SetUserID(testUser.ID).
		SetTitle("流程公司").
		SetTaxNumber("TAX002").
		SetEmail("flow@example.com").
		SetIsDefault(true).
		SaveX(ctx)
	order := client.TopupOrder.Create().
		SetOrderNo("TPFLOW1").
		SetUserID(testUser.ID).
		SetAmountCnyFen(30000).
		SetPayType("alipay").
		SetStatus(TopupStatusCompleted).
		SaveX(ctx)

	created, err := svc.CreateRequest(ctx, testUser.ID, CreateInvoiceRequestInput{
		ProfileID: profile.ID,
		OrderIDs:  []int64{order.ID},
	})
	require.NoError(t, err)

	require.NoError(t, svc.RejectRequest(ctx, 99, created.InvoiceRequestID, RejectInvoiceRequestInput{RejectReason: "信息有误"}))

	rejected := client.InvoiceRequest.GetX(ctx, created.InvoiceRequestID)
	require.Equal(t, InvoiceRequestStatusRejected, rejected.Status)
	require.NotNil(t, rejected.RejectReason)
	require.Equal(t, "信息有误", *rejected.RejectReason)
	require.False(t, client.InvoiceRequestOrder.Query().Where(invoicerequestorder.InvoiceRequestIDEQ(created.InvoiceRequestID)).ExistX(ctx))
	require.Equal(t, TopupInvoiceStatusNone, client.TopupOrder.GetX(ctx, order.ID).InvoiceStatus)

	created2, err := svc.CreateRequest(ctx, testUser.ID, CreateInvoiceRequestInput{
		ProfileID: profile.ID,
		OrderIDs:  []int64{order.ID},
	})
	require.NoError(t, err)

	require.NoError(t, svc.CompleteRequest(ctx, 100, created2.InvoiceRequestID))

	completed := client.InvoiceRequest.GetX(ctx, created2.InvoiceRequestID)
	require.Equal(t, InvoiceRequestStatusCompleted, completed.Status)
	require.NotNil(t, completed.CompletedBy)
	require.Equal(t, int64(100), *completed.CompletedBy)
	require.Equal(t, TopupInvoiceStatusInvoiced, client.TopupOrder.GetX(ctx, order.ID).InvoiceStatus)
}

func TestInvoiceServiceExportRequestsIncludeProcessed(t *testing.T) {
	svc, client := newInvoiceServiceForTest(t)
	ctx := context.Background()

	testUser := client.User.Create().
		SetEmail("invoice-export@example.com").
		SetPasswordHash("hash").
		SaveX(ctx)
	profile := client.InvoiceProfile.Create().
		SetUserID(testUser.ID).
		SetTitle("导出公司").
		SetTaxNumber("TAX003").
		SetEmail("export@example.com").
		SetIsDefault(true).
		SaveX(ctx)
	order := client.TopupOrder.Create().
		SetOrderNo("TPEXPORT1").
		SetUserID(testUser.ID).
		SetAmountCnyFen(30000).
		SetPayType("alipay").
		SetStatus(TopupStatusCompleted).
		SaveX(ctx)

	created, err := svc.CreateRequest(ctx, testUser.ID, CreateInvoiceRequestInput{
		ProfileID: profile.ID,
		OrderIDs:  []int64{order.ID},
	})
	require.NoError(t, err)
	require.NoError(t, svc.CompleteRequest(ctx, 101, created.InvoiceRequestID))

	result, err := svc.ExportRequests(ctx, 101, ExportInvoiceRequestsInput{
		IncludeExported: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, result.Content)
	require.Equal(t, 1, result.Exported)
}

func TestBuildInvoiceRequestWorkbookFillsTaxBureauTemplate(t *testing.T) {
	content, err := buildInvoiceRequestWorkbook([]InvoiceRequest{
		{
			SerialNo:       "INV202604170000000001",
			TotalAmountFen: 40000,
			ProfileSnapshot: InvoiceProfileSnapshot{
				Title:       "测试公司A",
				TaxNumber:   "TAX-A",
				Email:       "a@example.com",
				Address:     stringPtr("地址A"),
				Phone:       stringPtr("13800000000"),
				BankName:    stringPtr("银行A"),
				BankAccount: stringPtr("账号A"),
			},
			Orders: []InvoiceRequestOrder{
				{TopupOrder: InvoiceTopupOrder{OrderNo: "ORDER-A1"}},
				{TopupOrder: InvoiceTopupOrder{OrderNo: "ORDER-A2"}},
			},
		},
		{
			SerialNo:       "INV202604170000000002",
			TotalAmountFen: 100000,
			ProfileSnapshot: InvoiceProfileSnapshot{
				Title:     "测试公司B",
				TaxNumber: "TAX-B",
				Email:     "b@example.com",
			},
			Orders: []InvoiceRequestOrder{
				{TopupOrder: InvoiceTopupOrder{OrderNo: "ORDER-B1"}},
			},
		},
	})
	require.NoError(t, err)

	f, err := excelize.OpenReader(bytes.NewReader(content))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	require.Contains(t, f.GetSheetList(), "1-发票基本信息")
	require.Contains(t, f.GetSheetList(), "2-发票明细信息")

	basic := "1-发票基本信息"
	require.Equal(t, "发票流水号", mustGetCellValue(t, f, basic, "A3"))
	require.Equal(t, "购买方名称", mustGetCellValue(t, f, basic, "F3"))

	require.Equal(t, "INV202604170000000001", mustGetCellValue(t, f, basic, "A4"))
	require.Equal(t, "普通发票", mustGetCellValue(t, f, basic, "B4"))
	require.Equal(t, "是", mustGetCellValue(t, f, basic, "D4"))
	require.Equal(t, "否", mustGetCellValue(t, f, basic, "E4"))
	require.Equal(t, "测试公司A", mustGetCellValue(t, f, basic, "F4"))
	require.Equal(t, "TAX-A", mustGetCellValue(t, f, basic, "G4"))
	require.Equal(t, "地址A", mustGetCellValue(t, f, basic, "K4"))
	require.Equal(t, "13800000000", mustGetCellValue(t, f, basic, "Q4"))
	require.Equal(t, "银行A", mustGetCellValue(t, f, basic, "R4"))
	require.Equal(t, "账号A", mustGetCellValue(t, f, basic, "S4"))
	require.Equal(t, invoiceSellerBank, mustGetCellValue(t, f, basic, "AB4"))
	require.Equal(t, invoiceSellerAccount, mustGetCellValue(t, f, basic, "AC4"))
	require.Equal(t, "a@example.com", mustGetCellValue(t, f, basic, "AE4"))

	// Optional snapshot fields should remain blank when absent.
	require.Equal(t, "INV202604170000000002", mustGetCellValue(t, f, basic, "A5"))
	require.Equal(t, "测试公司B", mustGetCellValue(t, f, basic, "F5"))
	require.Equal(t, "", mustGetCellValue(t, f, basic, "K5"))
	require.Equal(t, "", mustGetCellValue(t, f, basic, "R5"))

	detail := "2-发票明细信息"
	require.Equal(t, "INV202604170000000001", mustGetCellValue(t, f, detail, "A4"))
	require.Equal(t, "技术服务费", mustGetCellValue(t, f, detail, "B4"))
	require.Equal(t, invoiceItemTaxCode, mustGetCellValue(t, f, detail, "C4"))
	require.Equal(t, "元", mustGetCellValue(t, f, detail, "E4"))
	require.Equal(t, "1", mustGetCellValue(t, f, detail, "F4"))
	require.Equal(t, "400.00", mustGetCellValue(t, f, detail, "G4"))
	require.Equal(t, "400.00", mustGetCellValue(t, f, detail, "H4"))
	require.Equal(t, "0.01", mustGetCellValue(t, f, detail, "I4"))

	require.Equal(t, "INV202604170000000002", mustGetCellValue(t, f, detail, "A5"))
	require.Equal(t, "1000.00", mustGetCellValue(t, f, detail, "G5"))
	require.Equal(t, "1000.00", mustGetCellValue(t, f, detail, "H5"))
}

func stringPtr(v string) *string {
	return &v
}

func mustGetCellValue(t *testing.T, f *excelize.File, sheet, cell string) string {
	t.Helper()
	value, err := f.GetCellValue(sheet, cell)
	require.NoError(t, err)
	return value
}
