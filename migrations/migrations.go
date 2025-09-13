package migrations

import (
	"embed"
	"path"
)

//go:embed  *.sql
var Migrations embed.FS

func GetMigrationFiles() ([]string, error) {
	dirEntries, err := Migrations.ReadDir(".")
	if err != nil {
		return nil, err
	}

	migrationContents := make([]string, 0, len(dirEntries))

	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			migrationContent, err := Migrations.ReadFile(path.Join(".", dirEntry.Name()))
			if err != nil {
				return nil, err
			}

			migrationContents = append(migrationContents, string(migrationContent))
		}
	}

	return migrationContents, nil
}
