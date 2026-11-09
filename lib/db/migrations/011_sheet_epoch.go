package migrations

import "database/sql"

// migration011SheetEpoch adds sheet.epoch, which counts how often a sheet's
// workbook was replaced (xlsx import). A replacement restarts the op
// numbering at 0, so the head alone cannot tell another instance that its
// cached copy belongs to the previous workbook.
func migration011SheetEpoch() Migration {
	return Migration{
		Version:     11,
		Description: "Add sheet.epoch to detect replaced workbooks",
		Up: func(db *sql.DB, dialect Dialect) error {
			_, err := db.Exec(`ALTER TABLE sheet ADD COLUMN epoch INTEGER NOT NULL DEFAULT 0`)
			return err
		},
	}
}
