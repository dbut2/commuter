package main

//go:generate sh -c "rm -rf ./strava && mkdir -p ./strava"
//go:generate go tool swagger generate client -f https://developers.strava.com/swagger/swagger.json -t ./strava --skip-validation
//go:generate go mod tidy
