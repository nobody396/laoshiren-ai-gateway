//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	infraerrors "github.com/bozhouDev/DragonCode-sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type bindUserRepoStub struct {
	mockUserRepo
	byID      map[int64]*User
	byEmail   map[string]*User
	bindCalls []bindUserRepoCall
	bindErr   error
}

type bindUserRepoCall struct {
	userID            int64
	agentID           int64
	setInviterIfEmpty bool
}

func (s *bindUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	if u := s.byID[id]; u != nil {
		return u, nil
	}
	return nil, ErrUserNotFound
}

func (s *bindUserRepoStub) GetByEmail(_ context.Context, email string) (*User, error) {
	if u := s.byEmail[email]; u != nil {
		return u, nil
	}
	return nil, ErrUserNotFound
}

func (s *bindUserRepoStub) AdminBindUserToAgent(_ context.Context, userID, agentID int64, setInviterIfEmpty bool) error {
	s.bindCalls = append(s.bindCalls, bindUserRepoCall{
		userID:            userID,
		agentID:           agentID,
		setInviterIfEmpty: setInviterIfEmpty,
	})
	if s.bindErr != nil {
		return s.bindErr
	}
	for _, u := range s.byEmail {
		if u.ID == userID {
			u.AgentID = &agentID
			if setInviterIfEmpty && u.InviterID == nil {
				u.InviterID = &agentID
			}
		}
	}
	return nil
}

func TestCommissionServiceBindUserToAgentBindsUnassignedUser(t *testing.T) {
	repo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
		byEmail: map[string]*User{
			"user@example.com": {ID: 42, Email: "user@example.com", Username: "user", Role: RoleUser},
		},
	}
	svc := NewCommissionService(repo, nil)

	result, err := svc.BindUserToAgent(context.Background(), 7, BindAgentUserInput{Email: " user@example.com "})
	require.NoError(t, err)
	require.Equal(t, int64(42), result.UserID)
	require.Equal(t, int64(7), result.AgentID)
	require.Nil(t, result.PreviousAgentID)
	require.NotNil(t, result.InviterID)
	require.Equal(t, int64(7), *result.InviterID)
	require.True(t, result.InviterIDChanged)
	require.False(t, result.AlreadyBound)
	require.False(t, result.Overwritten)
	require.Equal(t, []bindUserRepoCall{{userID: 42, agentID: 7, setInviterIfEmpty: true}}, repo.bindCalls)
}

func TestCommissionServiceBindUserToAgentRejectsExistingOtherAgentWithoutOverwrite(t *testing.T) {
	otherAgentID := int64(8)
	repo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
		byEmail: map[string]*User{
			"user@example.com": {ID: 42, Email: "user@example.com", Role: RoleUser, AgentID: &otherAgentID},
		},
	}
	svc := NewCommissionService(repo, nil)

	result, err := svc.BindUserToAgent(context.Background(), 7, BindAgentUserInput{Email: "user@example.com"})
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Empty(t, repo.bindCalls)
}

func TestCommissionServiceBindUserToAgentOverwritePreservesExistingInviter(t *testing.T) {
	otherAgentID := int64(8)
	inviterID := int64(99)
	repo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
		byEmail: map[string]*User{
			"user@example.com": {ID: 42, Email: "user@example.com", Role: RoleUser, AgentID: &otherAgentID, InviterID: &inviterID},
		},
	}
	svc := NewCommissionService(repo, nil)

	result, err := svc.BindUserToAgent(context.Background(), 7, BindAgentUserInput{Email: "user@example.com", OverwriteAgent: true})
	require.NoError(t, err)
	require.NotNil(t, result.PreviousAgentID)
	require.Equal(t, otherAgentID, *result.PreviousAgentID)
	require.NotNil(t, result.InviterID)
	require.Equal(t, inviterID, *result.InviterID)
	require.False(t, result.InviterIDChanged)
	require.True(t, result.Overwritten)
	require.Equal(t, []bindUserRepoCall{{userID: 42, agentID: 7, setInviterIfEmpty: false}}, repo.bindCalls)
}

func TestCommissionServiceBindUserToAgentSameAgentIsIdempotent(t *testing.T) {
	agentID := int64(7)
	repo := &bindUserRepoStub{
		byID: map[int64]*User{
			agentID: {ID: agentID, Role: RoleAgent},
		},
		byEmail: map[string]*User{
			"user@example.com": {ID: 42, Email: "user@example.com", Role: RoleUser, AgentID: &agentID, InviterID: &agentID},
		},
	}
	svc := NewCommissionService(repo, nil)

	result, err := svc.BindUserToAgent(context.Background(), agentID, BindAgentUserInput{Email: "user@example.com"})
	require.NoError(t, err)
	require.True(t, result.AlreadyBound)
	require.Empty(t, repo.bindCalls)
}

func TestCommissionServiceBindUserToAgentRejectsNonUserTarget(t *testing.T) {
	repo := &bindUserRepoStub{
		byID: map[int64]*User{
			7: {ID: 7, Role: RoleAgent},
		},
		byEmail: map[string]*User{
			"admin@example.com": {ID: 42, Email: "admin@example.com", Role: RoleAdmin},
		},
	}
	svc := NewCommissionService(repo, nil)

	result, err := svc.BindUserToAgent(context.Background(), 7, BindAgentUserInput{Email: "admin@example.com"})
	require.Nil(t, result)
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Empty(t, repo.bindCalls)
}
