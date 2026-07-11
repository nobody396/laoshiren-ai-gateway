package service

import "context"

type AccountingPipelineMeter struct {
	accounting *AccountingService
}

func NewAccountingPipelineMeter(accounting *AccountingService) *AccountingPipelineMeter {
	return &AccountingPipelineMeter{accounting: accounting}
}

func (m *AccountingPipelineMeter) Record(ctx context.Context, observation GatewayPipelineObservation) {
	if m != nil && m.accounting != nil {
		m.accounting.ObservePipelineOutcome(ctx, observation)
	}
}
