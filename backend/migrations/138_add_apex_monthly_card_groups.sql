-- Add Apex monthly-card subscription groups.
-- Apex is a sellable subscription-card product with one shared credit pool across
-- the GPT and Claude monthly groups. It intentionally has no weekly limit.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

DO $$
DECLARE
    gpt_source_group_id BIGINT;
    claude_source_group_id BIGINT;
    gpt_apex_group_id BIGINT;
    claude_apex_group_id BIGINT;
BEGIN
    SELECT id INTO gpt_source_group_id
    FROM groups
    WHERE deleted_at IS NULL
      AND name = 'GPT Ultra 月卡组'
    ORDER BY id
    LIMIT 1;

    SELECT id INTO claude_source_group_id
    FROM groups
    WHERE deleted_at IS NULL
      AND name = 'Claude Ultra 月卡组'
    ORDER BY id
    LIMIT 1;

    IF gpt_source_group_id IS NULL THEN
        RAISE EXCEPTION 'source group GPT Ultra 月卡组 not found';
    END IF;

    IF claude_source_group_id IS NULL THEN
        RAISE EXCEPTION 'source group Claude Ultra 月卡组 not found';
    END IF;

    SELECT id INTO gpt_apex_group_id
    FROM groups
    WHERE deleted_at IS NULL
      AND name = 'GPT Apex 月卡组'
    ORDER BY id
    LIMIT 1;

    IF gpt_apex_group_id IS NULL THEN
        INSERT INTO groups (
            name,
            description,
            rate_multiplier,
            is_exclusive,
            status,
            platform,
            subscription_type,
            daily_limit_usd,
            weekly_limit_usd,
            monthly_limit_usd,
            default_validity_days,
            image_price_1k,
            image_price_2k,
            image_price_4k,
            gpt_image_call_price,
            claude_code_only,
            fallback_group_id,
            fallback_group_id_on_invalid_request,
            model_routing,
            model_routing_enabled,
            mcp_xml_inject,
            supported_model_scopes,
            sort_order,
            allow_messages_dispatch,
            require_oauth_only,
            require_privacy_set,
            default_mapped_model,
            messages_dispatch_model_config,
            created_at,
            updated_at
        )
        SELECT
            'GPT Apex 月卡组',
            'GPT Apex 月卡专用 credit 订阅分组；0.4 credit = 1 美元 GPT Pro 用量；月额度 2898 credits，不设置周限制；复用月卡 Codex 上游账号。',
            0.4,
            is_exclusive,
            'active',
            platform,
            'credit',
            NULL,
            NULL,
            2898,
            30,
            image_price_1k,
            image_price_2k,
            image_price_4k,
            gpt_image_call_price,
            claude_code_only,
            fallback_group_id,
            fallback_group_id_on_invalid_request,
            model_routing,
            model_routing_enabled,
            mcp_xml_inject,
            supported_model_scopes,
            1050,
            allow_messages_dispatch,
            require_oauth_only,
            require_privacy_set,
            default_mapped_model,
            messages_dispatch_model_config,
            NOW(),
            NOW()
        FROM groups
        WHERE id = gpt_source_group_id
        RETURNING id INTO gpt_apex_group_id;
    ELSE
        UPDATE groups
        SET
            description = 'GPT Apex 月卡专用 credit 订阅分组；0.4 credit = 1 美元 GPT Pro 用量；月额度 2898 credits，不设置周限制；复用月卡 Codex 上游账号。',
            rate_multiplier = 0.4,
            status = 'active',
            subscription_type = 'credit',
            daily_limit_usd = NULL,
            weekly_limit_usd = NULL,
            monthly_limit_usd = 2898,
            default_validity_days = 30,
            sort_order = 1050,
            updated_at = NOW()
        WHERE id = gpt_apex_group_id;
    END IF;

    SELECT id INTO claude_apex_group_id
    FROM groups
    WHERE deleted_at IS NULL
      AND name = 'Claude Apex 月卡组'
    ORDER BY id
    LIMIT 1;

    IF claude_apex_group_id IS NULL THEN
        INSERT INTO groups (
            name,
            description,
            rate_multiplier,
            is_exclusive,
            status,
            platform,
            subscription_type,
            daily_limit_usd,
            weekly_limit_usd,
            monthly_limit_usd,
            default_validity_days,
            image_price_1k,
            image_price_2k,
            image_price_4k,
            gpt_image_call_price,
            claude_code_only,
            fallback_group_id,
            fallback_group_id_on_invalid_request,
            model_routing,
            model_routing_enabled,
            mcp_xml_inject,
            supported_model_scopes,
            sort_order,
            allow_messages_dispatch,
            require_oauth_only,
            require_privacy_set,
            default_mapped_model,
            messages_dispatch_model_config,
            created_at,
            updated_at
        )
        SELECT
            'Claude Apex 月卡组',
            'Claude Apex 月卡专用 credit 订阅分组；1.25 credit = 1 美元 Claude Max 用量；月额度 2898 credits，不设置周限制；复用月卡 Claude 上游账号。',
            1.25,
            is_exclusive,
            'active',
            platform,
            'credit',
            NULL,
            NULL,
            2898,
            30,
            image_price_1k,
            image_price_2k,
            image_price_4k,
            gpt_image_call_price,
            claude_code_only,
            fallback_group_id,
            fallback_group_id_on_invalid_request,
            model_routing,
            model_routing_enabled,
            mcp_xml_inject,
            supported_model_scopes,
            1150,
            allow_messages_dispatch,
            require_oauth_only,
            require_privacy_set,
            default_mapped_model,
            messages_dispatch_model_config,
            NOW(),
            NOW()
        FROM groups
        WHERE id = claude_source_group_id
        RETURNING id INTO claude_apex_group_id;
    ELSE
        UPDATE groups
        SET
            description = 'Claude Apex 月卡专用 credit 订阅分组；1.25 credit = 1 美元 Claude Max 用量；月额度 2898 credits，不设置周限制；复用月卡 Claude 上游账号。',
            rate_multiplier = 1.25,
            status = 'active',
            subscription_type = 'credit',
            daily_limit_usd = NULL,
            weekly_limit_usd = NULL,
            monthly_limit_usd = 2898,
            default_validity_days = 30,
            sort_order = 1150,
            updated_at = NOW()
        WHERE id = claude_apex_group_id;
    END IF;

    INSERT INTO account_groups (account_id, group_id, priority, created_at)
    SELECT account_id, gpt_apex_group_id, priority, NOW()
    FROM account_groups
    WHERE group_id = gpt_source_group_id
    ON CONFLICT (account_id, group_id) DO UPDATE
    SET priority = EXCLUDED.priority;

    INSERT INTO account_groups (account_id, group_id, priority, created_at)
    SELECT account_id, claude_apex_group_id, priority, NOW()
    FROM account_groups
    WHERE group_id = claude_source_group_id
    ON CONFLICT (account_id, group_id) DO UPDATE
    SET priority = EXCLUDED.priority;

    INSERT INTO scheduler_outbox (event_type, group_id, payload)
    VALUES
        (
            'group_changed',
            gpt_apex_group_id,
            jsonb_build_object(
                'reason', 'apex_monthly_card_group_upsert',
                'name', 'GPT Apex 月卡组',
                'monthly_limit_usd', 2898,
                'weekly_limit_usd', NULL,
                'rate_multiplier', 0.4
            )
        ),
        (
            'group_changed',
            claude_apex_group_id,
            jsonb_build_object(
                'reason', 'apex_monthly_card_group_upsert',
                'name', 'Claude Apex 月卡组',
                'monthly_limit_usd', 2898,
                'weekly_limit_usd', NULL,
                'rate_multiplier', 1.25
            )
        );
END $$;
