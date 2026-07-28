package service

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/enttest"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

type paymentCompleteOrderRepoStub struct {
	mu              sync.Mutex
	order           *PaymentOrder
	completeResults []bool
	completeCalls   int
	updateCalls     int
}

func (s *paymentCompleteOrderRepoStub) Create(context.Context, *PaymentOrder) error {
	panic("unexpected Create call")
}

func (s *paymentCompleteOrderRepoStub) GetByOrderNo(_ context.Context, orderNo string) (*PaymentOrder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.order == nil || s.order.OrderNo != orderNo {
		return nil, ErrPaymentNotFound
	}
	cp := *s.order
	return &cp, nil
}

func (s *paymentCompleteOrderRepoStub) UpdateStatus(context.Context, int64, string, *string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.updateCalls++
	return nil
}

func (s *paymentCompleteOrderRepoStub) CompleteIfPending(context.Context, int64, *string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.completeCalls++
	if len(s.completeResults) == 0 {
		return false, nil
	}
	result := s.completeResults[0]
	s.completeResults = s.completeResults[1:]
	return result, nil
}

func (s *paymentCompleteOrderRepoStub) UpdateQRCodeURL(context.Context, int64, string) error {
	panic("unexpected UpdateQRCodeURL call")
}

func (s *paymentCompleteOrderRepoStub) ListByUser(context.Context, int64, int) ([]PaymentOrder, error) {
	panic("unexpected ListByUser call")
}

type paymentAssignUserSubRepoStub struct {
	userSubRepoNoop

	mu          sync.Mutex
	createCalls int
	nextID      int64
	last        *UserSubscription
}

func (s *paymentAssignUserSubRepoStub) GetByUserIDAndGroupID(context.Context, int64, int64) (*UserSubscription, error) {
	return nil, ErrSubscriptionNotFound
}

func (s *paymentAssignUserSubRepoStub) Create(_ context.Context, sub *UserSubscription) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.createCalls++
	if s.nextID == 0 {
		s.nextID = 1
	}
	cp := *sub
	cp.ID = s.nextID
	s.nextID++
	sub.ID = cp.ID
	s.last = &cp
	return nil
}

func (s *paymentAssignUserSubRepoStub) GetByID(_ context.Context, id int64) (*UserSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.last == nil || s.last.ID != id {
		return nil, ErrSubscriptionNotFound
	}
	cp := *s.last
	return &cp, nil
}

func (s *paymentAssignUserSubRepoStub) countCreates() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createCalls
}

func newPaymentServiceTestEntClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:payment_service_idempotency_%d?mode=memory&cache=shared&_fk=1", time.Now().UnixNano()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func newPaymentCompleteOrderService(t *testing.T, repo PaymentOrderRepository, subRepo *paymentAssignUserSubRepoStub) *PaymentService {
	t.Helper()

	groupRepo := &subscriptionGroupRepoStub{
		group: &Group{ID: 7, SubscriptionType: SubscriptionTypeSubscription},
	}
	subscriptionSvc := NewSubscriptionService(groupRepo, subRepo, nil, nil, nil)
	return NewPaymentService(repo, subscriptionSvc, nil, newPaymentServiceTestEntClient(t), nil, nil)
}

func newPendingPaymentOrderForIdempotencyTest() *PaymentOrder {
	return &PaymentOrder{
		ID:           101,
		OrderNo:      "payment-order-idempotency",
		UserID:       1001,
		GroupID:      7,
		ValidityDays: 30,
		Status:       PaymentStatusPending,
	}
}

func TestPaymentServiceCompleteOrderConcurrentOnlyOneSubscriptionSideEffect(t *testing.T) {
	repo := &paymentCompleteOrderRepoStub{
		order:           newPendingPaymentOrderForIdempotencyTest(),
		completeResults: []bool{true, false},
	}
	subRepo := &paymentAssignUserSubRepoStub{}
	svc := newPaymentCompleteOrderService(t, repo, subRepo)

	start := make(chan struct{})
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			<-start
			errs <- svc.completeOrder(context.Background(), repo.order.OrderNo, ptrString("trade-idempotent"))
		}()
	}
	close(start)

	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	require.Equal(t, 2, repo.completeCalls)
	require.Equal(t, 0, repo.updateCalls, "completeOrder should not use unconditional status updates")
	require.Equal(t, 1, subRepo.countCreates(), "only the winning pending->completed transition grants a subscription")
}

func TestPaymentServiceCompleteOrderCompleteIfPendingFalseSkipsSubscription(t *testing.T) {
	repo := &paymentCompleteOrderRepoStub{
		order:           newPendingPaymentOrderForIdempotencyTest(),
		completeResults: []bool{false},
	}
	subRepo := &paymentAssignUserSubRepoStub{}
	svc := newPaymentCompleteOrderService(t, repo, subRepo)

	require.NoError(t, svc.completeOrder(context.Background(), repo.order.OrderNo, ptrString("trade-duplicate")))
	require.Equal(t, 1, repo.completeCalls)
	require.Equal(t, 0, subRepo.countCreates())
}

func TestPaymentServiceCompleteOrderPendingCompletionAssignsSubscription(t *testing.T) {
	repo := &paymentCompleteOrderRepoStub{
		order:           newPendingPaymentOrderForIdempotencyTest(),
		completeResults: []bool{true},
	}
	subRepo := &paymentAssignUserSubRepoStub{}
	svc := newPaymentCompleteOrderService(t, repo, subRepo)

	require.NoError(t, svc.completeOrder(context.Background(), repo.order.OrderNo, ptrString("trade-success")))
	require.Equal(t, 1, repo.completeCalls)
	require.Equal(t, 1, subRepo.countCreates())
}
