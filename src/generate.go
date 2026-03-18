package main

//go:generate docker run --name swagger-gen swaggerapi/swagger-codegen-cli generate -i https://developers.strava.com/swagger/swagger.json -l go -o /strava -D packageName=strava --type-mappings LatLng=[2]float64
//go:generate mkdir -p ./strava
//go:generate sh -c "rm -rf ./strava/*"
//go:generate docker cp swagger-gen:/strava/. ./strava
//go:generate docker rm swagger-gen
//go:generate sh -c "find ./strava -type f ! -name '*.go' -delete"
//go:generate sh -c "find ./strava -type d -empty -delete"
//go:generate go fmt ./strava
