-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

-- 部门
CREATE TABLE departments (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- 内部用户（HR / 部门负责人 / 管理员）
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL UNIQUE,
    name          text NOT NULL,
    password_hash text NOT NULL,
    role          text NOT NULL CHECK (role IN ('admin', 'hr', 'hiring_manager')),
    department_id uuid REFERENCES departments(id),
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- 岗位
CREATE TABLE job_postings (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    title         text NOT NULL,
    department_id uuid REFERENCES departments(id),
    owner_id      uuid REFERENCES users(id),
    approver_id   uuid REFERENCES users(id),
    status        text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'pending', 'open', 'closed')),
    headcount     int NOT NULL DEFAULT 1,
    salary_min    int,
    salary_max    int,
    location      text,
    job_type      text NOT NULL DEFAULT 'full_time' CHECK (job_type IN ('full_time', 'intern')),
    description   text,
    requirements  jsonb NOT NULL,
    embedding     vector(1024),
    published_at  timestamptz,
    closed_at     timestamptz,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_job_postings_status_published ON job_postings (status, published_at DESC);
CREATE INDEX idx_job_postings_department ON job_postings (department_id);

-- 外部求职者
CREATE TABLE candidates (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email      text NOT NULL UNIQUE,
    phone      text,
    name       text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- 投递记录
CREATE TABLE applications (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    candidate_id     uuid NOT NULL REFERENCES candidates(id),
    job_id           uuid NOT NULL REFERENCES job_postings(id),
    stage            text NOT NULL DEFAULT 'new' CHECK (stage IN ('new', 'screening', 'interview', 'offer', 'hired', 'rejected')),
    source           text NOT NULL DEFAULT '',
    resume_file_key  text,
    resume_text      text,
    parsed_resume    jsonb,
    match_score      numeric(5,1),
    match_detail     jsonb,
    hard_pass        boolean NOT NULL DEFAULT false,
    parse_failed     boolean NOT NULL DEFAULT false,
    resume_embedding vector(1024),
    submitted_at     timestamptz NOT NULL DEFAULT now(),
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (candidate_id, job_id)
);

CREATE INDEX idx_applications_job_id ON applications (job_id);
CREATE INDEX idx_applications_job_stage ON applications (job_id, stage);

-- 审计日志
CREATE TABLE audit_logs (
    id          bigserial PRIMARY KEY,
    actor_id    uuid,
    action      text NOT NULL,
    entity_type text NOT NULL,
    entity_id   text,
    detail      jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_audit_logs_entity ON audit_logs (entity_type, entity_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS candidates;
DROP TABLE IF EXISTS job_postings;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS departments;
-- +goose StatementEnd
