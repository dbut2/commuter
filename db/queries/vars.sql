-- name: ListVars :many
select * from vars where user_id = $1 order by name;

-- name: UpsertVar :exec
insert into vars (user_id, name, type, value)
values ($1, $2, $3, $4)
on conflict (user_id, name) do update
set type = excluded.type, value = excluded.value, updated_at = now();

-- name: DeleteVar :exec
delete from vars where user_id = $1 and name = $2;
