package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// TestBuildInvoiceRequestWorkbookSmokeFromRealExport feeds six synthetic
// pending requests through buildInvoiceRequestWorkbook and asserts on key
// cells. The resulting xlsx is written to <TempDir>/dragoncode_batch_invoice_smoke.xlsx
// so it can be opened locally for a visual sanity check.
//
// Stable output path: open with `open $TMPDIR/dragoncode_batch_invoice_smoke.xlsx`
// (macOS) — overwritten on every run.
func TestBuildInvoiceRequestWorkbookSmokeFromRealExport(t *testing.T) {
	requests := []InvoiceRequest{
		newSmokeRequest(
			"INV20260507000000025", 40000,
			"测试科技有限公司A", "91370600XXXXXXXXXXX",
			"contact_a@example.com",
			"山东省烟台市示例路100号示例大厦1号",
			"13800000001",
			"示例银行烟台支行",
			"6200000000000000001",
		),
		newSmokeRequest(
			"INV20260507000000024", 100000,
			"测试科技有限公司B", "91310113XXXXXXXXXXX",
			"contact_b@example.com",
			"", "",
			"示例银行上海支行",
			"6200000000000000002",
		),
		newSmokeRequest(
			"INV20260506000000023", 40000,
			"测试商贸有限公司C", "91371402XXXXXXXXXXX",
			"contact_c@example.com",
			"", "", "", "",
		),
		newSmokeRequest(
			"INV20260506000000022", 200000,
			"测试科技有限公司D", "91110108XXXXXXXXXXX",
			"contact_d@example.com",
			"北京市海淀区示例路200号示例大厦10层",
			"010-00000000",
			"示例银行北京支行",
			"6200000000000000003",
		),
		newSmokeRequest(
			"INV20260506000000021", 30000,
			"测试科技有限公司A", "91370600XXXXXXXXXXX",
			"contact_a@example.com",
			"山东省烟台市示例路100号示例大厦1号",
			"13800000001",
			"示例银行烟台支行",
			"6200000000000000001",
		),
		newSmokeRequest(
			"INV20260506000000020", 50000,
			"测试科技有限公司E", "91440300XXXXXXXXXXX",
			"contact_e@example.com",
			"", "", "", "",
		),
	}

	content, err := buildInvoiceRequestWorkbook(requests)
	require.NoError(t, err)
	require.NotEmpty(t, content)

	outputPath := filepath.Join(os.TempDir(), "dragoncode_batch_invoice_smoke.xlsx")
	require.NoError(t, os.WriteFile(outputPath, content, 0o644))
	t.Logf("smoke output written: %s (size=%d bytes)", outputPath, len(content))

	f, err := excelize.OpenReader(bytes.NewReader(content))
	require.NoError(t, err)
	defer func() { _ = f.Close() }()

	require.Contains(t, f.GetSheetList(), invoiceTemplateSheetBasic)
	require.Contains(t, f.GetSheetList(), invoiceTemplateSheetDetail)

	// --- Sheet 1: 发票基本信息 -------------------------------------------------
	basic := invoiceTemplateSheetBasic

	// Row 4 (first request, full snapshot)
	require.Equal(t, "INV20260507000000025", mustGetCellValue(t, f, basic, "A4"))
	require.Equal(t, "普通发票", mustGetCellValue(t, f, basic, "B4"))
	require.Equal(t, "是", mustGetCellValue(t, f, basic, "D4"))
	require.Equal(t, "否", mustGetCellValue(t, f, basic, "E4"))
	require.Equal(t, "测试科技有限公司A", mustGetCellValue(t, f, basic, "F4"))
	require.Equal(t, "91370600XXXXXXXXXXX", mustGetCellValue(t, f, basic, "G4"))
	require.Equal(t,
		"山东省烟台市示例路100号示例大厦1号",
		mustGetCellValue(t, f, basic, "K4"),
	)
	require.Equal(t, "13800000001", mustGetCellValue(t, f, basic, "Q4"))
	require.Equal(t, "示例银行烟台支行", mustGetCellValue(t, f, basic, "R4"))
	require.Equal(t, "6200000000000000001", mustGetCellValue(t, f, basic, "S4"))
	require.Equal(t, invoiceSellerBank, mustGetCellValue(t, f, basic, "AB4"))
	require.Equal(t, invoiceSellerAccount, mustGetCellValue(t, f, basic, "AC4"))
	require.Equal(t, "contact_a@example.com", mustGetCellValue(t, f, basic, "AE4"))

	// Row 5 (second request, address/phone empty)
	require.Equal(t, "INV20260507000000024", mustGetCellValue(t, f, basic, "A5"))
	require.Equal(t, "测试科技有限公司B", mustGetCellValue(t, f, basic, "F5"))
	require.Equal(t, "", mustGetCellValue(t, f, basic, "K5"))
	require.Equal(t, "", mustGetCellValue(t, f, basic, "Q5"))
	require.Equal(t, "示例银行上海支行", mustGetCellValue(t, f, basic, "R5"))
	require.Equal(t, "6200000000000000002", mustGetCellValue(t, f, basic, "S5"))

	// Row 6 (third request, no bank info either)
	require.Equal(t, "INV20260506000000023", mustGetCellValue(t, f, basic, "A6"))
	require.Equal(t, "", mustGetCellValue(t, f, basic, "R6"))
	require.Equal(t, "", mustGetCellValue(t, f, basic, "S6"))
	require.Equal(t, invoiceSellerBank, mustGetCellValue(t, f, basic, "AB6"))

	// Row 9 (last request)
	require.Equal(t, "INV20260506000000020", mustGetCellValue(t, f, basic, "A9"))
	require.Equal(t, "测试科技有限公司E", mustGetCellValue(t, f, basic, "F9"))

	// --- Sheet 2: 发票明细信息 -------------------------------------------------
	detail := invoiceTemplateSheetDetail

	expectedAmounts := []string{"400.00", "1000.00", "400.00", "2000.00", "300.00", "500.00"}
	expectedSerials := []string{
		"INV20260507000000025",
		"INV20260507000000024",
		"INV20260506000000023",
		"INV20260506000000022",
		"INV20260506000000021",
		"INV20260506000000020",
	}
	for i, amt := range expectedAmounts {
		row := invoiceTemplateDataStartRow + i
		serialCell, _ := excelize.CoordinatesToCellName(1, row)
		nameCell, _ := excelize.CoordinatesToCellName(2, row)
		taxCodeCell, _ := excelize.CoordinatesToCellName(3, row)
		unitCell, _ := excelize.CoordinatesToCellName(5, row)
		qtyCell, _ := excelize.CoordinatesToCellName(6, row)
		priceCell, _ := excelize.CoordinatesToCellName(7, row)
		amtCell, _ := excelize.CoordinatesToCellName(8, row)
		taxRateCell, _ := excelize.CoordinatesToCellName(9, row)

		require.Equal(t, expectedSerials[i], mustGetCellValue(t, f, detail, serialCell))
		require.Equal(t, "技术服务费", mustGetCellValue(t, f, detail, nameCell))
		require.Equal(t, invoiceItemTaxCode, mustGetCellValue(t, f, detail, taxCodeCell))
		require.Equal(t, "元", mustGetCellValue(t, f, detail, unitCell))
		require.Equal(t, "1", mustGetCellValue(t, f, detail, qtyCell))
		require.Equal(t, amt, mustGetCellValue(t, f, detail, priceCell))
		require.Equal(t, amt, mustGetCellValue(t, f, detail, amtCell))
		require.Equal(t, "0.01", mustGetCellValue(t, f, detail, taxRateCell))
	}

	// No leftover rows below the last data row.
	emptyCell, _ := excelize.CoordinatesToCellName(1, invoiceTemplateDataStartRow+len(requests))
	require.Equal(t, "", mustGetCellValue(t, f, basic, emptyCell))
	require.Equal(t, "", mustGetCellValue(t, f, detail, emptyCell))
}

func newSmokeRequest(
	serialNo string, totalAmountFen int64,
	title, taxNumber, email, address, phone, bankName, bankAccount string,
) InvoiceRequest {
	snapshot := InvoiceProfileSnapshot{
		Title:     title,
		TaxNumber: taxNumber,
		Email:     email,
	}
	if address != "" {
		v := address
		snapshot.Address = &v
	}
	if phone != "" {
		v := phone
		snapshot.Phone = &v
	}
	if bankName != "" {
		v := bankName
		snapshot.BankName = &v
	}
	if bankAccount != "" {
		v := bankAccount
		snapshot.BankAccount = &v
	}
	return InvoiceRequest{
		SerialNo:        serialNo,
		TotalAmountFen:  totalAmountFen,
		ProfileSnapshot: snapshot,
	}
}
