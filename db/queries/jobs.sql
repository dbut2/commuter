-- name: UpsertJob :exec
insert into jobs (activity_id, status, next_run, expires_at)
values ($1, 'queued', $2, $3)
on conflict (activity_id) do update
set status = 'queued', next_run = excluded.next_run, expires_at = excluded.expires_at, updated_at = now();

-- name: GetJob :one
select * from jobs where activity_id = $1;

-- name: CompleteJob :exec
update jobs set status = $2, last_error = $3, updated_at = now() where activity_id = $1;

-- name: ClaimJobs :many
select j.activity_id, a.user_id, a.strava_id, j.expires_at
from jobs j
join activities a on a.id = j.activity_id
where j.status = 'queued' and j.next_run <= now()
order by j.next_run
limit $1;
