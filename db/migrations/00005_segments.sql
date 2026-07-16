-- +goose Up
create table segments (
    user_id    uuid        not null references users (id) on delete cascade,
    name       text        not null,
    segment_id bigint      not null,
    created_at timestamptz not null default now(),
    updated_at timestamptz not null default now(),
    primary key (user_id, name)
);

-- +goose Down
drop table if exists segments;
