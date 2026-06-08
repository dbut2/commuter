-- name: UpsertActivity :one
insert into activities (user_id, strava_id, status, applied_rules, activity_time)
values ($1, $2, $3, $4, $5)
on conflict (strava_id) do update
set status = excluded.status,
    applied_rules = excluded.applied_rules,
    activity_time = coalesce(excluded.activity_time, activities.activity_time),
    updated_at = now()
returning *;

-- name: GetActivity :one
select * from activities where id = $1 and user_id = $2;

-- name: ListFeed :many
select
    a.*,
    sd.data as strava_data,
    j.status as job_status,
    j.next_run as job_next_run,
    j.expires_at as job_expires_at
from activities a
left join provider_data sd on sd.activity_id = a.id and sd.provider = 'strava'
left join jobs j on j.activity_id = a.id
where a.user_id = $1
order by a.activity_time desc nulls last, a.created_at desc;

-- name: SetActivityStatus :exec
update activities set status = $2, updated_at = now() where id = $1;
