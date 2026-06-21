# Kollaborative Tabelle — Plan 2b: Persistenz

> **For agentic workers:** REQUIRED SUB-SKILL: superpowers:executing-plans. Steps use `- [ ]`.

**Goal:** Den `lib/sheet`-Kern persistieren: Workbook-Snapshot + Op-Log über die `DataStore`-Abstraktion (SQLite/Postgres/MySQL/Memory), inkl. Migration 008. Headless gegen In-Memory-SQLite und Memory-Store testbar.

**Architecture:** Ein `sheet`-Header-Row pro Dokument (keyed by pad id) hält `head` und den kompletten Workbook-Snapshot als JSON (analog dazu, wie das `pad`-Row die ganze AText hält). Eine `sheet_op`-Tabelle ist das Op-Log (analog `padRev`), für Reconnect/History. Beide referenzieren `pad(id)` mit `ON DELETE CASCADE`. Die Serialisierung (Workbook ↔ JSON-Snapshot) gehört ins Modell `lib/sheet`.

**Tech Stack:** Go, squirrel (`?`/`$n` je Dialekt), `encoding/json`, in-memory SQLite für Tests.

**Bezug:** Spec §3 (Persistenz) + Plan 2 (`lib/sheet`). Abweichung von der Spec-Skizze: statt einer sparse `sheet_cell`-Tabelle wird der Zellzustand als JSON-Blob im `sheet`-Row gehalten (konsistent mit dem Pad-AText-Blob-Muster, vermeidet teures Re-Materialisieren bei Struktur-Ops). Sparse-Tabelle bleibt eine spätere Optimierung.

---

## Datei-Struktur

| Datei | Aktion |
|---|---|
| `lib/sheet/snapshot.go` (+test) | NEU: `WorkbookSnapshot`, `Workbook.Snapshot()`, `WorkbookFromSnapshot` |
| `lib/models/db/SheetDB.go` | NEU: `SheetDB`, `SheetOpDB` |
| `lib/db/migrations/008_sheets.go` | NEU: Tabellen `sheet`, `sheet_op` (3 Dialekte) |
| `lib/db/migrations/001_initial_schema.go` | migration008 registrieren |
| `lib/db/DataStore.go` | `SheetMethods` + in `DataStore` einbetten |
| `lib/db/MemoryDataStore.go` | Felder + Impl + Init |
| `lib/db/SQLiteDB.go` | Impl |
| `lib/db/PostgresDB.go` | Impl |
| `lib/db/MySQLDB.go` | Impl |

---

## Task 1: Snapshot-Serialisierung in `lib/sheet`

**Files:** Create `lib/sheet/snapshot.go`, `lib/sheet/snapshot_test.go`

- [ ] **Step 1: Failing-Test** — `lib/sheet/snapshot_test.go`:
```go
package sheet

import (
	"encoding/json"
	"testing"
)

func TestWorkbookSnapshotRoundTrip(t *testing.T) {
	w := NewWorkbook()
	s := w.AddSheet("s1", "Sheet1")
	sid := w.Styles.Put(Style{Props: map[string]string{"bold": "1"}})
	s.SetCell(CellRef{1, 2}, Cell{Raw: "=A1+1", StyleId: sid})
	s.SetCell(CellRef{0, 0}, Cell{Raw: "hi"})

	b, err := json.Marshal(w.Snapshot())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var snap WorkbookSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	got := WorkbookFromSnapshot(snap)

	if got.SheetByID("s1").GetCell(CellRef{1, 2}).Raw != "=A1+1" {
		t.Fatal("cell raw lost in round-trip")
	}
	// style pool index must be rebuilt so dedup still works after load
	if got.Styles.Put(Style{Props: map[string]string{"bold": "1"}}) != sid {
		t.Fatal("style pool dedup index not rebuilt after load")
	}
}
```

- [ ] **Step 2: Run — fail.** `go test ./lib/sheet/ -run TestWorkbookSnapshot -v`

- [ ] **Step 3: Implement** — `lib/sheet/snapshot.go`:
```go
package sheet

import "sort"

// CellSnapshot is the serializable form of one populated cell (map keys can't
// be JSON-encoded, so cells become a flat slice).
type CellSnapshot struct {
	Row       int    `json:"row"`
	Col       int    `json:"col"`
	Raw       string `json:"raw"`
	Value     string `json:"value,omitempty"`
	ValueType string `json:"valueType,omitempty"`
	StyleId   int    `json:"styleId,omitempty"`
}

type SheetSnapshot struct {
	Id    string         `json:"id"`
	Name  string         `json:"name"`
	Cells []CellSnapshot `json:"cells"`
}

// WorkbookSnapshot is the JSON-serializable form of a Workbook for persistence.
type WorkbookSnapshot struct {
	Sheets []SheetSnapshot `json:"sheets"`
	Styles *StylePool      `json:"styles"`
}

// Snapshot converts the workbook to its serializable form. Cells are emitted in
// (row, col) order for deterministic output.
func (w *Workbook) Snapshot() WorkbookSnapshot {
	out := WorkbookSnapshot{Sheets: make([]SheetSnapshot, len(w.Sheets)), Styles: w.Styles}
	for i, s := range w.Sheets {
		cells := make([]CellSnapshot, 0, len(s.Cells))
		for ref, c := range s.Cells {
			cells = append(cells, CellSnapshot{ref.Row, ref.Col, c.Raw, c.Value, c.ValueType, c.StyleId})
		}
		sort.Slice(cells, func(a, b int) bool {
			if cells[a].Row != cells[b].Row {
				return cells[a].Row < cells[b].Row
			}
			return cells[a].Col < cells[b].Col
		})
		out.Sheets[i] = SheetSnapshot{Id: s.Id, Name: s.Name, Cells: cells}
	}
	return out
}

// WorkbookFromSnapshot rebuilds a Workbook (and its StylePool dedup index) from
// a deserialized snapshot.
func WorkbookFromSnapshot(snap WorkbookSnapshot) *Workbook {
	w := &Workbook{Sheets: make([]*Sheet, len(snap.Sheets))}
	if snap.Styles == nil {
		w.Styles = NewStylePool()
	} else {
		w.Styles = snap.Styles
		if w.Styles.IdToStyle == nil {
			w.Styles.IdToStyle = map[int]Style{}
		}
		if w.Styles.NextId == 0 {
			w.Styles.NextId = 1
		}
		w.Styles.rebuildIndex()
	}
	for i, ss := range snap.Sheets {
		sh := NewSheet(ss.Id, ss.Name)
		for _, c := range ss.Cells {
			sh.Cells[CellRef{c.Row, c.Col}] = Cell{Raw: c.Raw, Value: c.Value, ValueType: c.ValueType, StyleId: c.StyleId}
		}
		w.Sheets[i] = sh
	}
	return w
}
```

- [ ] **Step 4: Run — pass.** `go test ./lib/sheet/ -run TestWorkbookSnapshot -v`
- [ ] **Step 5: Commit.** `git add lib/sheet/snapshot.go lib/sheet/snapshot_test.go && git commit -m "feat(sheet): add workbook snapshot serialization"`

---

## Task 2: DB-Modelle + Migration 008

**Files:** Create `lib/models/db/SheetDB.go`, `lib/db/migrations/008_sheets.go`; modify `lib/db/migrations/001_initial_schema.go`

- [ ] **Step 1: Modelle** — `lib/models/db/SheetDB.go`:
```go
package db

import "time"

// SheetDB is the persisted header of a spreadsheet document (keyed by pad id).
// Snapshot is a marshaled sheet.WorkbookSnapshot.
type SheetDB struct {
	ID        string
	Head      int
	Snapshot  string
	CreatedAt time.Time
	UpdatedAt *time.Time
}

// SheetOpDB is one persisted operation in a sheet document's op-log.
type SheetOpDB struct {
	PadId     string
	Rev       int
	Op        string
	AuthorId  *string
	Timestamp int64
}
```

- [ ] **Step 2: Migration** — `lib/db/migrations/008_sheets.go`:
```go
package migrations

import "database/sql"

func migration008Sheets() Migration {
	return Migration{
		Version:     8,
		Description: "Create sheet and sheet_op tables",
		Up: func(db *sql.DB, dialect Dialect) error {
			var stmts []string
			switch dialect {
			case DialectMySQL:
				stmts = []string{
					`CREATE TABLE IF NOT EXISTS sheet (
						id VARCHAR(255) PRIMARY KEY,
						head INT NOT NULL DEFAULT 0,
						snapshot LONGTEXT,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
					`CREATE TABLE IF NOT EXISTS sheet_op (
						id VARCHAR(255) NOT NULL,
						rev INT NOT NULL,
						op LONGTEXT,
						author_id VARCHAR(255),
						timestamp BIGINT,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						PRIMARY KEY (id, rev),
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
				}
			case DialectPostgres:
				stmts = []string{
					`CREATE TABLE IF NOT EXISTS sheet (
						id TEXT PRIMARY KEY,
						head INTEGER NOT NULL DEFAULT 0,
						snapshot TEXT,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
					`CREATE TABLE IF NOT EXISTS sheet_op (
						id TEXT NOT NULL,
						rev INTEGER NOT NULL,
						op TEXT,
						author_id TEXT,
						timestamp BIGINT,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						PRIMARY KEY (id, rev),
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
				}
			default: // SQLite
				stmts = []string{
					`CREATE TABLE IF NOT EXISTS sheet (
						id TEXT PRIMARY KEY,
						head INTEGER NOT NULL DEFAULT 0,
						snapshot TEXT,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
					`CREATE TABLE IF NOT EXISTS sheet_op (
						id TEXT NOT NULL,
						rev INTEGER NOT NULL,
						op TEXT,
						author_id TEXT,
						timestamp INTEGER,
						created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
						PRIMARY KEY (id, rev),
						FOREIGN KEY (id) REFERENCES pad(id) ON DELETE CASCADE
					)`,
				}
			}
			for _, q := range stmts {
				if _, err := db.Exec(q); err != nil {
					return err
				}
			}
			return nil
		},
	}
}
```

- [ ] **Step 3: Register** in `001_initial_schema.go` GetMigrations after `migration007DocumentType(),`:
```go
		migration007DocumentType(),
		migration008Sheets(),
	}
```

- [ ] **Step 4: Build.** `go build ./lib/db/...`
- [ ] **Step 5: Commit.** `git add lib/models/db/SheetDB.go lib/db/migrations/008_sheets.go lib/db/migrations/001_initial_schema.go && git commit -m "feat(db): migration 008 sheet + sheet_op tables and models"`

---

## Task 3: `SheetMethods` interface + MemoryDataStore

**Files:** modify `lib/db/DataStore.go`, `lib/db/MemoryDataStore.go`; create `lib/db/sheet_memory_test.go`

- [ ] **Step 1: Interface** in `DataStore.go` (after `SecretMethods`, before `DataStore`):
```go
// SheetMethods persist spreadsheet documents (header snapshot + op-log),
// keyed by pad id (a sheet document is a pad with document_type "sheet").
type SheetMethods interface {
	SaveSheet(padId string, head int, snapshot string) error
	GetSheet(padId string) (*db.SheetDB, error)
	DoesSheetExist(padId string) (*bool, error)
	RemoveSheet(padId string) error
	SaveSheetOp(padId string, rev int, op string, authorId *string, timestamp int64) error
	GetSheetOps(padId string, startRev int, endRev int) (*[]db.SheetOpDB, error)
}
```
Add `SheetMethods` to the `DataStore` interface composition (after `SecretMethods`).

- [ ] **Step 2: Constant** — add to `lib/db/constants.go` (mirror `PadDoesNotExistError`): `SheetDoesNotExistError = "sheet does not exist"`. (Verify the existing constant names/style in that file first.)

- [ ] **Step 3: Failing-Test** — `lib/db/sheet_memory_test.go`:
```go
package db

import "testing"

func TestMemorySheetRoundTrip(t *testing.T) {
	m := NewMemoryDataStore()
	if err := m.SaveSheet("p1", 3, `{"sheets":[]}`); err != nil {
		t.Fatalf("SaveSheet: %v", err)
	}
	got, err := m.GetSheet("p1")
	if err != nil {
		t.Fatalf("GetSheet: %v", err)
	}
	if got.Head != 3 || got.Snapshot != `{"sheets":[]}` {
		t.Fatalf("unexpected sheet: %+v", got)
	}
	ex, _ := m.DoesSheetExist("p1")
	if ex == nil || !*ex {
		t.Fatal("DoesSheetExist should be true")
	}
}

func TestMemorySheetOps(t *testing.T) {
	m := NewMemoryDataStore()
	_ = m.SaveSheet("p1", 0, "{}")
	for r := 1; r <= 3; r++ {
		if err := m.SaveSheetOp("p1", r, `{"type":"setCell"}`, nil, int64(r)); err != nil {
			t.Fatalf("SaveSheetOp: %v", err)
		}
	}
	ops, err := m.GetSheetOps("p1", 2, 3)
	if err != nil {
		t.Fatalf("GetSheetOps: %v", err)
	}
	if len(*ops) != 2 || (*ops)[0].Rev != 2 || (*ops)[1].Rev != 3 {
		t.Fatalf("expected revs 2,3 got %+v", *ops)
	}
}
```

- [ ] **Step 4: Run — fail.** `go test ./lib/db/ -run TestMemorySheet -v`

- [ ] **Step 5: Implement Memory** — add fields to `MemoryDataStore` struct:
```go
	sheetStore map[string]db.SheetDB
	sheetOps   map[string]map[int]db.SheetOpDB
```
Init in `NewMemoryDataStore`:
```go
		sheetStore:             make(map[string]db.SheetDB),
		sheetOps:               make(map[string]map[int]db.SheetOpDB),
```
Add methods (new file `lib/db/MemorySheet.go` or append to MemoryDataStore.go):
```go
func (m *MemoryDataStore) SaveSheet(padId string, head int, snapshot string) error {
	now := time.Now()
	existing, ok := m.sheetStore[padId]
	created := now
	if ok {
		created = existing.CreatedAt
	}
	m.sheetStore[padId] = db.SheetDB{ID: padId, Head: head, Snapshot: snapshot, CreatedAt: created, UpdatedAt: &now}
	return nil
}

func (m *MemoryDataStore) GetSheet(padId string) (*db.SheetDB, error) {
	s, ok := m.sheetStore[padId]
	if !ok {
		return nil, errors.New(SheetDoesNotExistError)
	}
	return &s, nil
}

func (m *MemoryDataStore) DoesSheetExist(padId string) (*bool, error) {
	_, ok := m.sheetStore[padId]
	return &ok, nil
}

func (m *MemoryDataStore) RemoveSheet(padId string) error {
	delete(m.sheetStore, padId)
	delete(m.sheetOps, padId)
	return nil
}

func (m *MemoryDataStore) SaveSheetOp(padId string, rev int, op string, authorId *string, timestamp int64) error {
	if m.sheetOps[padId] == nil {
		m.sheetOps[padId] = make(map[int]db.SheetOpDB)
	}
	if _, exists := m.sheetOps[padId][rev]; exists {
		return nil // write-once
	}
	m.sheetOps[padId][rev] = db.SheetOpDB{PadId: padId, Rev: rev, Op: op, AuthorId: authorId, Timestamp: timestamp}
	return nil
}

func (m *MemoryDataStore) GetSheetOps(padId string, startRev int, endRev int) (*[]db.SheetOpDB, error) {
	out := make([]db.SheetOpDB, 0)
	for r := startRev; r <= endRev; r++ {
		if op, ok := m.sheetOps[padId][r]; ok {
			out = append(out, op)
		}
	}
	return &out, nil
}
```
(Confirm `time` and `errors` are imported in the target file.)

- [ ] **Step 6: Run — pass.** `go test ./lib/db/ -run TestMemorySheet -v`
- [ ] **Step 7: Commit.** `git add lib/db/DataStore.go lib/db/constants.go lib/db/MemoryDataStore.go lib/db/MemorySheet.go lib/db/sheet_memory_test.go && git commit -m "feat(db): SheetMethods interface + memory store impl"`

---

## Task 4: SQLite implementation + test

**Files:** create `lib/db/SQLiteSheet.go`, `lib/db/sheet_sqlite_test.go`

- [ ] **Step 1: Failing-Test** — `lib/db/sheet_sqlite_test.go`:
```go
package db

import (
	"testing"

	dbmodel "github.com/ether/etherpad-go/lib/models/db"
)

func TestSQLiteSheetRoundTrip(t *testing.T) {
	store := newTestSQLiteStore(t) // helper from document_type_sqlite_test.go
	// FK requires the pad row to exist first.
	if err := store.CreatePad("p1", dbmodelPadDB("p1", "sheet")); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	if err := store.SaveSheet("p1", 5, `{"sheets":[{"id":"s1"}]}`); err != nil {
		t.Fatalf("SaveSheet: %v", err)
	}
	got, err := store.GetSheet("p1")
	if err != nil {
		t.Fatalf("GetSheet: %v", err)
	}
	if got.Head != 5 || got.Snapshot != `{"sheets":[{"id":"s1"}]}` {
		t.Fatalf("unexpected: %+v", got)
	}
	// upsert
	if err := store.SaveSheet("p1", 6, "{}"); err != nil {
		t.Fatalf("SaveSheet upsert: %v", err)
	}
	got2, _ := store.GetSheet("p1")
	if got2.Head != 6 {
		t.Fatalf("upsert head not updated: %+v", got2)
	}

	for r := 1; r <= 3; r++ {
		if err := store.SaveSheetOp("p1", r, `{"type":"setCell"}`, nil, int64(r*10)); err != nil {
			t.Fatalf("SaveSheetOp: %v", err)
		}
	}
	ops, err := store.GetSheetOps("p1", 1, 2)
	if err != nil {
		t.Fatalf("GetSheetOps: %v", err)
	}
	if len(*ops) != 2 {
		t.Fatalf("expected 2 ops, got %d", len(*ops))
	}
	_ = dbmodel.SheetDB{}
}
```

- [ ] **Step 2: Run — fail.** `go test ./lib/db/ -run TestSQLiteSheet -v`

- [ ] **Step 3: Implement** — `lib/db/SQLiteSheet.go`:
```go
package db

import (
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/models/db"
)

func (d SQLiteDB) SaveSheet(padId string, head int, snapshot string) error {
	q, args, err := sq.Insert("sheet").
		Columns("id", "head", "snapshot").
		Values(padId, head, snapshot).
		Suffix(`ON CONFLICT(id) DO UPDATE SET
			head = excluded.head,
			snapshot = excluded.snapshot,
			updated_at = CURRENT_TIMESTAMP`).
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(q, args...)
	return err
}

func (d SQLiteDB) GetSheet(padId string) (*db.SheetDB, error) {
	q, args, err := sq.Select("id", "head", "snapshot", "created_at", "updated_at").
		From("sheet").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return nil, err
	}
	var s db.SheetDB
	err = d.sqlDB.QueryRow(q, args...).Scan(&s.ID, &s.Head, &s.Snapshot, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New(SheetDoesNotExistError)
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d SQLiteDB) DoesSheetExist(padId string) (*bool, error) {
	q, args, err := sq.Select("1").From("sheet").Where(sq.Eq{"id": padId}).Limit(1).ToSql()
	if err != nil {
		return nil, err
	}
	var x int
	err = d.sqlDB.QueryRow(q, args...).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		f := false
		return &f, nil
	}
	if err != nil {
		return nil, err
	}
	tr := true
	return &tr, nil
}

func (d SQLiteDB) RemoveSheet(padId string) error {
	q, args, err := sq.Delete("sheet").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(q, args...)
	return err
}

func (d SQLiteDB) SaveSheetOp(padId string, rev int, op string, authorId *string, timestamp int64) error {
	q, args, err := sq.Insert("sheet_op").
		Columns("id", "rev", "op", "author_id", "timestamp").
		Values(padId, rev, op, authorId, timestamp).
		Suffix("ON CONFLICT(id, rev) DO NOTHING").
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(q, args...)
	return err
}

func (d SQLiteDB) GetSheetOps(padId string, startRev int, endRev int) (*[]db.SheetOpDB, error) {
	q, args, err := sq.Select("id", "rev", "op", "author_id", "timestamp").
		From("sheet_op").
		Where(sq.Eq{"id": padId}).
		Where(sq.GtOrEq{"rev": startRev}).
		Where(sq.LtOrEq{"rev": endRev}).
		OrderBy("rev ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := d.sqlDB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]db.SheetOpDB, 0)
	for rows.Next() {
		var o db.SheetOpDB
		if err := rows.Scan(&o.PadId, &o.Rev, &o.Op, &o.AuthorId, &o.Timestamp); err != nil {
			return nil, fmt.Errorf("scan sheet_op: %w", err)
		}
		out = append(out, o)
	}
	return &out, rows.Err()
}
```

- [ ] **Step 4: Run — pass.** `go test ./lib/db/ -run TestSQLiteSheet -v`
- [ ] **Step 5: Commit.** `git add lib/db/SQLiteSheet.go lib/db/sheet_sqlite_test.go && git commit -m "feat(db): SQLite SheetMethods implementation"`

---

## Task 5: Postgres + MySQL implementations + interface assertions

**Files:** create `lib/db/PostgresSheet.go`, `lib/db/MySQLSheet.go`

- [ ] **Step 1: Postgres** — `lib/db/PostgresSheet.go` (uses pgx pool + `$n`, mirrors PostgresDB SaveRevision style):
```go
package db

import (
	"context"
	"errors"

	"github.com/ether/etherpad-go/lib/models/db"
	"github.com/jackc/pgx/v5"
)

func (d PostgresDB) SaveSheet(padId string, head int, snapshot string) error {
	_, err := d.pool.Exec(context.Background(),
		`INSERT INTO sheet (id, head, snapshot, created_at, updated_at)
         VALUES ($1, $2, $3, NOW(), NOW())
         ON CONFLICT (id) DO UPDATE SET head = EXCLUDED.head, snapshot = EXCLUDED.snapshot, updated_at = NOW()`,
		padId, head, snapshot)
	return err
}

func (d PostgresDB) GetSheet(padId string) (*db.SheetDB, error) {
	var s db.SheetDB
	err := d.pool.QueryRow(context.Background(),
		`SELECT id, head, snapshot, created_at, updated_at FROM sheet WHERE id = $1`, padId).
		Scan(&s.ID, &s.Head, &s.Snapshot, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, errors.New(SheetDoesNotExistError)
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d PostgresDB) DoesSheetExist(padId string) (*bool, error) {
	var exists bool
	err := d.pool.QueryRow(context.Background(),
		`SELECT EXISTS(SELECT 1 FROM sheet WHERE id = $1)`, padId).Scan(&exists)
	if err != nil {
		return nil, err
	}
	return &exists, nil
}

func (d PostgresDB) RemoveSheet(padId string) error {
	_, err := d.pool.Exec(context.Background(), `DELETE FROM sheet WHERE id = $1`, padId)
	return err
}

func (d PostgresDB) SaveSheetOp(padId string, rev int, op string, authorId *string, timestamp int64) error {
	_, err := d.pool.Exec(context.Background(),
		`INSERT INTO sheet_op (id, rev, op, author_id, timestamp, created_at)
         VALUES ($1, $2, $3, $4, $5, NOW()) ON CONFLICT (id, rev) DO NOTHING`,
		padId, rev, op, authorId, timestamp)
	return err
}

func (d PostgresDB) GetSheetOps(padId string, startRev int, endRev int) (*[]db.SheetOpDB, error) {
	rows, err := d.pool.Query(context.Background(),
		`SELECT id, rev, op, author_id, timestamp FROM sheet_op
         WHERE id = $1 AND rev >= $2 AND rev <= $3 ORDER BY rev ASC`,
		padId, startRev, endRev)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]db.SheetOpDB, 0)
	for rows.Next() {
		var o db.SheetOpDB
		if err := rows.Scan(&o.PadId, &o.Rev, &o.Op, &o.AuthorId, &o.Timestamp); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return &out, rows.Err()
}
```

- [ ] **Step 2: MySQL** — `lib/db/MySQLSheet.go` (mirrors MysqlDB; uses `mysql` builder var + `d.sqlDB`):
```go
package db

import (
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/ether/etherpad-go/lib/models/db"
)

func (d MysqlDB) SaveSheet(padId string, head int, snapshot string) error {
	q, args, err := mysql.Insert("sheet").
		Columns("id", "head", "snapshot").
		Values(padId, head, snapshot).
		Suffix("ON DUPLICATE KEY UPDATE head = VALUES(head), snapshot = VALUES(snapshot)").
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(q, args...)
	return err
}

func (d MysqlDB) GetSheet(padId string) (*db.SheetDB, error) {
	q, args, err := mysql.Select("id", "head", "snapshot", "created_at", "updated_at").
		From("sheet").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return nil, err
	}
	var s db.SheetDB
	err = d.sqlDB.QueryRow(q, args...).Scan(&s.ID, &s.Head, &s.Snapshot, &s.CreatedAt, &s.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, errors.New(SheetDoesNotExistError)
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func (d MysqlDB) DoesSheetExist(padId string) (*bool, error) {
	q, args, err := mysql.Select("1").From("sheet").Where(sq.Eq{"id": padId}).Limit(1).ToSql()
	if err != nil {
		return nil, err
	}
	var x int
	err = d.sqlDB.QueryRow(q, args...).Scan(&x)
	if errors.Is(err, sql.ErrNoRows) {
		f := false
		return &f, nil
	}
	if err != nil {
		return nil, err
	}
	tr := true
	return &tr, nil
}

func (d MysqlDB) RemoveSheet(padId string) error {
	q, args, err := mysql.Delete("sheet").Where(sq.Eq{"id": padId}).ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(q, args...)
	return err
}

func (d MysqlDB) SaveSheetOp(padId string, rev int, op string, authorId *string, timestamp int64) error {
	q, args, err := mysql.Insert("sheet_op").
		Columns("id", "rev", "op", "author_id", "timestamp").
		Values(padId, rev, op, authorId, timestamp).
		Suffix("ON DUPLICATE KEY UPDATE id = id").
		ToSql()
	if err != nil {
		return err
	}
	_, err = d.sqlDB.Exec(q, args...)
	return err
}

func (d MysqlDB) GetSheetOps(padId string, startRev int, endRev int) (*[]db.SheetOpDB, error) {
	q, args, err := mysql.Select("id", "rev", "op", "author_id", "timestamp").
		From("sheet_op").
		Where(sq.Eq{"id": padId}).
		Where(sq.GtOrEq{"rev": startRev}).
		Where(sq.LtOrEq{"rev": endRev}).
		OrderBy("rev ASC").
		ToSql()
	if err != nil {
		return nil, err
	}
	rows, err := d.sqlDB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]db.SheetOpDB, 0)
	for rows.Next() {
		var o db.SheetOpDB
		if err := rows.Scan(&o.PadId, &o.Rev, &o.Op, &o.AuthorId, &o.Timestamp); err != nil {
			return nil, fmt.Errorf("scan sheet_op: %w", err)
		}
		out = append(out, o)
	}
	return &out, rows.Err()
}
```

- [ ] **Step 3: Build (compile-checks all 4 `var _ DataStore` assertions).** `go build ./...`
Expected: ok. If a struct is missing a method, the `var _ DataStore = (*X)(nil)` assertion fails to compile — fix the missing method.

- [ ] **Step 4: Full db tests (non-docker).** `go test ./lib/db/ -run 'TestMemorySheet|TestSQLiteSheet'`
- [ ] **Step 5: gofmt + commit.** `gofmt -w lib/db/*.go && git add lib/db/PostgresSheet.go lib/db/MySQLSheet.go && git commit -m "feat(db): Postgres and MySQL SheetMethods implementations"`

---

## Self-Review (Planner)

- **Coverage:** Snapshot serialization (T1), tables+models+migration (T2), interface+memory (T3), SQLite (T4), Postgres+MySQL+assertions (T5). All four DataStore implementers get SheetMethods → `var _ DataStore` compiles.
- **Placeholders:** none; every method body shown.
- **Type consistency:** `SaveSheet(padId,head,snapshot string)`, `GetSheet→*db.SheetDB`, `SaveSheetOp(padId,rev,op,authorId,timestamp)`, `GetSheetOps(padId,start,end)→*[]db.SheetOpDB` identical across all 4 impls and the interface. `SheetDoesNotExistError` constant shared. Snapshot JSON produced by `Workbook.Snapshot()` (T1) is what the WS/manager layer (plan 2c) will pass to `SaveSheet`.
- **FK note:** sheet/sheet_op reference `pad(id) ON DELETE CASCADE`, so removing a pad cleans up its sheet data automatically; tests must `CreatePad` first (SQLite enforces FKs).

## Roadmap next: Plan 2c (WS handler), Plan 3 (frontend), Plan 4 (xlsx).
