package service

import "testing"

func TestValidateAffiliateCustomerRate(t *testing.T) {
	t.Parallel()
	for _, rate := range []int32{0, 100, 300, 1_000} {
		if err := validateAffiliateCustomerRate(rate); err != nil {
			t.Fatalf("rate %d rejected: %v", rate, err)
		}
	}
	for _, rate := range []int32{-100, 50, 1_100} {
		if err := validateAffiliateCustomerRate(rate); err == nil {
			t.Fatalf("rate %d accepted", rate)
		}
	}
}
