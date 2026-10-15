-- name: UpsertActivity :one
insert into activities (user_id, strava_id, status, applied_rules, activity_time, run_log)
values ($1, $2, $3, $4, $5, $6)
on conflict (strava_id) do update
set status = excluded.status,
    applied_rules = excluded.applied_rules,
    activity_time = coalesce(excluded.activity_time, activities.activity_time),
    run_log = case when excluded.run_log = '[]'::jsonb then activities.run_log else excluded.run_log end,
    updated_at = now()
returning *;

-- name: ExistingStravaIDs :many
select strava_id from activities where user_id = $1 and strava_id = any(sqlc.arg(strava_ids)::bigint[]);

-- name: GetActivity :one
select * from activities where id = $1 and user_id = $2;

-- name: ActivityIDByStrava :one
select id from activities where user_id = $1 and strava_id = $2;

-- name: SetActivityTime :exec
update activities set activity_time = $2, updated_at = now() where id = $1;

-- name: ListFeed :many
select
    a.*,
    sd.data as strava_data
from activities a
left join provider_data sd on sd.activity_id = a.id and sd.provider = 'strava'
where a.user_id = $1
order by coalesce(a.activity_time, a.created_at) desc;

-- name: LatestActivityJobs :many
select distinct on ((args->>'strava_id')::bigint)
    (args->>'strava_id')::bigint as strava_id,
    state,
    scheduled_at,
    (args->>'expires_at')::timestamptz as expires_at,
    coalesce(errors[array_length(errors, 1)]->>'error', '')::text as last_error
from river_job
where kind = 'process_activity' and (args->>'strava_id')::bigint = any(sqlc.arg(strava_ids)::bigint[])
order by (args->>'strava_id')::bigint, id desc;

-- name: SetActivityStatus :exec
update activities set status = $2, updated_at = now() where id = $1;
