package service

import (
	"context"
	"errors"
	"testing"
)

func TestShouldFailoverOpenAITransportError(t *testing.T) {
	transportErr := errors.New("connection reset")

	if !shouldFailoverOpenAITransportError(context.Background(), transportErr) {
		t.Fatal("live request transport errors should be eligible for account failover")
	}
	if shouldFailoverOpenAITransportError(context.Background(), nil) {
		t.Fatal("nil errors must not trigger failover")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if shouldFailoverOpenAITransportError(canceled, transportErr) {
		t.Fatal("canceled client requests must not create another upstream attempt")
	}

	deadline, cancelDeadline := context.WithTimeout(context.Background(), 0)
	defer cancelDeadline()
	<-deadline.Done()
	if shouldFailoverOpenAITransportError(deadline, transportErr) {
		t.Fatal("expired client requests must not create another upstream attempt")
	}
}
