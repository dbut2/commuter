-- name: EnsureUser :one
insert into users (strava_athlete_id, name)
values ($1, $2)
on conflict (strava_athlete_id)
do update set name = excluded.name, updated_at = now()
returning *;

-- name: GetUser :one
select * from users where id = $1;

-- name: GetParkrunID :one
select coalesce(parkrun_id, '') from users where id = $1;

-- name: SetParkrunID :exec
update users set parkrun_id = $2, updated_at = now() where id = $1;

-- name: DeleteUser :exec
delete from users where id = $1;

-- name: SetStravaToken :exec
insert into strava_credentials (user_id, token)
values ($1, $2)
on conflict (user_id) do update set token = excluded.token, updated_at = now();

-- name: GetStravaToken :one
select token from strava_credentials where user_id = $1;

-- name: UserIDByAthlete :one
select id from users where strava_athlete_id = $1;
