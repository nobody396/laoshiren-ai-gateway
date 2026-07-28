-- Affiliate V3 monthly catalog.
--
-- New groups are additive so existing subscriptions keep their original quota,
-- multiplier and expiry semantics.  Only newly generated V3 cards point at
-- these groups.

SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

DO $$
DECLARE
    plan RECORD;
    channel RECORD;
    source_group_id BIGINT;
    target_group_id BIGINT;
    target_group_name TEXT;
BEGIN
    FOR plan IN
        SELECT *
        FROM (VALUES
            ('Plus'::TEXT, 220::NUMERIC, 'Lite'::TEXT, 1200::INTEGER),
            ('Pro'::TEXT, 650::NUMERIC, 'Pro'::TEXT, 1210::INTEGER),
            ('Max'::TEXT, 1400::NUMERIC, 'Max'::TEXT, 1220::INTEGER)
        ) AS plans(plan_name, monthly_limit, source_plan_name, sort_order)
    LOOP
        FOR channel IN
            SELECT *
            FROM (VALUES
                ('GPT'::TEXT, 'openai'::TEXT, 0.50::NUMERIC, 0::INTEGER),
                ('Claude'::TEXT, 'anthropic'::TEXT, 2.40::NUMERIC, 100::INTEGER)
            ) AS channels(prefix, platform_name, rate_multiplier, sort_offset)
        LOOP
            SELECT id
            INTO source_group_id
            FROM groups
            WHERE deleted_at IS NULL
              AND name = channel.prefix || ' ' || plan.source_plan_name || ' 月卡组'
            ORDER BY id
            LIMIT 1;

            IF source_group_id IS NULL THEN
                SELECT id
                INTO source_group_id
                FROM groups
                WHERE deleted_at IS NULL
                  AND name = channel.prefix || ' Ultra 月卡组'
                ORDER BY id
                LIMIT 1;
            END IF;

            IF source_group_id IS NULL THEN
                RAISE EXCEPTION 'source monthly group missing for channel: %', channel.prefix;
            END IF;

            target_group_name := CASE
                WHEN plan.plan_name = 'Plus'
                    THEN channel.prefix || ' Plus 月卡组'
                ELSE channel.prefix || ' ' || plan.plan_name || ' V3 月卡组'
            END;

            SELECT id
            INTO target_group_id
            FROM groups
            WHERE deleted_at IS NULL
              AND name = target_group_name
            ORDER BY id
            LIMIT 1;

            IF target_group_id IS NULL THEN
                INSERT INTO groups (
                    name, description, rate_multiplier, is_exclusive, status,
                    platform, subscription_type,
                    daily_limit_usd, weekly_limit_usd, monthly_limit_usd,
                    default_validity_days,
                    image_price_1k, image_price_2k, image_price_4k,
                    gpt_image_call_price, claude_code_only,
                    fallback_group_id, fallback_group_id_on_invalid_request,
                    model_routing, model_routing_enabled, mcp_xml_inject,
                    supported_model_scopes, sort_order,
                    allow_messages_dispatch, require_oauth_only,
                    require_privacy_set, default_mapped_model,
                    messages_dispatch_model_config,
                    created_at, updated_at
                )
                SELECT
                    target_group_name,
                    '',
                    channel.rate_multiplier,
                    is_exclusive,
                    'active',
                    channel.platform_name,
                    'credit',
                    NULL,
                    NULL,
                    plan.monthly_limit,
                    31,
                    image_price_1k, image_price_2k, image_price_4k,
                    gpt_image_call_price, claude_code_only,
                    fallback_group_id, fallback_group_id_on_invalid_request,
                    model_routing, model_routing_enabled, mcp_xml_inject,
                    supported_model_scopes,
                    plan.sort_order + channel.sort_offset,
                    allow_messages_dispatch, require_oauth_only,
                    require_privacy_set, default_mapped_model,
                    messages_dispatch_model_config,
                    NOW(), NOW()
                FROM groups
                WHERE id = source_group_id
                RETURNING id INTO target_group_id;
            ELSE
                UPDATE groups
                SET description = '',
                    rate_multiplier = channel.rate_multiplier,
                    status = 'active',
                    platform = channel.platform_name,
                    subscription_type = 'credit',
                    daily_limit_usd = NULL,
                    weekly_limit_usd = NULL,
                    monthly_limit_usd = plan.monthly_limit,
                    default_validity_days = 31,
                    sort_order = plan.sort_order + channel.sort_offset,
                    updated_at = NOW()
                WHERE id = target_group_id;
            END IF;

            INSERT INTO account_groups (account_id, group_id, priority, created_at)
            SELECT account_id, target_group_id, priority, NOW()
            FROM account_groups
            WHERE group_id = source_group_id
            ON CONFLICT (account_id, group_id) DO UPDATE
            SET priority = EXCLUDED.priority;

            INSERT INTO scheduler_outbox (event_type, group_id, payload)
            VALUES (
                'group_changed',
                target_group_id,
                jsonb_build_object(
                    'reason', 'affiliate_v3_monthly_catalog_upsert',
                    'name', target_group_name,
                    'monthly_limit_usd', plan.monthly_limit,
                    'daily_limit_usd', NULL,
                    'weekly_limit_usd', NULL,
                    'rate_multiplier', channel.rate_multiplier,
                    'pricing_table_version', 'v3-2026-07-28'
                )
            );
        END LOOP;
    END LOOP;
END $$;
