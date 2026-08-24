-- +goose Up
-- +goose StatementBegin
-- 招聘流程 OA 化：Offer 审批 + 流转时间线 + 淘汰原因/面试评价

ALTER TABLE applications
    ADD COLUMN reject_reason text NOT NULL DEFAULT '',
    ADD COLUMN interview_feedback text NOT NULL DEFAULT '';

-- 校验约束更新：允许 offer_pending 阶段
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_stage_check;
ALTER TABLE applications ADD CONSTRAINT applications_stage_check
    CHECK (stage IN ('new', 'screening', 'interview', 'offer_pending', 'offered', 'hired', 'rejected'));

-- Offer 审批表
CREATE TABLE offers (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    salary         text NOT NULL DEFAULT '',
    join_date      text NOT NULL DEFAULT '',
    note           text NOT NULL DEFAULT '',
    status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
    requested_by   uuid NOT NULL REFERENCES users(id),
    decided_by     uuid REFERENCES users(id),
    requested_at   timestamptz NOT NULL DEFAULT now(),
    decided_at     timestamptz
);
CREATE INDEX idx_offers_application ON offers (application_id);

-- 流转时间线
CREATE TABLE application_events (
    id             bigserial PRIMARY KEY,
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    from_stage     text NOT NULL DEFAULT '',
    to_stage       text NOT NULL,
    action         text NOT NULL DEFAULT 'stage_change',
    actor_id       uuid REFERENCES users(id),
    actor_name     text NOT NULL DEFAULT '',
    reason         text NOT NULL DEFAULT '',
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_application_events_app ON application_events (application_id, created_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS application_events;
DROP TABLE IF EXISTS offers;
ALTER TABLE applications DROP COLUMN IF EXISTS reject_reason;
ALTER TABLE applications DROP COLUMN IF EXISTS interview_feedback;
-- +goose StatementEnd
