package service

import (
	"math"
	"testing"
)

func TestAffiliateMultiplyMicros(t *testing.T) {
	t.Parallel()
	got, err := AffiliateMultiplyMicros(10_000_000, 1200)
	if err != nil {
		t.Fatalf("multiply: %v", err)
	}
	if got != 12_000_000 {
		t.Fatalf("got %d", got)
	}
	if _, err := AffiliateMultiplyMicros(math.MaxInt64, 1200); err == nil {
		t.Fatal("expected overflow to be rejected")
	}
}
