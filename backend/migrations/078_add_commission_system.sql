-- 078_add_commission_system.sql
-- 新增分佣/邀请系统相关表结构
-- 包含：users 表扩展字段、commission_records 新表
-- 所有操作均为幂等（IF NOT EXISTS）

-- =============================================
-- 1. 扩展 users 表：邀请/分佣系统字段
-- =============================================

-- 用户自己的邀请码，全局唯一，注册时自动生成，格式 [A-Z0-9]{8}
ALTER TABLE users ADD COLUMN IF NOT EXISTS invite_code VARCHAR(20);

-- 邀请人 user_id（直接邀请该用户注册的人）
ALTER TABLE users ADD COLUMN IF NOT EXISTS inviter_id BIGINT;

-- 归属代理商 user_id（邀请人是 agent 角色时填写，用于永久消耗分佣快速查询）
ALTER TABLE users ADD COLUMN IF NOT EXISTS agent_id BIGINT;

-- 是否已完成首充，防止重复触发首充奖励（幂等标志）
ALTER TABLE users ADD COLUMN IF NOT EXISTS first_recharged BOOLEAN NOT NULL DEFAULT false;

-- invite_code 唯一索引（仅对非 NULL 值生效，支持多用户未设置邀请码的情况）
CREATE UNIQUE INDEX IF NOT EXISTS users_invite_code_unique
    ON users(invite_code)
    WHERE invite_code IS NOT NULL;

-- inviter_id 索引（查询某人邀请了哪些用户）
CREATE INDEX IF NOT EXISTS users_inviter_id_idx ON users(inviter_id);

-- agent_id 索引（快速查询代理商旗下所有用户）
CREATE INDEX IF NOT EXISTS users_agent_id_idx ON users(agent_id);

-- =============================================
-- 2. 新建 commission_records 表
-- =============================================
-- 记录每一笔分佣/奖励流水，仅追加，不可删除
CREATE TABLE IF NOT EXISTS commission_records (
    id             BIGSERIAL PRIMARY KEY,

    -- 获得分佣/奖励的用户（代理商或普通邀请人）
    beneficiary_id BIGINT NOT NULL,

    -- 触发分佣的用户（消费方/充值方）
    user_id        BIGINT NOT NULL,

    -- 分佣/奖励金额（已入账到 beneficiary 余额）
    amount         DECIMAL(20,8) NOT NULL,

    -- 原始触发金额（API 消费额或充值额）
    source_amount  DECIMAL(20,8) NOT NULL,

    -- 分佣类型：
    --   consumption_commission        — 代理商消耗 10% 分佣
    --   first_recharge_agent_bonus    — 首充时代理商获得 10%
    --   first_recharge_invitee_bonus  — 首充时被邀请用户获得 10%
    --   first_recharge_referral_bonus — 普通用户邀请人获得 5%
    type           VARCHAR(50) NOT NULL,

    -- 关联来源记录 ID（usage_log.id 或 redeem_code.id），可为空
    source_id      BIGINT,

    -- 备注
    note           TEXT,

    -- 创建时间（不可变）
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 索引
CREATE INDEX IF NOT EXISTS commission_beneficiary_idx
    ON commission_records(beneficiary_id);

CREATE INDEX IF NOT EXISTS commission_user_idx
    ON commission_records(user_id);

CREATE INDEX IF NOT EXISTS commission_type_idx
    ON commission_records(type);

CREATE INDEX IF NOT EXISTS commission_created_at_idx
    ON commission_records(created_at);

-- 代理商按日期查询分佣的复合索引（最常用查询）
CREATE INDEX IF NOT EXISTS commission_beneficiary_created_idx
    ON commission_records(beneficiary_id, created_at);

-- 代理商按类型汇总的复合索引
CREATE INDEX IF NOT EXISTS commission_beneficiary_type_idx
    ON commission_records(beneficiary_id, type);
