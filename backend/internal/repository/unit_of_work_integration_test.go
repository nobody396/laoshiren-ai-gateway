//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func unitOfWorkTestEmail(t *testing.T, suffix string) string {
	t.Helper()
	return fmt.Sprintf("uow-%d-%s@example.com", time.Now().UnixNano(), suffix)
}

func cleanupUnitOfWorkFixtures(t *testing.T, codes, emails []string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		if len(codes) > 0 {
			_, _ = integrationDB.ExecContext(ctx, `DELETE FROM redeem_codes WHERE code = ANY($1)`, pq.Array(codes))
		}
		if len(emails) > 0 {
			_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE email = ANY($1)`, pq.Array(emails))
		}
	})
}

func TestUnitOfWork_RegistrationRollbackAndCommit(t *testing.T) {
	ctx := context.Background()
	uow := NewUnitOfWork(integrationDB)
	userRepo := newUserRepositoryWithSQL(integrationEntClient, integrationDB)
	redeemRepo := NewRedeemCodeRepository(integrationEntClient).(*redeemCodeRepository)

	rollbackEmail := unitOfWorkTestEmail(t, "rollback")
	successEmail := unitOfWorkTestEmail(t, "success")
	codeValue := fmt.Sprintf("UOW-SUCCESS-%d", time.Now().UnixNano())
	cleanupUnitOfWorkFixtures(t, []string{codeValue}, []string{rollbackEmail, successEmail})

	rolledBackUser := &service.User{Email: rollbackEmail, PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive}
	err := uow.WithinTx(ctx, func(txCtx context.Context) error {
		require.NoError(t, userRepo.Create(txCtx, rolledBackUser))
		return redeemRepo.Use(txCtx, -1, rolledBackUser.ID)
	})
	require.Error(t, err)
	exists, err := userRepo.ExistsByEmail(ctx, rollbackEmail)
	require.NoError(t, err)
	require.False(t, exists, "invitation failure must roll back user creation")

	invitation := &service.RedeemCode{Code: codeValue, Type: service.RedeemTypeInvitation, Status: service.StatusUnused}
	require.NoError(t, redeemRepo.Create(ctx, invitation))
	committedUser := &service.User{Email: successEmail, PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive}
	require.NoError(t, uow.WithinTx(ctx, func(txCtx context.Context) error {
		require.NoError(t, userRepo.Create(txCtx, committedUser))
		return redeemRepo.Use(txCtx, invitation.ID, committedUser.ID)
	}))
	exists, err = userRepo.ExistsByEmail(ctx, successEmail)
	require.NoError(t, err)
	require.True(t, exists)
	used, err := redeemRepo.GetByCode(ctx, codeValue)
	require.NoError(t, err)
	require.Equal(t, service.StatusUsed, used.Status)
	require.NotNil(t, used.UsedBy)
	require.Equal(t, committedUser.ID, *used.UsedBy)
}

func TestUnitOfWork_ConcurrentInvitationAtMostOneUser(t *testing.T) {
	ctx := context.Background()
	uow := NewUnitOfWork(integrationDB)
	userRepo := newUserRepositoryWithSQL(integrationEntClient, integrationDB)
	redeemRepo := NewRedeemCodeRepository(integrationEntClient).(*redeemCodeRepository)
	emails := []string{unitOfWorkTestEmail(t, "race-a"), unitOfWorkTestEmail(t, "race-b")}
	codeValue := fmt.Sprintf("UOW-RACE-%d", time.Now().UnixNano())
	cleanupUnitOfWorkFixtures(t, []string{codeValue}, emails)

	invitation := &service.RedeemCode{Code: codeValue, Type: service.RedeemTypeInvitation, Status: service.StatusUnused}
	require.NoError(t, redeemRepo.Create(ctx, invitation))

	var wg sync.WaitGroup
	errs := make(chan error, len(emails))
	for _, email := range emails {
		email := email
		wg.Add(1)
		go func() {
			defer wg.Done()
			user := &service.User{Email: email, PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive}
			errs <- uow.WithinTx(ctx, func(txCtx context.Context) error {
				if err := userRepo.Create(txCtx, user); err != nil {
					return err
				}
				return redeemRepo.Use(txCtx, invitation.ID, user.ID)
			})
		}()
	}
	wg.Wait()
	close(errs)
	successes := 0
	for err := range errs {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE email = ANY($1)`, pq.Array(emails)).Scan(&count))
	require.Equal(t, 1, count)
}

func TestUnitOfWork_NestedAndPanicRollbackRealPostgres(t *testing.T) {
	ctx := context.Background()
	uow := NewUnitOfWork(integrationDB)
	userRepo := newUserRepositoryWithSQL(integrationEntClient, integrationDB)
	email := unitOfWorkTestEmail(t, "panic")
	cleanupUnitOfWorkFixtures(t, nil, []string{email})

	require.Panics(t, func() {
		_ = uow.WithinTx(ctx, func(txCtx context.Context) error {
			user := &service.User{Email: email, PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive}
			require.NoError(t, userRepo.Create(txCtx, user))
			require.NoError(t, uow.WithinTx(txCtx, func(nested context.Context) error {
				require.True(t, clientFromContext(nested, integrationEntClient) != integrationEntClient)
				return nil
			}))
			panic("rollback")
		})
	})
	exists, err := userRepo.ExistsByEmail(ctx, email)
	require.NoError(t, err)
	require.False(t, exists)
}
