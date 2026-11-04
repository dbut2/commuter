package client

//go:generate curl -fsSLo strava/swagger.json https://developers.strava.com/swagger/swagger.json
//go:generate go tool openapi swagger upgrade strava/swagger.json strava/strava.yaml
//go:generate rm strava/swagger.json
//go:generate go tool openapi spec bundle -w strava/strava.yaml
//go:generate go tool oapi-codegen -config strava/oapi.yaml strava/strava.yaml
