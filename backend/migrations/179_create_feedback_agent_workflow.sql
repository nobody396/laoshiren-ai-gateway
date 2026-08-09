SET lock_timeout = '5s';
SET statement_timeout = '60s';

ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS request_id VARCHAR(128) NOT NULL DEFAULT '';
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS triage_status VARCHAR(24) NOT NULL DEFAULT 'unreviewed';
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS triage_priority VARCHAR(4) NOT NULL DEFAULT '';
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS triage_summary TEXT NOT NULL DEFAULT '';
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS triage_confidence DOUBLE PRECISION;
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS repair_difficulty VARCHAR(16) NOT NULL DEFAULT 'unknown';
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS repair_recommendation TEXT NOT NULL DEFAULT '';
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS owner_decision VARCHAR(16) NOT NULL DEFAULT 'pending';
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS fix_status VARCHAR(24) NOT NULL DEFAULT 'not_started';
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS duplicate_of_id BIGINT;
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS resolved_version VARCHAR(64) NOT NULL DEFAULT '';
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS accepted_at TIMESTAMPTZ;
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMPTZ;
ALTER TABLE feedbacks ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS feedbacks_triage_status ON feedbacks (triage_status);
CREATE INDEX IF NOT EXISTS feedbacks_triage_priority ON feedbacks (triage_priority);
CREATE INDEX IF NOT EXISTS feedbacks_owner_decision ON feedbacks (owner_decision);
CREATE INDEX IF NOT EXISTS feedbacks_fix_status ON feedbacks (fix_status);
CREATE INDEX IF NOT EXISTS feedbacks_request_id ON feedbacks (request_id);

CREATE TABLE IF NOT EXISTS feedback_events (
    id BIGSERIAL PRIMARY KEY,
    feedback_id BIGINT NOT NULL REFERENCES feedbacks(id) ON DELETE CASCADE,
    event_type VARCHAR(40) NOT NULL,
    actor_type VARCHAR(16) NOT NULL,
    actor_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    summary TEXT NOT NULL,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS feedback_events_feedback_id_created_at ON feedback_events (feedback_id, created_at);
CREATE INDEX IF NOT EXISTS feedback_events_event_type ON feedback_events (event_type);

CREATE TABLE IF NOT EXISTS feedback_rewards (
    id BIGSERIAL PRIMARY KEY,
    feedback_id BIGINT NOT NULL REFERENCES feedbacks(id) ON DELETE RESTRICT,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    amount DECIMAL(20,8) NOT NULL,
    reason VARCHAR(64) NOT NULL,
    batch_id VARCHAR(64) NOT NULL,
    operator_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    account_change_record_id BIGINT REFERENCES account_change_records(id) ON DELETE SET NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT feedback_rewards_feedback_id_key UNIQUE (feedback_id)
);
CREATE INDEX IF NOT EXISTS feedback_rewards_user_id_created_at ON feedback_rewards (user_id, created_at);
CREATE INDEX IF NOT EXISTS feedback_rewards_batch_id ON feedback_rewards (batch_id);

CREATE TABLE IF NOT EXISTS user_notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    feedback_id BIGINT REFERENCES feedbacks(id) ON DELETE CASCADE,
    type VARCHAR(40) NOT NULL,
    title VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    action_url VARCHAR(500) NOT NULL DEFAULT '',
    dedupe_key VARCHAR(128),
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT user_notifications_dedupe_key_key UNIQUE (dedupe_key)
);
CREATE INDEX IF NOT EXISTS user_notifications_user_id_read_at_created_at ON user_notifications (user_id, read_at, created_at);
CREATE INDEX IF NOT EXISTS user_notifications_feedback_id ON user_notifications (feedback_id);

INSERT INTO feedback_events (feedback_id, event_type, actor_type, summary, metadata, created_at)
SELECT f.id, 'submitted', 'user', '用户已提交反馈', '{}'::jsonb, f.created_at
FROM feedbacks f
WHERE NOT EXISTS (
    SELECT 1 FROM feedback_events e WHERE e.feedback_id = f.id AND e.event_type = 'submitted'
);

RESET statement_timeout;
RESET lock_timeout;
