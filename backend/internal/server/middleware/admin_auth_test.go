//go:build unit

package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/bozhouDev/DragonCode-sub2api/internal/config"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestAdminAuthAPIKeyUsesDeterministicServicePrincipal(t *testing.T) {
	gin.SetMode(gin.TestMode)

	const storedKey = "admin-test-key"
	settingService := service.NewSettingService(&bmSettingRepo{
		values: map[string]string{
			service.SettingKeyAdminAPIKey: storedKey,
		},
	}, &config.Config{})

	userRepo := &stubUserRepo{
		getFirstAdmin: func(ctx context.Context) (*service.User, error) {
			return &service.User{
				ID:          101,
				Role:        service.RoleAdmin,
				Status:      service.StatusActive,
				Concurrency: 9,
			}, nil
		},
	}
	userService := service.NewUserService(userRepo, nil, nil)

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAdminAuthMiddleware(nil, userService, settingService, nil)))
	router.Use(RequireAPIPermission())
	router.DELETE("/api/v1/admin/users/:id", func(c *gin.Context) {
		subject, ok := GetAuthSubjectFromContext(c)
		require.True(t, ok)
		role, ok := GetUserRoleFromContext(c)
		require.True(t, ok)
		perms, ok := GetPermissionsFromContext(c)
		require.True(t, ok)
		authMethod, _ := c.Get("auth_method")
		c.JSON(http.StatusOK, gin.H{
			"auth_method": authMethod,
			"user_id":     subject.UserID,
			"concurrency": subject.Concurrency,
			"role":        role,
			"permissions": perms,
			"super":       IsSuperAdminFromContext(c),
		})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/123", nil)
	req.Header.Set("x-api-key", storedKey)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, 0, userRepo.getFirstAdminCalls)

	var body struct {
		AuthMethod  string   `json:"auth_method"`
		UserID      int64    `json:"user_id"`
		Concurrency int      `json:"concurrency"`
		Role        string   `json:"role"`
		Permissions []string `json:"permissions"`
		Super       bool     `json:"super"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, "admin_api_key", body.AuthMethod)
	require.Equal(t, int64(-1), body.UserID)
	require.Equal(t, 0, body.Concurrency)
	require.Equal(t, service.RoleAdmin, body.Role)
	require.Equal(t, []string{"*"}, body.Permissions)
	require.True(t, body.Super)
}

func TestAdminAuthAPIKeyInvalidOrMissingReturnsUnauthorized(t *testing.T) {
	gin.SetMode(gin.TestMode)

	buildRouter := func(storedKey string) (*gin.Engine, *stubUserRepo) {
		values := map[string]string{}
		if storedKey != "" {
			values[service.SettingKeyAdminAPIKey] = storedKey
		}
		settingService := service.NewSettingService(&bmSettingRepo{values: values}, &config.Config{})
		userRepo := &stubUserRepo{
			getFirstAdmin: func(ctx context.Context) (*service.User, error) {
				return &service.User{ID: 101, Role: service.RoleAdmin, Status: service.StatusActive}, nil
			},
		}
		userService := service.NewUserService(userRepo, nil, nil)
		router := gin.New()
		router.Use(gin.HandlerFunc(NewAdminAuthMiddleware(nil, userService, settingService, nil)))
		router.GET("/t", func(c *gin.Context) {
			c.Status(http.StatusOK)
		})
		return router, userRepo
	}

	tests := []struct {
		name      string
		storedKey string
		headerKey *string
	}{
		{name: "missing_header", storedKey: "admin-test-key"},
		{name: "invalid_header", storedKey: "admin-test-key", headerKey: stringPtr("wrong-key")},
		{name: "not_configured", headerKey: stringPtr("admin-test-key")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router, userRepo := buildRouter(tc.storedKey)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/t", nil)
			if tc.headerKey != nil {
				req.Header.Set("x-api-key", *tc.headerKey)
			}

			router.ServeHTTP(w, req)

			require.Equal(t, http.StatusUnauthorized, w.Code)
			require.Equal(t, 0, userRepo.getFirstAdminCalls)
		})
	}
}

func TestAdminAuthJWTUsesActualUserRBACContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1}}
	authService := service.NewAuthService(nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil)

	admin := &service.User{
		ID:           77,
		Email:        "admin@example.com",
		Role:         service.RoleAdmin,
		Status:       service.StatusActive,
		TokenVersion: 3,
		Concurrency:  4,
	}

	userRepo := &stubUserRepo{
		getByID: func(ctx context.Context, id int64) (*service.User, error) {
			if id != admin.ID {
				return nil, service.ErrUserNotFound
			}
			clone := *admin
			return &clone, nil
		},
	}
	userService := service.NewUserService(userRepo, nil, nil)
	rbacRepo := &adminAuthRBACRepo{
		rolesByUserID: map[int64][]*service.AdminRole{
			admin.ID: {{ID: 11, Name: "ops", Status: service.ResourceStatusActive}},
		},
		apisByRoleID: map[int64][]*service.AdminAPI{
			11: {{ID: 21, Method: http.MethodGet, Path: "/admin/users", Status: service.ResourceStatusActive}},
		},
	}
	rbacService := service.NewRBACService(rbacRepo, &adminAuthRBACCache{})

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAdminAuthMiddleware(authService, userService, nil, rbacService)))
	router.GET("/t", func(c *gin.Context) {
		subject, ok := GetAuthSubjectFromContext(c)
		require.True(t, ok)
		perms, ok := GetPermissionsFromContext(c)
		require.True(t, ok)
		c.JSON(http.StatusOK, gin.H{
			"user_id":     subject.UserID,
			"permissions": perms,
			"super":       IsSuperAdminFromContext(c),
		})
	})

	token, err := authService.GenerateToken(&service.User{
		ID:           admin.ID,
		Email:        admin.Email,
		Role:         admin.Role,
		TokenVersion: admin.TokenVersion,
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/t", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, []int64{admin.ID, admin.ID}, rbacRepo.getRolesByUserIDCalls)

	var body struct {
		UserID      int64    `json:"user_id"`
		Permissions []string `json:"permissions"`
		Super       bool     `json:"super"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, admin.ID, body.UserID)
	require.Equal(t, []string{service.APIPermissionKey(http.MethodGet, "/admin/users")}, body.Permissions)
	require.False(t, body.Super)
}

func TestAdminAuthJWTValidatesTokenVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{JWT: config.JWTConfig{Secret: "test-secret", ExpireHour: 1}}
	authService := service.NewAuthService(nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil, nil, nil)

	admin := &service.User{
		ID:           1,
		Email:        "admin@example.com",
		Role:         service.RoleAdmin,
		Status:       service.StatusActive,
		TokenVersion: 2,
		Concurrency:  1,
	}

	userRepo := &stubUserRepo{
		getByID: func(ctx context.Context, id int64) (*service.User, error) {
			if id != admin.ID {
				return nil, service.ErrUserNotFound
			}
			clone := *admin
			return &clone, nil
		},
	}
	userService := service.NewUserService(userRepo, nil, nil)

	router := gin.New()
	router.Use(gin.HandlerFunc(NewAdminAuthMiddleware(authService, userService, nil, nil)))
	router.GET("/t", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	t.Run("token_version_mismatch_rejected", func(t *testing.T) {
		token, err := authService.GenerateToken(&service.User{
			ID:           admin.ID,
			Email:        admin.Email,
			Role:         admin.Role,
			TokenVersion: admin.TokenVersion - 1,
		})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code)
		require.Contains(t, w.Body.String(), "TOKEN_REVOKED")
	})

	t.Run("token_version_match_allows", func(t *testing.T) {
		token, err := authService.GenerateToken(&service.User{
			ID:           admin.ID,
			Email:        admin.Email,
			Role:         admin.Role,
			TokenVersion: admin.TokenVersion,
		})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("websocket_token_version_mismatch_rejected", func(t *testing.T) {
		token, err := authService.GenerateToken(&service.User{
			ID:           admin.ID,
			Email:        admin.Email,
			Role:         admin.Role,
			TokenVersion: admin.TokenVersion - 1,
		})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Sec-WebSocket-Protocol", "sub2api-admin, jwt."+token)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusUnauthorized, w.Code)
		require.Contains(t, w.Body.String(), "TOKEN_REVOKED")
	})

	t.Run("websocket_token_version_match_allows", func(t *testing.T) {
		token, err := authService.GenerateToken(&service.User{
			ID:           admin.ID,
			Email:        admin.Email,
			Role:         admin.Role,
			TokenVersion: admin.TokenVersion,
		})
		require.NoError(t, err)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/t", nil)
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Sec-WebSocket-Protocol", "sub2api-admin, jwt."+token)
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
	})
}

type stubUserRepo struct {
	service.UserRepository
	getByID            func(ctx context.Context, id int64) (*service.User, error)
	getFirstAdmin      func(ctx context.Context) (*service.User, error)
	getFirstAdminCalls int
}

func (s *stubUserRepo) Create(ctx context.Context, user *service.User) error {
	panic("unexpected Create call")
}

func (s *stubUserRepo) GetByID(ctx context.Context, id int64) (*service.User, error) {
	if s.getByID == nil {
		panic("GetByID not stubbed")
	}
	return s.getByID(ctx, id)
}

func (s *stubUserRepo) GetByEmail(ctx context.Context, email string) (*service.User, error) {
	panic("unexpected GetByEmail call")
}

func (s *stubUserRepo) GetFirstAdmin(ctx context.Context) (*service.User, error) {
	s.getFirstAdminCalls++
	if s.getFirstAdmin == nil {
		panic("unexpected GetFirstAdmin call")
	}
	return s.getFirstAdmin(ctx)
}

func (s *stubUserRepo) Update(ctx context.Context, user *service.User) error {
	panic("unexpected Update call")
}

func (s *stubUserRepo) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (s *stubUserRepo) TouchLastActive(ctx context.Context, userID int64, ts time.Time) error {
	return nil
}

func (s *stubUserRepo) List(ctx context.Context, params pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (s *stubUserRepo) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (s *stubUserRepo) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	panic("unexpected UpdateBalance call")
}

func (s *stubUserRepo) DeductBalance(ctx context.Context, id int64, amount float64) error {
	panic("unexpected DeductBalance call")
}

func (s *stubUserRepo) UpdateConcurrency(ctx context.Context, id int64, amount int) error {
	panic("unexpected UpdateConcurrency call")
}

func (s *stubUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	panic("unexpected ExistsByEmail call")
}

func (s *stubUserRepo) RemoveGroupFromAllowedGroups(ctx context.Context, groupID int64) (int64, error) {
	panic("unexpected RemoveGroupFromAllowedGroups call")
}

func (s *stubUserRepo) AddGroupToAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	panic("unexpected AddGroupToAllowedGroups call")
}

func (s *stubUserRepo) UpdateTotpSecret(ctx context.Context, userID int64, encryptedSecret *string) error {
	panic("unexpected UpdateTotpSecret call")
}

func (s *stubUserRepo) EnableTotp(ctx context.Context, userID int64) error {
	panic("unexpected EnableTotp call")
}

func (s *stubUserRepo) DisableTotp(ctx context.Context, userID int64) error {
	panic("unexpected DisableTotp call")
}

type adminAuthRBACRepo struct {
	service.RBACRepository
	rolesByUserID         map[int64][]*service.AdminRole
	apisByRoleID          map[int64][]*service.AdminAPI
	getRolesByUserIDCalls []int64
}

func (r *adminAuthRBACRepo) GetRolesByUserID(ctx context.Context, userID int64) ([]*service.AdminRole, error) {
	r.getRolesByUserIDCalls = append(r.getRolesByUserIDCalls, userID)
	return r.rolesByUserID[userID], nil
}

func (r *adminAuthRBACRepo) GetMenusByRoleIDs(ctx context.Context, roleIDs []int64) ([]*service.AdminMenu, error) {
	return nil, nil
}

func (r *adminAuthRBACRepo) GetAPIsByRoleIDs(ctx context.Context, roleIDs []int64) ([]*service.AdminAPI, error) {
	var apis []*service.AdminAPI
	for _, roleID := range roleIDs {
		apis = append(apis, r.apisByRoleID[roleID]...)
	}
	return apis, nil
}

type adminAuthRBACCache struct {
	service.RBACCache
}

func (c *adminAuthRBACCache) GetUserPermissionKeys(context.Context, int64) ([]string, error) {
	return nil, nil
}

func (c *adminAuthRBACCache) SetUserPermissionKeys(context.Context, int64, []string) error {
	return nil
}
