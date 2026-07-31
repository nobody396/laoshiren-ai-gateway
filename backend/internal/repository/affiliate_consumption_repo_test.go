package repository

import (
	"testing"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
)

func TestValidateAffiliateSourcePolicySeparatesPartnerAndSelfAttribution(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		userID    int64
		policy    string
		partnerID int64
		customer  int32
		partner   int32
		wantError bool
	}{
		{
			name:      "regular partner",
			userID:    11,
			policy:    service.AffiliateSourcePolicyPartnerUsage,
			partnerID: 22,
			customer:  300,
			partner:   700,
		},
		{
			name:      "regular partner cannot be self",
			userID:    11,
			policy:    service.AffiliateSourcePolicyPartnerUsage,
			partnerID: 11,
			customer:  300,
			partner:   700,
			wantError: true,
		},
		{
			name:      "self fixed cash pool",
			userID:    11,
			policy:    service.AffiliateSourcePolicyPartnerSelfUsage,
			partnerID: 11,
			customer:  0,
			partner:   service.AffiliateAgentPoolRateBPS,
		},
		{
			name:      "self cannot point elsewhere",
			userID:    11,
			policy:    service.AffiliateSourcePolicyPartnerSelfUsage,
			partnerID: 22,
			customer:  0,
			partner:   service.AffiliateAgentPoolRateBPS,
			wantError: true,
		},
		{
			name:      "self cannot split customer rebate",
			userID:    11,
			policy:    service.AffiliateSourcePolicyPartnerSelfUsage,
			partnerID: 11,
			customer:  500,
			partner:   500,
			wantError: true,
		},
		{
			name:      "self rate is not configurable",
			userID:    11,
			policy:    service.AffiliateSourcePolicyPartnerSelfUsage,
			partnerID: 11,
			customer:  0,
			partner:   900,
			wantError: true,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := validateAffiliateSourcePolicy(
				tt.userID,
				tt.policy,
				tt.partnerID,
				tt.customer,
				tt.partner,
			)
			if tt.wantError && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantError && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
