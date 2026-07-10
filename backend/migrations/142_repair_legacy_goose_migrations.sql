-- Repair the forward state that was reverted when the custom migration runner
-- executed historical Goose Down sections. This migration is intentionally
-- forward-only, additive, idempotent, and compatible with the previous app.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '2min';

-- 037_ops_alert_silences.sql: recreate the table removed by its Down section.
CREATE TABLE IF NOT EXISTS ops_alert_silences (
    id BIGSERIAL PRIMARY KEY,
    rule_id BIGINT NOT NULL,
    platform VARCHAR(64) NOT NULL,
    group_id BIGINT,
    region VARCHAR(64),
    until TIMESTAMPTZ NOT NULL,
    reason TEXT,
    created_by BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ops_alert_silences_lookup
    ON ops_alert_silences (rule_id, platform, group_id, region, until);

-- 019_migrate_wechat_to_attributes.sql: restore exactly one active definition
-- and copy only safe legacy values. Keep users.wechat when it exists so the
-- previous application digest can still run after this migration.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS wechat VARCHAR(100) DEFAULT '';

UPDATE users
SET wechat = ''
WHERE wechat IS NULL;

ALTER TABLE users
    ALTER COLUMN wechat SET DEFAULT '',
    ALTER COLUMN wechat SET NOT NULL;

DO $repair_wechat$
DECLARE
    active_count BIGINT;
    wechat_attribute_id BIGINT;
    conflict_count BIGINT;
BEGIN
    LOCK TABLE user_attribute_definitions IN SHARE ROW EXCLUSIVE MODE;

    SELECT COUNT(*), MIN(id)
    INTO active_count, wechat_attribute_id
    FROM user_attribute_definitions
    WHERE key = 'wechat'
      AND deleted_at IS NULL;

    IF active_count > 1 THEN
        RAISE EXCEPTION 'multiple active wechat attribute definitions found';
    END IF;

    IF active_count = 0 THEN
        SELECT id
        INTO wechat_attribute_id
        FROM user_attribute_definitions
        WHERE key = 'wechat'
          AND deleted_at IS NOT NULL
        ORDER BY id DESC
        LIMIT 1;

        IF wechat_attribute_id IS NULL THEN
            INSERT INTO user_attribute_definitions (
                key,
                name,
                description,
                type,
                options,
                required,
                validation,
                placeholder,
                display_order,
                enabled,
                created_at,
                updated_at
            )
            VALUES (
                'wechat',
                '微信',
                '用户微信号',
                'text',
                '[]'::jsonb,
                false,
                '{}'::jsonb,
                '请输入微信号',
                -1,
                true,
                NOW(),
                NOW()
            )
            RETURNING id INTO wechat_attribute_id;
        ELSE
            UPDATE user_attribute_definitions
            SET
                deleted_at = NULL,
                enabled = true,
                display_order = -1,
                updated_at = NOW()
            WHERE id = wechat_attribute_id;
        END IF;
    ELSE
        UPDATE user_attribute_definitions
        SET
            enabled = true,
            display_order = LEAST(display_order, -1),
            updated_at = NOW()
        WHERE id = wechat_attribute_id;
    END IF;

    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'wechat'
    ) THEN
        EXECUTE format(
            $sql$
                SELECT COUNT(*)
                FROM users AS u
                JOIN user_attribute_values AS uav
                  ON uav.user_id = u.id
                 AND uav.attribute_id = %s
                WHERE u.deleted_at IS NULL
                  AND BTRIM(COALESCE(u.wechat, '')) <> ''
                  AND BTRIM(COALESCE(uav.value, '')) <> ''
                  AND uav.value IS DISTINCT FROM u.wechat
            $sql$,
            wechat_attribute_id
        ) INTO conflict_count;

        IF conflict_count > 0 THEN
            RAISE EXCEPTION 'conflicting non-empty WeChat values found for % user(s)', conflict_count;
        END IF;

        EXECUTE format(
            $sql$
                INSERT INTO user_attribute_values (
                    user_id,
                    attribute_id,
                    value,
                    created_at,
                    updated_at
                )
                SELECT
                    u.id,
                    %s,
                    u.wechat,
                    NOW(),
                    NOW()
                FROM users AS u
                WHERE u.deleted_at IS NULL
                  AND BTRIM(COALESCE(u.wechat, '')) <> ''
                ON CONFLICT (user_id, attribute_id) DO UPDATE
                SET
                    value = EXCLUDED.value,
                    updated_at = NOW()
                WHERE BTRIM(COALESCE(user_attribute_values.value, '')) = ''
            $sql$,
            wechat_attribute_id
        );

        EXECUTE format(
            $sql$
                UPDATE users AS u
                SET wechat = uav.value
                FROM user_attribute_values AS uav
                WHERE uav.user_id = u.id
                  AND uav.attribute_id = %s
                  AND BTRIM(COALESCE(u.wechat, '')) = ''
                  AND BTRIM(COALESCE(uav.value, '')) <> ''
            $sql$,
            wechat_attribute_id
        );
    END IF;
END
$repair_wechat$;

-- 024_add_gemini_tier_id.sql: restore the default only for the original target
-- population and never overwrite an existing tier.
UPDATE accounts
SET credentials = jsonb_set(
    credentials,
    '{tier_id}',
    '"LEGACY"',
    true
)
WHERE platform = 'gemini'
  AND type = 'oauth'
  AND jsonb_typeof(credentials) = 'object'
  AND credentials->>'tier_id' IS NULL
  AND (
    credentials->>'oauth_type' = 'code_assist'
    OR (
        credentials->>'oauth_type' IS NULL
        AND credentials->>'project_id' IS NOT NULL
    )
  );
