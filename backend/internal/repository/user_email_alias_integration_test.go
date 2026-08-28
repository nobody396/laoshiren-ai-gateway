//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryEmailAliasLookupMatchesGmailMailboxIdentity(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	repo := newUserRepositoryWithSQL(client, integrationDB)
	unique := fmt.Sprintf("alias-lookup-%d", time.Now().UnixNano())
	owner := mustCreateUser(t, client, &service.User{Email: unique + ".mailbox+primary@gmail.com"})
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, owner.ID) })

	exists, err := repo.ExistsByEmailAlias(ctx, unique+"mailbox+new@googlemail.com", 0)
	require.NoError(t, err)
	require.True(t, exists)

	exists, err = repo.ExistsByEmailAlias(ctx, unique+"mailbox+new@googlemail.com", owner.ID)
	require.NoError(t, err)
	require.False(t, exists)
}

func TestUserRepositoryConcurrentGmailAliasesCreateExactlyOneUser(t *testing.T) {
	ctx := context.Background()
	unique := fmt.Sprintf("alias-race-%d", time.Now().UnixNano())
	first := &service.User{Email: unique + ".mailbox+one@gmail.com", PasswordHash: "test-password-hash", Role: service.RoleUser, Status: service.StatusActive}
	second := &service.User{Email: unique + "mailbox+two@googlemail.com", PasswordHash: "test-password-hash", Role: service.RoleUser, Status: service.StatusActive}
	repos := []*userRepository{
		newUserRepositoryWithSQL(integrationEntClient, integrationDB),
		newUserRepositoryWithSQL(integrationEntClient, integrationDB),
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i, user := range []*service.User{first, second} {
		wg.Add(1)
		go func(repo *userRepository, candidate *service.User) {
			defer wg.Done()
			<-start
			errs <- repo.Create(ctx, candidate)
		}(repos[i], user)
	}
	close(start)
	wg.Wait()
	close(errs)

	var successes, conflicts int
	for err := range errs {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, service.ErrEmailExists):
			conflicts++
		default:
			t.Fatalf("unexpected create error: %v", err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)

	for _, user := range []*service.User{first, second} {
		if user.ID > 0 {
			_, _ = integrationDB.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, user.ID)
		}
	}
}
