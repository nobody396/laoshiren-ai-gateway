package service

import "context"

type recordUsageCommissionRepoStub struct {
	CommissionRepository

	recordCh chan CommissionRecord
	records  []CommissionRecord
}

func (s *recordUsageCommissionRepoStub) Create(_ context.Context, record *CommissionRecord) error {
	if record == nil {
		return nil
	}
	copied := *record
	s.records = append(s.records, copied)
	if s.recordCh != nil {
		s.recordCh <- copied
	}
	return nil
}
