// Package migrations embute os arquivos .sql para que binario e testes
// apliquem exatamente o mesmo schema.
package migrations

import (
	"database/sql"
	"embed"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var FS embed.FS

// Aplicar roda as migrations pendentes contra url.
func Aplicar(url string) error {
	goose.SetBaseFS(FS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	db, err := sql.Open("pgx/v5", url)
	if err != nil {
		return err
	}
	defer db.Close()
	return goose.Up(db, ".")
}
