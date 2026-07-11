package service

import "context"

// PreserveGatewayFailover keeps the existing error identity and retry contract;
// legacy handlers continue to own account-switch policy during strangling.
type PreserveGatewayFailover struct{}

func (PreserveGatewayFailover) Classify(_ context.Context, _ GatewayPipelineRequest, err error) error {
	return err
}
