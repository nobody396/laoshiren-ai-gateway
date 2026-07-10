package repository

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"strings"
	"testing"

	embeddedmigrations "github.com/bozhouDev/DragonCode-sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestHistoricalGooseMigrationChecksumsRemainImmutable(t *testing.T) {
	want := map[string]string{
		"019_migrate_wechat_to_attributes.sql": "d45e05b4bb722b287377790583c2677b8666dbf7e02b626c93468491d4ce8cf8",
		"024_add_gemini_tier_id.sql":           "b54de1b9a4423224f7aef5e644d1af115214d58dd61befd3c25db3e709b9163a",
		"037_ops_alert_silences.sql":           "72143a1ce3528ebc47472759c59011ec6993b25a3f22d50485538710047438c6",
	}

	for name, expected := range want {
		content, err := fs.ReadFile(embeddedmigrations.FS, name)
		require.NoError(t, err, name)
		sum := sha256.Sum256([]byte(strings.TrimSpace(string(content))))
		require.Equal(t, expected, hex.EncodeToString(sum[:]), name)
	}
}

func TestIsMigrationChecksumCompatible(t *testing.T) {
	t.Run("054历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"054_drop_legacy_cache_columns.sql",
			"182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d",
		)
		require.True(t, ok)
	})

	t.Run("054在未知文件checksum下不兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"054_drop_legacy_cache_columns.sql",
			"182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"0000000000000000000000000000000000000000000000000000000000000000",
		)
		require.False(t, ok)
	})

	t.Run("061历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"061_add_usage_log_request_type.sql",
			"08a248652cbab7cfde147fc6ef8cda464f2477674e20b718312faa252e0481c0",
			"66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c",
		)
		require.True(t, ok)
	})

	t.Run("061第二个历史checksum可兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"061_add_usage_log_request_type.sql",
			"222b4a09c797c22e5922b6b172327c824f5463aaa8760e4f621bc5c22e2be0f3",
			"66207e7aa5dd0429c2e2c0fabdaf79783ff157fa0af2e81adff2ee03790ec65c",
		)
		require.True(t, ok)
	})

	t.Run("111历史checksum可兼容排除source_id零值修复", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"111_add_commission_consumption_idempotency.sql",
			"bbbd704d99e4700da6fed36f0eccf05afbc71319cbda1cfc4e8ccbd7aa1f5dda",
			"c4f13930e5049baeec9b80757c9dcb8420d3e85a16a99aeba6fa55aecf7574e2",
		)
		require.True(t, ok)
	})

	t.Run("非白名单迁移不兼容", func(t *testing.T) {
		ok := isMigrationChecksumCompatible(
			"001_init.sql",
			"182c193f3359946cf094090cd9e57d5c3fd9abaffbc1e8fc378646b8a6fa12b4",
			"82de761156e03876653e7a6a4eee883cd927847036f779b0b9f34c42a8af7a7d",
		)
		require.False(t, ok)
	})
}
