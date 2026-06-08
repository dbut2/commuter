-- name: UpsertProviderData :exec
insert into provider_data (activity_id, provider, data, found)
values ($1, $2, $3, $4)
on conflict (activity_id, provider) do update
set data = excluded.data, found = excluded.found, fetched_at = now();

-- name: GetProviderData :one
select data, found from provider_data where activity_id = $1 and provider = $2;
