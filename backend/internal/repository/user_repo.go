package repository

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	dbent "github.com/bozhouDev/DragonCode-sub2api/ent"
	"github.com/bozhouDev/DragonCode-sub2api/ent/apikey"
	"github.com/bozhouDev/DragonCode-sub2api/ent/schema/mixins"
	dbuser "github.com/bozhouDev/DragonCode-sub2api/ent/user"
	"github.com/bozhouDev/DragonCode-sub2api/ent/userallowedgroup"
	"github.com/bozhouDev/DragonCode-sub2api/ent/usersubscription"
	"github.com/bozhouDev/DragonCode-sub2api/internal/pkg/pagination"
	"github.com/bozhouDev/DragonCode-sub2api/internal/service"
	"github.com/lib/pq"
)

type userRepository struct {
	client *dbent.Client
	sql    sqlExecutor
}

func NewUserRepository(client *dbent.Client, sqlDB *sql.DB) service.UserRepository {
	return newUserRepositoryWithSQL(client, sqlDB)
}

func newUserRepositoryWithSQL(client *dbent.Client, sqlq sqlExecutor) *userRepository {
	return &userRepository{client: client, sql: sqlq}
}

func (r *userRepository) Create(ctx context.Context, userIn *service.User) error {
	if userIn == nil {
		return nil
	}

	// Reuse an outer UnitOfWork/Ent transaction when present. Only create and
	// own a transaction when this repository is the outermost boundary.
	txClient, hasOuterTx := transactionClientFromContext(ctx)
	var tx *dbent.Tx
	var err error
	if !hasOuterTx {
		tx, err = r.client.Tx(ctx)
		if errors.Is(err, dbent.ErrTxStarted) {
			txClient = r.client
			err = nil
		}
		if err != nil {
			return err
		}
		if tx != nil {
			defer func() { _ = tx.Rollback() }()
			txClient = tx.Client()
		}
	}

	created, err := txClient.User.Create().
		SetEmail(userIn.Email).
		SetUsername(userIn.Username).
		SetNotes(userIn.Notes).
		SetPasswordHash(userIn.PasswordHash).
		SetRole(userIn.Role).
		SetBalance(userIn.Balance).
		SetConcurrency(userIn.Concurrency).
		SetStatus(userIn.Status).
		SetTokenVersion(userIn.TokenVersion).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, nil, service.ErrEmailExists)
	}
	var inviteCode string
	inviteCodeSaved := false
	for attempt := 0; attempt < 10; attempt++ {
		inviteCode = defaultAffiliateInviteCode(created.ID, attempt)
		if _, err := txClient.User.UpdateOneID(created.ID).
			SetInviteCode(inviteCode).
			Save(ctx); err != nil {
			if dbent.IsConstraintError(err) {
				continue
			}
			return translatePersistenceError(err, service.ErrUserNotFound, nil)
		}
		inviteCodeSaved = true
		break
	}
	if !inviteCodeSaved {
		return errors.New("failed to allocate affiliate invite code")
	}
	created.InviteCode = &inviteCode

	if err := r.syncUserAllowedGroupsWithClient(ctx, txClient, created.ID, userIn.AllowedGroups); err != nil {
		return err
	}

	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	applyUserEntityToService(userIn, created)
	return nil
}

func defaultAffiliateInviteCode(userID int64, attempt int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%d", userID, attempt)))
	return "U" + strings.ToUpper(hex.EncodeToString(sum[:])[:15])
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*service.User, error) {
	m, err := clientFromContext(ctx, r.client).User.Query().Where(dbuser.IDEQ(id)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}

	out := userEntityToService(m)
	groups, err := r.loadAllowedGroups(ctx, []int64{id})
	if err != nil {
		return nil, err
	}
	if v, ok := groups[id]; ok {
		out.AllowedGroups = v
	}
	return out, nil
}

// GetByIDIncludingDeleted is reserved for historical settlement that was
// accepted before a later soft-delete. It must not be used for authentication.
func (r *userRepository) GetByIDIncludingDeleted(ctx context.Context, id int64) (*service.User, error) {
	u, err := r.client.User.Query().Where(dbuser.IDEQ(id)).Only(mixins.SkipSoftDelete(ctx))
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}
	return userEntityToService(u), nil
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*service.User, error) {
	m, err := clientFromContext(ctx, r.client).User.Query().Where(dbuser.EmailEQ(email)).Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}

	out := userEntityToService(m)
	groups, err := r.loadAllowedGroups(ctx, []int64{m.ID})
	if err != nil {
		return nil, err
	}
	if v, ok := groups[m.ID]; ok {
		out.AllowedGroups = v
	}
	return out, nil
}

func (r *userRepository) Update(ctx context.Context, userIn *service.User) error {
	if userIn == nil {
		return nil
	}

	txClient, hasOuterTx := transactionClientFromContext(ctx)
	var tx *dbent.Tx
	var err error
	if !hasOuterTx {
		tx, err = r.client.Tx(ctx)
		if errors.Is(err, dbent.ErrTxStarted) {
			txClient = r.client
			err = nil
		}
		if err != nil {
			return err
		}
		if tx != nil {
			defer func() { _ = tx.Rollback() }()
			txClient = tx.Client()
		}
	}

	updateOp := txClient.User.UpdateOneID(userIn.ID).
		SetEmail(userIn.Email).
		SetUsername(userIn.Username).
		SetNotes(userIn.Notes).
		SetPasswordHash(userIn.PasswordHash).
		SetRole(userIn.Role).
		SetBalance(userIn.Balance).
		SetConcurrency(userIn.Concurrency).
		SetStatus(userIn.Status).
		SetTokenVersion(userIn.TokenVersion).
		SetTotalRecharged(userIn.TotalRecharged)
	updated, err := updateOp.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, service.ErrEmailExists)
	}

	if err := r.syncUserAllowedGroupsWithClient(ctx, txClient, updated.ID, userIn.AllowedGroups); err != nil {
		return err
	}

	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
	}

	userIn.UpdatedAt = updated.UpdatedAt
	userIn.TokenVersion = updated.TokenVersion
	return nil
}

func (r *userRepository) UpdatePasswordAndIncrementTokenVersion(ctx context.Context, userID int64, passwordHash string) (int64, error) {
	updated, err := clientFromContext(ctx, r.client).User.UpdateOneID(userID).
		SetPasswordHash(passwordHash).
		AddTokenVersion(1).
		Save(ctx)
	if err != nil {
		return 0, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return updated.TokenVersion, nil
}

func (r *userRepository) IncrementTokenVersion(ctx context.Context, userID int64) (int64, error) {
	updated, err := clientFromContext(ctx, r.client).User.UpdateOneID(userID).
		AddTokenVersion(1).
		Save(ctx)
	if err != nil {
		return 0, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return updated.TokenVersion, nil
}

func (r *userRepository) Delete(ctx context.Context, id int64) error {
	affected, err := clientFromContext(ctx, r.client).User.Delete().Where(dbuser.IDEQ(id)).Exec(ctx)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && strings.Contains(pqErr.Message, "TEAM_OWNER_TRANSFER_REQUIRED") {
			return service.ErrTeamOwnerTransferRequired
		}
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	if affected == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) TouchLastActive(ctx context.Context, userID int64, ts time.Time) error {
	if r.sql == nil || userID <= 0 {
		return nil
	}
	threshold := ts.Add(-60 * time.Second)
	sqlq := sqlExecutorFromContext(ctx, r.sql)
	_, err := sqlq.ExecContext(ctx, `
		UPDATE users
		SET last_active_at = $2
		WHERE id = $1
		  AND deleted_at IS NULL
		  AND (last_active_at IS NULL OR last_active_at < $3)
	`, userID, ts.UTC(), threshold.UTC())
	return err
}

func (r *userRepository) List(ctx context.Context, params pagination.PaginationParams) ([]service.User, *pagination.PaginationResult, error) {
	return r.ListWithFilters(ctx, params, service.UserListFilters{})
}

func (r *userRepository) ListWithFilters(ctx context.Context, params pagination.PaginationParams, filters service.UserListFilters) ([]service.User, *pagination.PaginationResult, error) {
	q := r.client.User.Query()

	if filters.Status != "" {
		q = q.Where(dbuser.StatusEQ(filters.Status))
	}
	if filters.Role != "" {
		q = q.Where(dbuser.RoleEQ(filters.Role))
	}
	if filters.Search != "" {
		q = q.Where(
			dbuser.Or(
				dbuser.EmailContainsFold(filters.Search),
				dbuser.UsernameContainsFold(filters.Search),
				dbuser.NotesContainsFold(filters.Search),
				dbuser.HasAPIKeysWith(apikey.KeyContainsFold(filters.Search)),
			),
		)
	}

	// If attribute filters are specified, we need to filter by user IDs first
	var allowedUserIDs []int64
	if len(filters.Attributes) > 0 {
		var attrErr error
		allowedUserIDs, attrErr = r.filterUsersByAttributes(ctx, filters.Attributes)
		if attrErr != nil {
			return nil, nil, attrErr
		}
		if len(allowedUserIDs) == 0 {
			// No users match the attribute filters
			return []service.User{}, paginationResultFromTotal(0, params), nil
		}
		q = q.Where(dbuser.IDIn(allowedUserIDs...))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	users, err := q.
		Offset(params.Offset()).
		Limit(params.Limit()).
		Order(dbent.Desc(dbuser.FieldID)).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	outUsers := make([]service.User, 0, len(users))
	if len(users) == 0 {
		return outUsers, paginationResultFromTotal(int64(total), params), nil
	}

	userIDs := make([]int64, 0, len(users))
	userMap := make(map[int64]*service.User, len(users))
	for i := range users {
		userIDs = append(userIDs, users[i].ID)
		u := userEntityToService(users[i])
		outUsers = append(outUsers, *u)
		userMap[u.ID] = &outUsers[len(outUsers)-1]
	}

	shouldLoadSubscriptions := filters.IncludeSubscriptions == nil || *filters.IncludeSubscriptions
	if shouldLoadSubscriptions {
		// Batch load active subscriptions with groups to avoid N+1.
		subs, err := r.client.UserSubscription.Query().
			Where(
				usersubscription.UserIDIn(userIDs...),
				usersubscription.StatusEQ(service.SubscriptionStatusActive),
			).
			WithGroup().
			All(ctx)
		if err != nil {
			return nil, nil, err
		}

		for i := range subs {
			if u, ok := userMap[subs[i].UserID]; ok {
				u.Subscriptions = append(u.Subscriptions, *userSubscriptionEntityToService(subs[i]))
			}
		}
	}

	allowedGroupsByUser, err := r.loadAllowedGroups(ctx, userIDs)
	if err != nil {
		return nil, nil, err
	}
	for id, u := range userMap {
		if groups, ok := allowedGroupsByUser[id]; ok {
			u.AllowedGroups = groups
		}
	}

	return outUsers, paginationResultFromTotal(int64(total), params), nil
}

// filterUsersByAttributes returns user IDs that match ALL the given attribute filters
func (r *userRepository) filterUsersByAttributes(ctx context.Context, attrs map[int64]string) ([]int64, error) {
	if len(attrs) == 0 {
		return nil, nil
	}

	if r.sql == nil {
		return nil, fmt.Errorf("sql executor is not configured")
	}

	clauses := make([]string, 0, len(attrs))
	args := make([]any, 0, len(attrs)*2+1)
	argIndex := 1
	for attrID, value := range attrs {
		clauses = append(clauses, fmt.Sprintf("(attribute_id = $%d AND value ILIKE $%d)", argIndex, argIndex+1))
		args = append(args, attrID, "%"+value+"%")
		argIndex += 2
	}

	query := fmt.Sprintf(
		`SELECT user_id
		 FROM user_attribute_values
		 WHERE %s
		 GROUP BY user_id
		 HAVING COUNT(DISTINCT attribute_id) = $%d`,
		strings.Join(clauses, " OR "),
		argIndex,
	)
	args = append(args, len(attrs))

	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	result := make([]int64, 0)
	for rows.Next() {
		var userID int64
		if scanErr := rows.Scan(&userID); scanErr != nil {
			return nil, scanErr
		}
		result = append(result, userID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *userRepository) UpdateBalance(ctx context.Context, id int64, amount float64) error {
	client := clientFromContext(ctx, r.client)
	n, err := client.User.Update().Where(dbuser.IDEQ(id)).AddBalance(amount).Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	if n == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

// DeductBalance 扣除用户余额
// 透支策略：允许余额变为负数，确保当前请求能够完成
// 中间件会阻止余额 <= 0 的用户发起后续请求
func (r *userRepository) DeductBalance(ctx context.Context, id int64, amount float64) error {
	client := clientFromContext(ctx, r.client)
	n, err := client.User.Update().
		Where(dbuser.IDEQ(id)).
		AddBalance(-amount).
		Save(ctx)
	if err != nil {
		return err
	}
	if n == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) UpdateConcurrency(ctx context.Context, id int64, amount int) error {
	client := clientFromContext(ctx, r.client)
	n, err := client.User.Update().Where(dbuser.IDEQ(id)).AddConcurrency(amount).Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	if n == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return clientFromContext(ctx, r.client).User.Query().Where(dbuser.EmailEQ(email)).Exist(ctx)
}

func (r *userRepository) AddGroupToAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	client := clientFromContext(ctx, r.client)
	return client.UserAllowedGroup.Create().
		SetUserID(userID).
		SetGroupID(groupID).
		OnConflictColumns(userallowedgroup.FieldUserID, userallowedgroup.FieldGroupID).
		DoNothing().
		Exec(ctx)
}

func (r *userRepository) RemoveGroupFromUserAllowedGroups(ctx context.Context, userID int64, groupID int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.UserAllowedGroup.Delete().
		Where(userallowedgroup.UserIDEQ(userID), userallowedgroup.GroupIDEQ(groupID)).
		Exec(ctx)
	return err
}

func (r *userRepository) RemoveGroupFromAllowedGroups(ctx context.Context, groupID int64) (int64, error) {
	// 仅操作 user_allowed_groups 联接表，legacy users.allowed_groups 列已弃用。
	affected, err := r.client.UserAllowedGroup.Delete().
		Where(userallowedgroup.GroupIDEQ(groupID)).
		Exec(ctx)
	if err != nil {
		return 0, err
	}
	return int64(affected), nil
}

func (r *userRepository) GetFirstAdmin(ctx context.Context) (*service.User, error) {
	m, err := clientFromContext(ctx, r.client).User.Query().
		Where(
			dbuser.RoleEQ(service.RoleAdmin),
			dbuser.StatusEQ(service.StatusActive),
		).
		Order(dbent.Asc(dbuser.FieldID)).
		First(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}

	out := userEntityToService(m)
	groups, err := r.loadAllowedGroups(ctx, []int64{m.ID})
	if err != nil {
		return nil, err
	}
	if v, ok := groups[m.ID]; ok {
		out.AllowedGroups = v
	}
	return out, nil
}

func (r *userRepository) loadAllowedGroups(ctx context.Context, userIDs []int64) (map[int64][]int64, error) {
	out := make(map[int64][]int64, len(userIDs))
	if len(userIDs) == 0 {
		return out, nil
	}

	rows, err := clientFromContext(ctx, r.client).UserAllowedGroup.Query().
		Where(userallowedgroup.UserIDIn(userIDs...)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	for i := range rows {
		out[rows[i].UserID] = append(out[rows[i].UserID], rows[i].GroupID)
	}

	for userID := range out {
		sort.Slice(out[userID], func(i, j int) bool { return out[userID][i] < out[userID][j] })
	}

	return out, nil
}

// syncUserAllowedGroupsWithClient 在 ent client/事务内同步用户允许分组：
// 仅操作 user_allowed_groups 联接表，legacy users.allowed_groups 列已弃用。
func (r *userRepository) syncUserAllowedGroupsWithClient(ctx context.Context, client *dbent.Client, userID int64, groupIDs []int64) error {
	if client == nil {
		return nil
	}

	// Keep join table as the source of truth for reads.
	if _, err := client.UserAllowedGroup.Delete().Where(userallowedgroup.UserIDEQ(userID)).Exec(ctx); err != nil {
		return err
	}

	unique := make(map[int64]struct{}, len(groupIDs))
	for _, id := range groupIDs {
		if id <= 0 {
			continue
		}
		unique[id] = struct{}{}
	}

	if len(unique) > 0 {
		creates := make([]*dbent.UserAllowedGroupCreate, 0, len(unique))
		for groupID := range unique {
			creates = append(creates, client.UserAllowedGroup.Create().SetUserID(userID).SetGroupID(groupID))
		}
		if err := client.UserAllowedGroup.
			CreateBulk(creates...).
			OnConflictColumns(userallowedgroup.FieldUserID, userallowedgroup.FieldGroupID).
			DoNothing().
			Exec(ctx); err != nil {
			return err
		}
	}

	return nil
}

func applyUserEntityToService(dst *service.User, src *dbent.User) {
	if dst == nil || src == nil {
		return
	}
	dst.ID = src.ID
	dst.TokenVersion = src.TokenVersion
	dst.InviteCode = src.InviteCode
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

// UpdateTotpSecret 更新用户的 TOTP 加密密钥
func (r *userRepository) UpdateTotpSecret(ctx context.Context, userID int64, encryptedSecret *string) error {
	client := clientFromContext(ctx, r.client)
	update := client.User.UpdateOneID(userID)
	if encryptedSecret == nil {
		update = update.ClearTotpSecretEncrypted()
	} else {
		update = update.SetTotpSecretEncrypted(*encryptedSecret)
	}
	_, err := update.Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return nil
}

// EnableTotp 启用用户的 TOTP 双因素认证
func (r *userRepository) EnableTotp(ctx context.Context, userID int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.User.UpdateOneID(userID).
		SetTotpEnabled(true).
		SetTotpEnabledAt(time.Now()).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return nil
}

// DisableTotp 禁用用户的 TOTP 双因素认证
func (r *userRepository) DisableTotp(ctx context.Context, userID int64) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.User.UpdateOneID(userID).
		SetTotpEnabled(false).
		ClearTotpEnabledAt().
		ClearTotpSecretEncrypted().
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return nil
}

// GetByInviteCode 根据邀请码查询用户
func (r *userRepository) GetByInviteCode(ctx context.Context, code string) (*service.User, error) {
	m, err := clientFromContext(ctx, r.client).User.Query().
		Where(dbuser.InviteCodeEQ(code)).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return userEntityToService(m), nil
}

// GetInviteCodeByUserID 查询用户的 invite_code 字段
func (r *userRepository) GetInviteCodeByUserID(ctx context.Context, userID int64) (*string, error) {
	m, err := clientFromContext(ctx, r.client).User.Query().
		Where(dbuser.IDEQ(userID)).
		Select(dbuser.FieldInviteCode).
		Only(ctx)
	if err != nil {
		return nil, translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return m.InviteCode, nil
}

// SetInviteCode 为用户设置邀请码（唯一）
func (r *userRepository) SetInviteCode(ctx context.Context, userID int64, code string) error {
	client := clientFromContext(ctx, r.client)
	_, err := client.User.UpdateOneID(userID).
		SetInviteCode(code).
		Save(ctx)
	if err != nil {
		return translatePersistenceError(err, service.ErrUserNotFound, nil)
	}
	return nil
}

// SetInviterAndAgent 设置用户的邀请人和归属代理商。
// 仅在 inviter_id 尚未绑定（IS NULL）时执行，已绑定则幂等忽略，防止重复覆盖。
func (r *userRepository) SetInviterAndAgent(ctx context.Context, userID, inviterID int64, agentID *int64) error {
	if r.sql == nil {
		return fmt.Errorf("sql executor is not configured")
	}
	executor := sqlExecutorFromContext(ctx, r.sql)
	var res interface{ RowsAffected() (int64, error) }
	var err error
	if agentID != nil {
		res, err = executor.ExecContext(ctx,
			`UPDATE users SET inviter_id=$1, agent_id=$2 WHERE id=$3 AND inviter_id IS NULL`,
			inviterID, *agentID, userID,
		)
	} else {
		res, err = executor.ExecContext(ctx,
			`UPDATE users SET inviter_id=$1 WHERE id=$2 AND inviter_id IS NULL`,
			inviterID, userID,
		)
	}
	if err != nil {
		return fmt.Errorf("set inviter and agent: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// 用户不存在或已绑定邀请人，幂等忽略
		return nil
	}
	if _, err := executor.ExecContext(ctx, `
		INSERT INTO affiliate_bindings (
			customer_user_id,
			inviter_user_id,
			binding_kind,
			customer_rebate_rate_snapshot_bps,
			agent_commission_rate_snapshot_bps,
			bound_at
		)
		VALUES ($1, $2, 'ordinary', 0, 0, NOW())
		ON CONFLICT (customer_user_id) DO NOTHING
	`, userID, inviterID); err != nil {
		return fmt.Errorf("create affiliate binding: %w", err)
	}
	if _, err := executor.ExecContext(ctx, `
		INSERT INTO affiliate_performance_events (
			user_id,
			direct_agent_id,
			event_type,
			amount_micros,
			source_type,
			source_id,
			event_key,
			occurred_at,
			metadata
		)
		VALUES (
			$1, $2, 'binding_created', 0,
			'referral_registration', $1, $3, NOW(), '{}'::jsonb
		)
		ON CONFLICT (event_key) DO NOTHING
	`, userID, inviterID, fmt.Sprintf("binding:user:%d", userID)); err != nil {
		return fmt.Errorf("create affiliate binding event: %w", err)
	}
	return nil
}

// AdminBindUserToAgent 设置用户的当前归属代理商。
// 管理员手动绑定只覆盖 agent_id；inviter_id 仅在为空时按需填充，保留已有邀请历史。
func (r *userRepository) AdminBindUserToAgent(ctx context.Context, userID, agentID int64, setInviterIfEmpty bool) error {
	if r.sql == nil {
		return fmt.Errorf("sql executor is not configured")
	}
	var res interface{ RowsAffected() (int64, error) }
	var err error
	if setInviterIfEmpty {
		res, err = r.sql.ExecContext(ctx,
			`UPDATE users
			 SET agent_id=$1, inviter_id=COALESCE(inviter_id, $1), updated_at=NOW()
			 WHERE id=$2 AND deleted_at IS NULL`,
			agentID, userID,
		)
	} else {
		res, err = r.sql.ExecContext(ctx,
			`UPDATE users
			 SET agent_id=$1, updated_at=NOW()
			 WHERE id=$2 AND deleted_at IS NULL`,
			agentID, userID,
		)
	}
	if err != nil {
		return fmt.Errorf("admin bind user to agent: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return service.ErrUserNotFound
	}
	return nil
}

// SetFirstRecharged 原子标记用户首充完成。
// 仅在 first_recharged=false 时更新，返回影响行数（0 表示已首充过，幂等保证）。
func (r *userRepository) SetFirstRecharged(ctx context.Context, userID int64) (int, error) {
	if r.sql == nil {
		return 0, fmt.Errorf("sql executor is not configured")
	}
	res, err := r.sql.ExecContext(ctx,
		`UPDATE users SET first_recharged = true WHERE id = $1 AND first_recharged = false`,
		userID,
	)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

// MarkFirstInvitedTopup 原子标记“被邀请用户首次虎皮椒充值”。
// 仅当用户存在邀请关系且尚未记录过首次虎皮椒充值时更新，返回影响行数（0 表示非邀请用户或已记录过）。
func (r *userRepository) MarkFirstInvitedTopup(ctx context.Context, userID, topupOrderID int64) (int, error) {
	if r.sql == nil {
		return 0, fmt.Errorf("sql executor is not configured")
	}
	res, err := r.sql.ExecContext(ctx, `
		UPDATE users
		SET first_invited_topup_at = NOW(),
		    first_invited_topup_order_id = $2
		WHERE id = $1
		  AND first_invited_topup_at IS NULL
		  AND (inviter_id IS NOT NULL OR agent_id IS NOT NULL)
	`, userID, topupOrderID)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(affected), nil
}

// CountInvitedByInviterID 统计以 inviterID 为邀请人的用户数（普通用户邀请统计）
func (r *userRepository) CountInvitedByInviterID(ctx context.Context, inviterID int64) (int64, error) {
	if r.sql == nil {
		return 0, fmt.Errorf("sql executor is not configured")
	}
	var count int64
	err := scanSingleRow(ctx, r.sql,
		`SELECT COUNT(*) FROM users WHERE inviter_id = $1 AND deleted_at IS NULL`,
		[]any{inviterID}, &count,
	)
	if err != nil {
		return 0, fmt.Errorf("count invited users: %w", err)
	}
	return count, nil
}
