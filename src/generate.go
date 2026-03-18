package main

//go:generate swagger-codegen generate --input-spec https://developers.strava.com/swagger/swagger.json --lang go --output strava --additional-properties packageName=strava
//go:generate go fmt ./strava
