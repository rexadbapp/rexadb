package main

import (
	_ "github.com/rexadb/rexadb/pkg/provider/mariadb"
	_ "github.com/rexadb/rexadb/pkg/provider/mongodb"
	_ "github.com/rexadb/rexadb/pkg/provider/mysql"
	_ "github.com/rexadb/rexadb/pkg/provider/postgres"
	_ "github.com/rexadb/rexadb/pkg/provider/redis"
	_ "github.com/rexadb/rexadb/pkg/provider/sqlite"

	"github.com/rexadb/rexadb/cmd"
)

func main() {
	cmd.Execute()
}
