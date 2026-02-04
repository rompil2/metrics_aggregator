// Package migrations contains embedded SQL migration files for migrating the database schema.
// path: internal/migrations
package migrations

import (
	"embed"
	"path"
)

// Migrations contains embedded SQL migration files used by the database migration system.
// It is populated at build time via the //go:embed directive and should not be modified at runtime.
var Migrations embed.FS

// GetMigrationFiles reads all embedded SQL migration files and returns their contents as a slice of strings.
// The order of migrations is not guaranteed and should be handled by the migration framework (e.g., goose).
// This function is typically used during application startup to initialize the database schema.
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
