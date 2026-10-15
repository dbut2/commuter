#!/bin/sh
set -eu
cd "$(dirname "$0")"

version=2.4.41
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

curl -fsSLo "$tmp/codegen.jar" "https://repo1.maven.org/maven2/io/swagger/swagger-codegen-cli/$version/swagger-codegen-cli-$version.jar"
java -jar "$tmp/codegen.jar" generate \
  -i https://developers.strava.com/swagger/swagger.json \
  -l go \
  -o "$tmp/out" \
  -D packageName=strava \
  --type-mappings 'LatLng=[2]float64'

find . -maxdepth 1 -name '*.go' -delete
cp "$tmp"/out/*.go .
gofmt -w .
