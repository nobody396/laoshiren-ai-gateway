package service

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestCompensationNoticePreviewAndFreezeRemainBoundedAtFiveThousandUsers(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	const userCount = 5000
	now := time.Now().UTC()
	rows := sqlmock.NewRows([]string{"draft_user_id", "user_id", "balance_cny_fen", "builder_pass_cny_fen", "impact_start", "impact_end", "products", "charged"})
	for i := 1; i <= userCount; i++ {
		rows.AddRow(int64(i), int64(10_000+i), int64(100), int64(200), now, now.Add(time.Hour), `{"OpenAI / Codex API"}`, false)
	}
	mock.ExpectQuery(`SELECT u\.id,u\.user_id`).WithArgs(int64(77)).WillReturnRows(rows)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	snapshots, err := buildCompensationNoticePreviews(ctx, db, 77)
	require.NoError(t, err)
	require.Len(t, snapshots, userCount)
	require.Len(t, snapshots[0].Preview.Services, 1)

	mock.ExpectBegin()
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	mock.ExpectExec(`INSERT INTO compensation_approval_notices`).WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, userCount))
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM compensation_approval_notices`).WithArgs(int64(88)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(userCount))
	require.NoError(t, freezeCompensationApprovalNoticesTx(ctx, tx, 88, snapshots))
	mock.ExpectRollback()
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet(), "preview must use one aggregate query and freeze one batch insert plus one readback")
}
