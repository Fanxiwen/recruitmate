-- +goose Up
-- +goose StatementBegin
-- 面试加一轮部门负责人面；薪资由部门负责人审批时确定
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_stage_check;
ALTER TABLE applications ADD CONSTRAINT applications_stage_check
    CHECK (stage IN ('new', 'screening', 'interview', 'manager_interview',
                     'offer_pending', 'offered', 'hired', 'rejected'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE applications DROP CONSTRAINT IF EXISTS applications_stage_check;
ALTER TABLE applications ADD CONSTRAINT applications_stage_check
    CHECK (stage IN ('new', 'screening', 'interview', 'offer_pending', 'offered', 'hired', 'rejected'));
-- +goose StatementEnd
