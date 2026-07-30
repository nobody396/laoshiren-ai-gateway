package repository

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

func TestExpandCommissionTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{
			name:   "canonical consumption expands with legacy alias",
			input:  service.CommissionTypeConsumption,
			expect: []string{service.CommissionTypeConsumption, "consumption_commission"},
		},
		{
			name:   "legacy consumption expands with canonical alias",
			input:  "consumption_commission",
			expect: []string{service.CommissionTypeConsumption, "consumption_commission"},
		},
		{
			name:   "self consumption remains independently filterable",
			input:  "self_consumption_commission",
			expect: []string{"self_consumption_commission"},
		},
		{
			name:   "canonical referral expands with legacy alias",
			input:  service.CommissionTypeFirstRechargeReferral,
			expect: []string{service.CommissionTypeFirstRechargeReferral, "first_recharge_referral_bonus"},
		},
		{
			name:   "unknown type remains unchanged",
			input:  "custom_type",
			expect: []string{"custom_type"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := expandCommissionTypes(tc.input)
			if len(got) != len(tc.expect) {
				t.Fatalf("unexpected alias count: got %d want %d", len(got), len(tc.expect))
			}
			for i := range tc.expect {
				if got[i] != tc.expect[i] {
					t.Fatalf("unexpected alias at index %d: got %q want %q", i, got[i], tc.expect[i])
				}
			}
		})
	}
}
