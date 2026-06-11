-- name: ListRules :many
select * from rules
where user_id = $1
order by priority, created_at;

-- name: GetRule :one
select * from rules where id = $1 and user_id = $2;

-- name: CountRules :one
select count(*) from rules where user_id = $1;

-- name: CreateRule :one
insert into rules (user_id, name, enabled, priority, conditions, actions)
values (
    $1,
    $2,
    true,
    coalesce((select max(priority) + 1 from rules where user_id = $1), 0),
    $3,
    $4
)
returning *;

-- name: UpdateRule :one
update rules
set name = $3, conditions = $4, actions = $5, enabled = $6, updated_at = now()
where id = $1 and user_id = $2
returning *;

-- name: ToggleRule :exec
update rules
set enabled = not enabled, updated_at = now()
where id = $1 and user_id = $2;

-- name: DeleteRule :exec
delete from rules where id = $1 and user_id = $2;

-- name: SetRulePriority :exec
update rules set priority = $3, updated_at = now()
where id = $1 and user_id = $2;
