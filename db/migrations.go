package db

import (
	"embed"
	"io/fs"
	"log"
)

//go:embed migrations/*.sql
var migrations embed.FS

var Migrations fs.FS

func init() {
	var err error
	Migrations, err = fs.Sub(migrations, "migrations")
	if err != nil {
		log.Fatal(err)
	}
}
