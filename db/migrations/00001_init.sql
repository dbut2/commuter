-- +goose Up
create table users (
    id                uuid        primary key default gen_random_uuid(),
    strava_athlete_id bigint      not null unique,
    name              text        not null,
    parkrun_id        text,
    created_at        timestamptz not null default now(),
    updated_at        timestamptz not null default now()
);

create table strava_credentials (
    user_id    uuid        primary key references users (id) on delete cascade,
    token      jsonb       not null,
    updated_at timestamptz not null default now()
);

create table rules (
    id         uuid        primary key default gen_random_uuid(),
    user_id    uuid        not null references users (id) on delete cascade,
    name       text        not null,
    enabled    boolean     not null default true,
    priority   integer     not null default 0,
    conditions jsonb       not null default '[]'::jsonb,
    actions    jsonb       not null default '[]'::jsonb,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now()
);

create table vars (
    user_id    uuid        not null references users (id) on delete cascade,
    name       text        not null,
    type       text        not null,
    value      text        not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (user_id, name)
);

create table activities (
    id            uuid        primary key default gen_random_uuid(),
    user_id       uuid        not null references users (id) on delete cascade,
    strava_id     bigint      not null unique,
    status        text        not null default 'unprocessed'
        check (status in ('unprocessed', 'pending', 'processed')),
    applied_rules jsonb       not null default '[]'::jsonb,
    run_log       jsonb       not null default '[]'::jsonb,
    activity_time timestamptz,
    created_at    timestamptz not null default now(),
    updated_at    timestamptz not null default now()
);

create table provider_data (
    activity_id uuid        not null references activities (id) on delete cascade,
    provider    text        not null,
    data        jsonb       not null default '{}'::jsonb,
    found       boolean     not null default false,
    fetched_at  timestamptz not null default now(),
    primary key (activity_id, provider)
);

create table jobs (
    id          uuid        primary key default gen_random_uuid(),
    activity_id uuid        not null unique references activities (id) on delete cascade,
    status      text        not null default 'queued'
        check (status in ('queued', 'waiting', 'completed', 'failed')),
    next_run    timestamptz not null default now(),
    expires_at  timestamptz not null,
    last_error  text,
    created_at  timestamptz not null default now(),
    updated_at  timestamptz not null default now()
);
create index jobs_claimable_idx on jobs (next_run) where status = 'queued';

-- +goose Down
drop table if exists jobs;
drop table if exists provider_data;
drop table if exists activities;
drop table if exists vars;
drop table if exists rules;
drop table if exists strava_credentials;
drop table if exists users;
