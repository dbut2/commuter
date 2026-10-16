package web

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed static
var static embed.FS

var Static fs.FS

func init() {
	var err error
	Static, err = fs.Sub(static, "static")
	if err != nil {
		log.Fatal(err)
	}
}
