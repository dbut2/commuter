-- name: ListSegments :many
select * from segments where user_id = $1 order by name;

-- name: UpsertSegment :exec
insert into segments (user_id, name, segment_id)
values ($1, $2, $3)
on conflict (user_id, name) do update
set segment_id = excluded.segment_id, updated_at = now();

-- name: DeleteSegment :exec
delete from segments where user_id = $1 and name = $2;
