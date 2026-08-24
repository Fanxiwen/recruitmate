-- +goose Up
-- +goose StatementBegin
-- 面试实体化：每轮面试有准确时间、评价与结论（HR 面 / 部门负责人面）
CREATE TABLE interviews (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id uuid NOT NULL REFERENCES applications(id) ON DELETE CASCADE,
    round          text NOT NULL CHECK (round IN ('hr', 'manager')),
    scheduled_at   timestamptz,
    status         text NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'completed', 'cancelled')),
    result         text NOT NULL DEFAULT 'pending' CHECK (result IN ('pending', 'pass', 'fail')),
    feedback       text NOT NULL DEFAULT '',
    reviewed_by    uuid REFERENCES users(id),
    reviewed_at    timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now(),
    UNIQUE (application_id, round)
);
CREATE INDEX idx_interviews_application ON interviews (application_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS interviews;
-- +goose StatementEnd
