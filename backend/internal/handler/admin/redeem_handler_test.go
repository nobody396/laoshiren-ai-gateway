package admin

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCreateAndRedeemHandler creates a RedeemHandler with a non-nil (but minimal)
// RedeemService so that CreateAndRedeem's nil guard passes and we can test the
// parameter-validation layer that runs before any service call.
func newCreateAndRedeemHandler() *RedeemHandler {
	return &RedeemHandler{
		adminService:  newStubAdminService(),
		redeemService: &service.RedeemService{}, // non-nil to pass nil guard
	}
}

// postCreateAndRedeemValidation calls CreateAndRedeem and returns the response
// status code. For cases that pass validation and proceed into the service layer,
// a panic may occur (because RedeemService internals are nil); this is expected
// and treated as "validation passed" (returns 0 to indicate panic).
func postCreateAndRedeemValidation(t *testing.T, handler *RedeemHandler, body any) (code int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	jsonBytes, err := json.Marshal(body)
	require.NoError(t, err)
	c.Request, _ = http.NewRequest(http.MethodPost, "/api/v1/admin/redeem-codes/create-and-redeem", bytes.NewReader(jsonBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	defer func() {
		if r := recover(); r != nil {
			// Panic means we passed validation and entered service layer (expected for minimal stub).
			code = 0
		}
	}()
	handler.CreateAndRedeem(c)
	return w.Code
}

func TestCreateAndRedeem_TypeDefaultsToBalance(t *testing.T) {
	// 不传 type 字段时应默认 balance，不触发 subscription 校验。
	// 验证通过后进入 service 层会 panic（返回 0），说明默认值生效。
	h := newCreateAndRedeemHandler()
	code := postCreateAndRedeemValidation(t, h, map[string]any{
		"code":    "test-balance-default",
		"value":   10.0,
		"user_id": 1,
	})

	assert.NotEqual(t, http.StatusBadRequest, code,
		"omitting type should default to balance and pass validation")
}

func TestCreateAndRedeem_SubscriptionRequiresGroupID(t *testing.T) {
	h := newCreateAndRedeemHandler()
	code := postCreateAndRedeemValidation(t, h, map[string]any{
		"code":          "test-sub-no-group",
		"type":          "subscription",
		"value":         29.9,
		"user_id":       1,
		"validity_days": 30,
		// group_id 缺失
	})

	assert.Equal(t, http.StatusBadRequest, code)
}

func TestCreateAndRedeem_SubscriptionRequiresPositiveValidityDays(t *testing.T) {
	groupID := int64(5)
	h := newCreateAndRedeemHandler()

	cases := []struct {
		name         string
		validityDays int
	}{
		{"zero", 0},
		{"negative", -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := postCreateAndRedeemValidation(t, h, map[string]any{
				"code":          "test-sub-bad-days-" + tc.name,
				"type":          "subscription",
				"value":         29.9,
				"user_id":       1,
				"group_id":      groupID,
				"validity_days": tc.validityDays,
			})

			assert.Equal(t, http.StatusBadRequest, code)
		})
	}
}

func TestCreateAndRedeem_SubscriptionValidParamsPassValidation(t *testing.T) {
	groupID := int64(5)
	h := newCreateAndRedeemHandler()
	code := postCreateAndRedeemValidation(t, h, map[string]any{
		"code":          "test-sub-valid",
		"type":          "subscription",
		"value":         29.9,
		"user_id":       1,
		"group_id":      groupID,
		"validity_days": 31,
	})

	assert.NotEqual(t, http.StatusBadRequest, code,
		"valid subscription params should pass validation")
}

func TestCreateAndRedeem_BalanceIgnoresSubscriptionFields(t *testing.T) {
	h := newCreateAndRedeemHandler()
	// balance 类型不传 group_id 和 validity_days，不应报 400
	code := postCreateAndRedeemValidation(t, h, map[string]any{
		"code":    "test-balance-no-extras",
		"type":    "balance",
		"value":   50.0,
		"user_id": 1,
	})

	assert.NotEqual(t, http.StatusBadRequest, code,
		"balance type should not require group_id or validity_days")
}

func TestRedeemGenerate_AcceptsBillingMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	body := map[string]any{
		"count":              2,
		"type":               "balance",
		"value":              20,
		"batch_name":         "2026-05 sold cards",
		"purpose":            "sale_recharge",
		"sales_status":       "sold",
		"sales_channel":      "manual",
		"external_url":       "https://example.com/shop",
		"sold_to_note":       "buyer@example.com",
		"external_order_no":  "ORDER-20",
		"external_order_url": "https://example.com/orders/ORDER-20",
		"internal_notes":     "manual reconciliation",
	}
	jsonBytes, err := json.Marshal(body)
	require.NoError(t, err)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/redeem-codes/generate", bytes.NewReader(jsonBytes))
	c.Request.Header.Set("Content-Type", "application/json")

	svc := newStubAdminService()
	NewRedeemHandler(svc, nil).Generate(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.generatedRedeemInput)
	require.Equal(t, 2, svc.generatedRedeemInput.Count)
	require.Equal(t, service.RedeemCodePurposeSaleRecharge, svc.generatedRedeemInput.Purpose)
	require.Equal(t, service.RedeemCodeSalesStatusSold, svc.generatedRedeemInput.SalesStatus)
	require.Equal(t, "ORDER-20", svc.generatedRedeemInput.ExternalOrderNo)
	require.Equal(t, "buyer@example.com", svc.generatedRedeemInput.SoldToNote)
}

func TestRedeemBilling_ParsesFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/redeem-codes/billing?page=2&page_size=50&search=ORDER-20&purpose=sale_recharge&sales_status=sold&redeem_status=used&amount_min=10&amount_max=30&used_start_time=2026-05-01&used_end_time=2026-05-31",
		nil,
	)

	svc := newStubAdminService()
	NewRedeemHandler(svc, nil).ListBilling(c)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 2, svc.lastBillingPage)
	require.Equal(t, 50, svc.lastBillingPageSize)
	require.Equal(t, "ORDER-20", svc.lastBillingFilters.Search)
	require.Equal(t, service.RedeemCodePurposeSaleRecharge, svc.lastBillingFilters.Purpose)
	require.Equal(t, service.RedeemCodeSalesStatusSold, svc.lastBillingFilters.SalesStatus)
	require.Equal(t, service.StatusUsed, svc.lastBillingFilters.RedeemStatus)
	require.NotNil(t, svc.lastBillingFilters.AmountMin)
	require.NotNil(t, svc.lastBillingFilters.AmountMax)
	assert.InDelta(t, 10, *svc.lastBillingFilters.AmountMin, 0.001)
	assert.InDelta(t, 30, *svc.lastBillingFilters.AmountMax, 0.001)
	require.NotNil(t, svc.lastBillingFilters.UsedStartTime)
	require.NotNil(t, svc.lastBillingFilters.UsedEndTime)
}

func TestRedeemExport_IncludesRedeemURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/redeem-codes/export?include_redeem_url=true&redeem_url_base=https://app.example.com/",
		nil,
	)

	NewRedeemHandler(newStubAdminService(), nil).Export(c)

	require.Equal(t, http.StatusOK, w.Code)
	records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "redeem_url", records[0][len(records[0])-1])
	require.Equal(t, "https://app.example.com/redeem?code=R-TEST", records[1][len(records[1])-1])
}

func TestRedeemExport_RejectsInvalidRedeemURLBase(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(
		http.MethodGet,
		"/api/v1/admin/redeem-codes/export?include_redeem_url=true&redeem_url_base=javascript:alert(1)",
		nil,
	)

	NewRedeemHandler(newStubAdminService(), nil).Export(c)

	require.Equal(t, http.StatusBadRequest, w.Code)
}
