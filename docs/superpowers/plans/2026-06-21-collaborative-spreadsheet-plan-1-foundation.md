# Kollaborative Tabelle — Plan 1: Dokumenttyp-Fundament

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Einen expliziten Dokumenttyp (`text` | `sheet`) end-to-end durch Modell, Persistenz und Routing ziehen, sodass man eine Tabelle anlegen kann, deren Typ persistiert wird und die unter `/s/:pad` einen (noch leeren) Sheet-Editor öffnet — vollständig abwärtskompatibel zu bestehenden Text-Pads.

**Architecture:** Wir erweitern das vorhandene `Pad`/`PadDB`-Modell um ein `DocumentType`-Feld (Default `"text"`), fügen eine DB-Migration und die Spalte in allen `DataStore`-Implementierungen hinzu, ergänzen einen typisierten Erstellungspfad im `PadManager`, und registrieren die Route `/s/:pad` mit einem Stub-Handler + Stub-Frontend. Der Text-Pfad bleibt unberührt.

**Tech Stack:** Go (Fiber, Squirrel, database/sql), templ (HTML-Komponenten), TypeScript + Vite (Frontend), SQLite/Postgres/MySQL/Memory als DataStores.

**Bezug:** Umsetzung der Spec `docs/superpowers/specs/2026-06-21-collaborative-spreadsheet-design.md`, Abschnitt 2 (Dokument-Modell & Integration). Spätere Pläne (2–4) bauen darauf auf — Roadmap am Ende dieses Dokuments.

---

## Datei-Struktur (Plan 1)

| Datei | Verantwortung | Aktion |
|-------|---------------|--------|
| `lib/models/db/PadDB.go` | DB-Repräsentation eines Pads | Feld `DocumentType` ergänzen |
| `lib/models/pad/Pad.go` | In-Memory-Pad-Modell + Persistenz (`Save()`) | Feld + Default + Persistenz |
| `lib/models/pad/mapper.go` | DB→Modell-Mapping | `DocumentType` mappen |
| `lib/db/migrations/007_document_type.go` | Schema-Migration | **NEU** |
| `lib/db/migrations/migrations.go` | Migrations-Registry | migration007 registrieren |
| `lib/db/commonMappers.go` | `ReadToPadDB` (row→PadDB) | Spalte scannen |
| `lib/db/SQLiteDB.go` | SQLite CreatePad/GetPad | Spalte ergänzen |
| `lib/db/PostgresDB.go` | Postgres CreatePad/GetPad | Spalte ergänzen |
| `lib/db/MySQLDB.go` | MySQL CreatePad/GetPad | Spalte ergänzen |
| `lib/pad/padManager.go` | Pad-Lebenszyklus | typisierter Erstellungspfad `GetTypedPad` |
| `assets/sheet/sheet.templ` | Stub-HTML-Shell des Sheet-Editors | **NEU** |
| `lib/api/pad/sheetFrontend.go` | HTTP-Handler `HandleSheetOpen` | **NEU** |
| `lib/api/static/init.go` | Routen-Registrierung | `/s/:pad` + statisches JS |
| `ui/src/sheet.entry.ts` | Frontend-Entry-Stub | **NEU** |
| `ui/vite.config.ts` | Vite-Entry-Mapping | `sheet`-Modus |
| `ui/package.json` | Build-Script | `vite build --mode sheet` |
| `assets/welcome/main.templ` | Landing-Page | Button „Neue Tabelle" |
| `ui/src/js/index.ts` | Landing-Logik | Tabelle → `/s/...` |

`MemoryDataStore` braucht **keine** Änderung: es speichert die komplette `PadDB`-Struct in einer Map (`lib/db/MemoryDataStore.go:61-87`), das neue Feld wird automatisch mitgeführt.

---

## Task 1: `DocumentType` im Datenmodell + Persistenz (Struct-Ebene)

**Files:**
- Modify: `lib/models/db/PadDB.go:30-42`
- Modify: `lib/models/pad/Pad.go:45-59` (Struct), `:61-74` (NewPad), `:502-514` (Save)
- Modify: `lib/models/pad/mapper.go:9-41`
- Test: `lib/test/pad/document_type_test.go` (NEU)

- [ ] **Step 1: Failing-Test schreiben**

Neue Datei `lib/test/pad/document_type_test.go`. Sie nutzt den Memory-Store, legt ein Pad an, setzt den Typ und prüft Roundtrip über `Save()` + `GetPad()`. Orientiere dich für Setup an `lib/test/pad/pad_test.go` (gleiches Paket-Setup, `db.NewMemoryDataStore()`-Konstruktor — exakten Konstruktornamen aus `lib/db/MemoryDataStore.go` übernehmen).

```go
package pad

import (
	"testing"

	"github.com/ether/etherpad-go/lib/models/db"
)

func TestDocumentTypeDefaultsToText(t *testing.T) {
	store := db.NewMemoryDataStore() // Konstruktornamen aus MemoryDataStore.go verifizieren
	padDB := db.PadDB{ID: "test-default"}
	if err := store.CreatePad("test-default", padDB); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	got, err := store.GetPad("test-default")
	if err != nil {
		t.Fatalf("GetPad: %v", err)
	}
	if got.DocumentType != "text" && got.DocumentType != "" {
		t.Fatalf("expected text/empty default, got %q", got.DocumentType)
	}
}

func TestDocumentTypeRoundTrip(t *testing.T) {
	store := db.NewMemoryDataStore()
	padDB := db.PadDB{ID: "test-sheet", DocumentType: "sheet"}
	if err := store.CreatePad("test-sheet", padDB); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	got, err := store.GetPad("test-sheet")
	if err != nil {
		t.Fatalf("GetPad: %v", err)
	}
	if got.DocumentType != "sheet" {
		t.Fatalf("expected sheet, got %q", got.DocumentType)
	}
}
```

- [ ] **Step 2: Test ausführen — muss fehlschlagen (Compile-Fehler)**

Run: `go test ./lib/test/pad/ -run TestDocumentType -v`
Expected: FAIL — Compile-Fehler `padDB.DocumentType undefined (type db.PadDB has no field DocumentType)`.

- [ ] **Step 3: Feld zu `PadDB` ergänzen**

In `lib/models/db/PadDB.go`, in der `PadDB`-Struct (ab Zeile 30) nach `PublicStatus` einfügen:

```go
	PublicStatus   bool            `json:"publicStatus"`
	DocumentType   string          `json:"documentType"`
	ATextText      string          `json:"atextText"`
```

- [ ] **Step 4: Feld zu `Pad`-Modell ergänzen + Default in `NewPad`**

In `lib/models/pad/Pad.go`, in der `Pad`-Struct (Zeile 45-59) nach `PublicStatus bool` einfügen:

```go
	PublicStatus   bool
	DocumentType   string
```

In `NewPad` (Zeile 61-74), vor `return *p` ergänzen:

```go
	p.PublicStatus = false
	p.DocumentType = "text"
```

- [ ] **Step 5: `DocumentType` in `Save()` persistieren**

In `lib/models/pad/Pad.go`, im `db.PadDB{...}`-Literal in `Save()` (Zeile 502-514) ergänzen:

```go
		PublicStatus:   p.PublicStatus,
		DocumentType:   p.DocumentType,
		UpdatedAt:      &updatedAt,
```

- [ ] **Step 6: Mapper erweitern**

In `lib/models/pad/mapper.go`, in `mapDBPadToModel` (Zeile 9-41) nach `padToAssignTo.PublicStatus = dbPad.PublicStatus` einfügen:

```go
	padToAssignTo.PublicStatus = dbPad.PublicStatus
	if dbPad.DocumentType == "" {
		padToAssignTo.DocumentType = "text"
	} else {
		padToAssignTo.DocumentType = dbPad.DocumentType
	}
```

- [ ] **Step 7: Test ausführen — muss bestehen**

Run: `go test ./lib/test/pad/ -run TestDocumentType -v`
Expected: PASS (beide Tests).

- [ ] **Step 8: Commit**

```bash
git add lib/models/db/PadDB.go lib/models/pad/Pad.go lib/models/pad/mapper.go lib/test/pad/document_type_test.go
git commit -m "feat(pad): add DocumentType field to pad model and persistence"
```

---

## Task 2: DB-Migration 007 (`document_type`-Spalte)

**Files:**
- Create: `lib/db/migrations/007_document_type.go`
- Modify: `lib/db/migrations/migrations.go:8-16` (GetMigrations-Liste)

- [ ] **Step 1: Migrationsdatei schreiben**

Neue Datei `lib/db/migrations/007_document_type.go` (Muster aus `002_server_version.go`):

```go
package migrations

import (
	"database/sql"
)

func migration007DocumentType() Migration {
	return Migration{
		Version:     7,
		Description: "Add document_type column to pad",
		Up: func(db *sql.DB, dialect Dialect) error {
			var query string
			switch dialect {
			case DialectMySQL:
				query = `ALTER TABLE pad ADD COLUMN document_type VARCHAR(32) NOT NULL DEFAULT 'text'`
			case DialectPostgres:
				query = `ALTER TABLE pad ADD COLUMN IF NOT EXISTS document_type TEXT NOT NULL DEFAULT 'text'`
			default:
				query = `ALTER TABLE pad ADD COLUMN document_type TEXT NOT NULL DEFAULT 'text'`
			}
			if _, err := db.Exec(query); err != nil {
				return err
			}
			return nil
		},
	}
}
```

> Hinweis MySQL/SQLite: `ADD COLUMN` ohne `IF NOT EXISTS`. Die Migration läuft genau einmal (Versionsvergleich in `migrations.go:61-70`), daher unkritisch. Falls die Zielversion bereits eine `pad`-Tabelle ohne `document_type` hat, fügt die Migration sie additiv hinzu — bestehende Zeilen erhalten `'text'`.

- [ ] **Step 2: Migration registrieren**

In `lib/db/migrations/migrations.go`, in `GetMigrations()` (Zeile 8-16) den neuen Eintrag ans Ende der Slice anhängen:

```go
		migration006SecretRotation(),
		migration007DocumentType(),
	}
```

- [ ] **Step 3: Build prüfen**

Run: `go build ./lib/db/...`
Expected: kein Fehler.

- [ ] **Step 4: Commit**

```bash
git add lib/db/migrations/007_document_type.go lib/db/migrations/migrations.go
git commit -m "feat(db): migration to add document_type column to pad table"
```

---

## Task 3: `document_type` in SQL-DataStores (SQLite, Postgres, MySQL)

**Files:**
- Modify: `lib/db/commonMappers.go:25-41` (ReadToPadDB)
- Modify: `lib/db/SQLiteDB.go:29-64` (CreatePad), `:66-89` (GetPad)
- Modify: `lib/db/PostgresDB.go:34-64` (CreatePad), `:66-82` (GetPad)
- Modify: `lib/db/MySQLDB.go:56-90` (CreatePad), `:92-115` (GetPad)
- Test: `lib/db/document_type_sqlite_test.go` (NEU)

- [ ] **Step 1: Failing-Test schreiben (SQLite, In-Memory)**

Neue Datei `lib/db/document_type_sqlite_test.go`. Verifiziere zuerst den SQLite-Konstruktor in `lib/db/SQLiteDB.go` (Funktion, die `SQLiteDB` erzeugt und Migrationen via `migrations.NewMigrationManager(...).Run()` laufen lässt) und den Test-Helfer in `lib/db/datastore_test_helper.go`. Nutze eine In-Memory-SQLite-DB (`:memory:` oder eine Temp-Datei).

```go
package db

import "testing"

func TestSQLiteDocumentTypePersists(t *testing.T) {
	store := newTestSQLiteStore(t) // Helfer: erstellt SQLiteDB + führt Migrationen aus
	if err := store.CreatePad("s1", dbmodelPadDB("s1", "sheet")); err != nil {
		t.Fatalf("CreatePad: %v", err)
	}
	got, err := store.GetPad("s1")
	if err != nil {
		t.Fatalf("GetPad: %v", err)
	}
	if got.DocumentType != "sheet" {
		t.Fatalf("expected sheet, got %q", got.DocumentType)
	}
}
```

Ergänze im Testfile zwei kleine Helfer (oder nutze vorhandene aus `datastore_test_helper.go`): `newTestSQLiteStore(t)` (öffnet eine frische SQLite-DB inkl. Migrationen) und `dbmodelPadDB(id, docType)` (baut eine minimale `db.PadDB`). Den Migrations-Runner-Aufruf aus dem produktiven SQLite-Konstruktor übernehmen, damit `document_type` existiert.

- [ ] **Step 2: Test ausführen — muss fehlschlagen**

Run: `go test ./lib/db/ -run TestSQLiteDocumentType -v`
Expected: FAIL — `document_type` wird nicht geschrieben/gelesen (entweder SQL-Fehler „no column" falls Migration nicht lief, oder `DocumentType` ist leer).

- [ ] **Step 3: `ReadToPadDB` um Spalte erweitern**

In `lib/db/commonMappers.go`, in `ReadToPadDB` (Zeile 25-41) die Scan-Reihenfolge erweitern. **Wichtig:** Die Spalte muss in CreatePad/GetPad-SELECT an derselben Position stehen. Wir hängen `document_type` direkt nach `public_status` an (konsistent mit dem Struct).

```go
	if err := reader.Scan(&padDB.ID, &padDB.Head, &savedRevisions, &padDB.ReadOnlyId, &pool,
		&padDB.ChatHead, &padDB.PublicStatus, &padDB.DocumentType, &padDB.ATextText, &padDB.ATextAttribs,
		&padDB.CreatedAt, &padDB.UpdatedAt); err != nil {
		return nil, err
	}
```

- [ ] **Step 4: SQLite CreatePad + GetPad**

In `lib/db/SQLiteDB.go`, `CreatePad` (Zeile 40-46): `document_type` in `Columns(...)` und den Wert `padDB.DocumentType` in `Values(...)` einfügen (nach `public_status`), und im `ON CONFLICT`-Suffix ergänzen:

```go
		Insert("pad").
		Columns("id", "head", "saved_revisions", "readonly_id", "pool", "chat_head",
			"public_status", "document_type", "atext_text", "atext_attribs").
		Values(padID, padDB.Head, string(savedRevisions), padDB.ReadOnlyId, string(pool),
			padDB.ChatHead, padDB.PublicStatus, padDB.DocumentType, padDB.ATextText, padDB.ATextAttribs).
		Suffix(`ON CONFLICT(id) DO UPDATE SET
			head = excluded.head,
			saved_revisions = excluded.saved_revisions,
			readonly_id = excluded.readonly_id,
			pool = excluded.pool,
			chat_head = excluded.chat_head,
			public_status = excluded.public_status,
			document_type = excluded.document_type,
			atext_text = excluded.atext_text,
			atext_attribs = excluded.atext_attribs,
			updated_at = CURRENT_TIMESTAMP`).
```

`GetPad` (Zeile 67-69): `document_type` nach `public_status` in `Select(...)` einfügen:

```go
		Select("id", "head", "saved_revisions", "readonly_id", "pool", "chat_head",
			"public_status", "document_type", "atext_text", "atext_attribs", "created_at", "updated_at").
```

- [ ] **Step 5: SQLite-Test ausführen — muss bestehen**

Run: `go test ./lib/db/ -run TestSQLiteDocumentType -v`
Expected: PASS.

- [ ] **Step 6: Postgres CreatePad + GetPad spiegeln**

In `lib/db/PostgresDB.go`, `CreatePad` (Zeile 47-62): die `INSERT`-Spaltenliste, die `VALUES`-Platzhalter ($-Nummern um eins erhöhen ab der eingefügten Position), das `ON CONFLICT`-Set und die Argumentliste um `document_type` / `padDB.DocumentType` erweitern:

```go
	_, err = d.pool.Exec(ctx,
		`INSERT INTO pad (id, head, saved_revisions, readonly_id, pool, chat_head, 
                          public_status, document_type, atext_text, atext_attribs, created_at, updated_at)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
         ON CONFLICT (id) DO UPDATE SET
             head = EXCLUDED.head,
             saved_revisions = EXCLUDED.saved_revisions,
             readonly_id = EXCLUDED.readonly_id,
             pool = EXCLUDED.pool,
             chat_head = EXCLUDED.chat_head,
             public_status = EXCLUDED.public_status,
             document_type = EXCLUDED.document_type,
             atext_text = EXCLUDED.atext_text,
             atext_attribs = EXCLUDED.atext_attribs,
             updated_at = NOW()`,
		padID, padDB.Head, savedRevisions, padDB.ReadOnlyId, pool,
		padDB.ChatHead, padDB.PublicStatus, padDB.DocumentType, padDB.ATextText, padDB.ATextAttribs)
```

`GetPad` (Zeile 70-72): `document_type` nach `public_status` ins SELECT einfügen:

```go
		`SELECT id, head, saved_revisions, readonly_id, pool, chat_head, 
                public_status, document_type, atext_text, atext_attribs, created_at, updated_at
         FROM pad WHERE id = $1`,
```

- [ ] **Step 7: MySQL CreatePad + GetPad spiegeln**

In `lib/db/MySQLDB.go`, `CreatePad` (Zeile 67-82): `document_type` in `Columns(...)`/`Values(...)` (nach `public_status`) und im `ON DUPLICATE KEY UPDATE` ergänzen:

```go
		Insert("pad").
		Columns("id", "head", "saved_revisions", "readonly_id", "pool", "chat_head",
			"public_status", "document_type", "atext_text", "atext_attribs").
		Values(padID, padDB.Head, string(savedRevisions), padDB.ReadOnlyId, string(pool),
			padDB.ChatHead, padDB.PublicStatus, padDB.DocumentType, padDB.ATextText, padDB.ATextAttribs).
		Suffix(`ON DUPLICATE KEY UPDATE
			head = VALUES(head),
			saved_revisions = VALUES(saved_revisions),
			readonly_id = VALUES(readonly_id),
			pool = VALUES(pool),
			chat_head = VALUES(chat_head),
			public_status = VALUES(public_status),
			document_type = VALUES(document_type),
			atext_text = VALUES(atext_text),
			atext_attribs = VALUES(atext_attribs)`).
```

`GetPad` (Zeile 94-95): `document_type` nach `public_status` ins `Select(...)` einfügen:

```go
		Select("id", "head", "saved_revisions", "readonly_id", "pool", "chat_head",
			"public_status", "document_type", "atext_text", "atext_attribs", "created_at", "updated_at").
```

- [ ] **Step 8: Gesamtes db-Paket bauen + Tests**

Run: `go build ./lib/db/... && go test ./lib/db/ -run TestSQLiteDocumentType -v`
Expected: Build ok, Test PASS. (Postgres/MySQL werden in CI/E2E mit echten DBs gedeckt; die Spaltenparität ist hier durch identisches Muster sichergestellt.)

- [ ] **Step 9: Commit**

```bash
git add lib/db/commonMappers.go lib/db/SQLiteDB.go lib/db/PostgresDB.go lib/db/MySQLDB.go lib/db/document_type_sqlite_test.go
git commit -m "feat(db): persist document_type across all SQL datastores"
```

---

## Task 4: Typisierter Erstellungspfad im PadManager

**Files:**
- Modify: `lib/pad/padManager.go:151-177` (GetPad → neue Schwester-Methode)
- Test: `lib/pad/pad_manager_document_type_test.go` (NEU)

- [ ] **Step 1: Failing-Test schreiben**

Neue Datei `lib/pad/pad_manager_document_type_test.go`. Erstelle einen `Manager` mit Memory-Store (Konstruktor aus `padManager.go` oben im File verifizieren, ebenso wie `m.author`/`m.hook` initialisiert werden — ggf. vorhandenen Manager-Test-Helfer wiederverwenden):

```go
package pad

import "testing"

func TestGetTypedPadPersistsSheetType(t *testing.T) {
	m := newTestManager(t) // Helfer analog zu bestehenden Manager-Tests
	p, err := m.GetTypedPad("sheet-1", "sheet", nil)
	if err != nil {
		t.Fatalf("GetTypedPad: %v", err)
	}
	if p.DocumentType != "sheet" {
		t.Fatalf("expected sheet, got %q", p.DocumentType)
	}
	reloaded, err := m.store.GetPad("sheet-1")
	if err != nil {
		t.Fatalf("store.GetPad: %v", err)
	}
	if reloaded.DocumentType != "sheet" {
		t.Fatalf("expected persisted sheet, got %q", reloaded.DocumentType)
	}
}
```

- [ ] **Step 2: Test ausführen — muss fehlschlagen**

Run: `go test ./lib/pad/ -run TestGetTypedPad -v`
Expected: FAIL — `m.GetTypedPad undefined`.

- [ ] **Step 3: `GetTypedPad` implementieren**

In `lib/pad/padManager.go`, direkt nach `GetPad` (nach Zeile 177) einfügen. Die Methode spiegelt `GetPad`, setzt aber `DocumentType` **vor** `Init` (damit `Init → AppendRevision → Save()` den Typ in `CreatePad` persistiert, vgl. `Pad.go:575`/`:502`):

```go
// GetTypedPad lädt oder erstellt ein Pad eines bestimmten Dokumenttyps.
// Bei bestehendem Pad wird der gespeicherte Typ beibehalten; documentType
// greift nur bei Erstanlage.
func (m *Manager) GetTypedPad(padID string, documentType string, authorId *string) (*pad.Pad, error) {
	if !m.IsValidPadId(padID) {
		return nil, errors.New("invalid pad id")
	}

	if cachedPad := m.globalPadCache.GetPad(padID); cachedPad != nil {
		return cachedPad, nil
	}

	newPad := pad.NewPad(padID, m.store, m.hook)
	newPad.DocumentType = documentType

	if err := newPad.Init(nil, authorId, m.author); err != nil {
		return nil, err
	}
	m.globalPadCache.SetPad(padID, &newPad)
	return &newPad, nil
}
```

> Prüfe beim Einfügen, ob `errors` bereits importiert ist (in `GetPad` wird `errors.New` genutzt → ja). `newPad.Init` gibt einen Fehler zurück (`Pad.go:352`); anders als das vorhandene `GetPad`, das den Fehler verschluckt, behandeln wir ihn hier korrekt.

- [ ] **Step 4: Test ausführen — muss bestehen**

Run: `go test ./lib/pad/ -run TestGetTypedPad -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add lib/pad/padManager.go lib/pad/pad_manager_document_type_test.go
git commit -m "feat(pad): add GetTypedPad creation path for document types"
```

---

## Task 5: Sheet-Route + Stub-Handler + Stub-Templ

**Files:**
- Create: `assets/sheet/sheet.templ`
- Create: `lib/api/pad/sheetFrontend.go`
- Modify: `lib/api/static/init.go:280-282` (Route), `:308-311` (statisches JS)
- Test: `lib/api/pad/sheet_frontend_test.go` (NEU)

- [ ] **Step 1: Stub-Templ-Komponente schreiben**

Neue Datei `assets/sheet/sheet.templ` (Paketname am vorhandenen `assets/pad/pad.templ`/`assets/welcome/main.templ` ausrichten — beide nutzen ein Paket pro Verzeichnis; verwende `package sheet`):

```go
package sheet

templ SheetIndex(padName string, jsScript string) {
	<!DOCTYPE html>
	<html lang="en">
		<head>
			<meta charset="utf-8"/>
			<meta name="viewport" content="width=device-width, initial-scale=1"/>
			<title>{ padName } — Spreadsheet</title>
		</head>
		<body>
			<div id="sheet-root" data-pad-name={ padName }></div>
			<script type="module" src={ jsScript }></script>
		</body>
	</html>
}
```

- [ ] **Step 2: templ generieren**

Run: `go run github.com/a-h/templ/cmd/templ generate ./assets/sheet/`
Expected: erzeugt `assets/sheet/sheet_templ.go`. (Falls eine globale templ-Binary installiert ist, geht auch `templ generate`.)

- [ ] **Step 3: Handler-Test schreiben (Failing)**

Neue Datei `lib/api/pad/sheet_frontend_test.go`. Orientiere dich an einem evtl. vorhandenen Handler-Test; minimal prüfen wir, dass `HandleSheetOpen` existiert und für einen Fiber-Test-Request 200 + HTML liefert. Setup analog zu `HandlePadOpen` (`padFrontend.go:31`):

```go
package pad

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestHandleSheetOpenRenders(t *testing.T) {
	app := fiber.New()
	app.Get("/s/:pad", func(c fiber.Ctx) error {
		return HandleSheetOpen(c)
	})
	req := httptest.NewRequest("GET", "/s/demo", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
```

- [ ] **Step 4: Test ausführen — muss fehlschlagen**

Run: `go test ./lib/api/pad/ -run TestHandleSheetOpen -v`
Expected: FAIL — `HandleSheetOpen undefined`.

- [ ] **Step 5: Handler implementieren**

Neue Datei `lib/api/pad/sheetFrontend.go`. Bewusst schlank (Stub): rendert die Sheet-Shell. Die Pad-Erstellung mit Typ `sheet` erfolgt beim WebSocket-Connect in Plan 2; hier reicht das Ausliefern der Shell.

```go
package pad

import (
	"strconv"

	"github.com/a-h/templ"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	sheetAsset "github.com/ether/etherpad-go/assets/sheet"
)

func HandleSheetOpen(c fiber.Ctx) error {
	padName := c.Params("pad")
	jsFilePath := "/js/sheet/assets/sheet.js?v=" + strconv.Itoa(utils.RandomVersionString)
	comp := sheetAsset.SheetIndex(padName, jsFilePath)
	return adaptor.HTTPHandler(templ.Handler(comp))(c)
}
```

> Den exakten Import-Pfad des generierten Pakets (`assets/sheet`) und das Symbol `utils.RandomVersionString` (genutzt in `padFrontend.go:44`) verifizieren.

- [ ] **Step 6: Route registrieren**

In `lib/api/static/init.go`, nach dem `/p/:pad`-Block (Zeile 280-282) einfügen:

```go
	store.C.Get("/s/:pad", func(ctx fiber.Ctx) error {
		return pad2.HandleSheetOpen(ctx)
	})
```

Und im statischen Block (bei Zeile 308-311) ergänzen:

```go
	registerEmbeddedStatic(store.C, "/js/sheet/assets/", "assets/js/sheet/assets", store.UiAssets)
```

- [ ] **Step 7: Tests + Build ausführen — muss bestehen**

Run: `go build ./... && go test ./lib/api/pad/ -run TestHandleSheetOpen -v`
Expected: Build ok, Test PASS.

- [ ] **Step 8: Commit**

```bash
git add assets/sheet/ lib/api/pad/sheetFrontend.go lib/api/pad/sheet_frontend_test.go lib/api/static/init.go
git commit -m "feat(sheet): add /s/:pad route with stub sheet shell handler"
```

---

## Task 6: Vite-`sheet`-Entry + Frontend-Stub

**Files:**
- Create: `ui/src/sheet.entry.ts`
- Modify: `ui/vite.config.ts:6-17`
- Modify: `ui/package.json:8` (build-Script)

- [ ] **Step 1: Entry-Stub schreiben**

Neue Datei `ui/src/sheet.entry.ts`:

```ts
const root = document.getElementById('sheet-root');
if (root) {
  root.textContent = `Spreadsheet editor for "${root.dataset.padName ?? ''}" — coming soon.`;
}
```

- [ ] **Step 2: Vite-Modus ergänzen**

In `ui/vite.config.ts`, im Entry-Mapping (Zeile 6-17) einen Zweig für `sheet` ergänzen (analog zu `pad`):

```ts
  } else if (mode === 'timeslider') {
    entry = { timeslider: path.resolve(__dirname, 'src/timeslider.entry.ts') };
    outDir = '../assets/js/timeslider';
  } else if (mode === 'sheet') {
    entry = { sheet: path.resolve(__dirname, 'src/sheet.entry.ts') };
    outDir = '../assets/js/sheet';
  }
```

- [ ] **Step 3: Build-Script ergänzen**

In `ui/package.json` (Zeile 8) `--mode sheet` an den `build`-Befehl anhängen:

```json
    "build": "tsc && vite build --mode pad && vite build --mode welcome && vite build --mode timeslider && vite build --mode sheet",
```

- [ ] **Step 4: Frontend bauen — Output prüfen**

Run: `cd ui && npm install && npm run build`
Expected: erzeugt `assets/js/sheet/assets/sheet.js` (Pfadparität mit dem Static-Handler aus Task 5/Step 6).

- [ ] **Step 5: Commit**

```bash
git add ui/src/sheet.entry.ts ui/vite.config.ts ui/package.json assets/js/sheet/
git commit -m "feat(sheet): add sheet vite entry and frontend stub bundle"
```

---

## Task 7: Typ-Auswahl auf der Welcome-Seite

**Files:**
- Modify: `assets/welcome/main.templ:22-27`
- Modify: `ui/src/js/index.ts:21-76`
- Test: manuell/E2E (Task 8)

- [ ] **Step 1: Button „Neue Tabelle" im Templ ergänzen**

In `assets/welcome/main.templ`, nach dem bestehenden `<form id="go2Name">`-Block (Zeile 24-27) einen zweiten Button für Tabellen einfügen (Übersetzungs-Key kann vorerst Klartext sein; Lokalisierung später):

```html
				</form>
				<ep-button variant="secondary" id="newSheet">New spreadsheet</ep-button>
```

- [ ] **Step 2: templ generieren**

Run: `go run github.com/a-h/templ/cmd/templ generate ./assets/welcome/`
Expected: aktualisiert `assets/welcome/main_templ.go`.

- [ ] **Step 3: Welcome-Logik erweitern**

In `ui/src/js/index.ts`, in `initWelcomeScreen` (Zeile 49-76) nach der Verdrahtung von `randomButton` ergänzen. Nutzt die vorhandene `randomPadName()`-Funktion (Zeile 21-39):

```ts
  const newSheetButton = byId<HTMLButtonElement>('newSheet');
  newSheetButton.addEventListener('click', () => {
    window.location.href = `s/${randomPadName()}`;
  });
```

> `byId` ist der vorhandene Helfer in derselben Datei; `newSheet` muss zur `id` aus Step 1 passen.

- [ ] **Step 4: Welcome-Bundle bauen**

Run: `cd ui && npm run build`
Expected: aktualisiertes `assets/js/welcome/assets/welcome.js`, kein TS-Fehler.

- [ ] **Step 5: Commit**

```bash
git add assets/welcome/main.templ assets/welcome/main_templ.go ui/src/js/index.ts assets/js/welcome/
git commit -m "feat(welcome): add 'New spreadsheet' option routing to /s/:pad"
```

---

## Task 8: Integrations-/Smoke-Verifikation

**Files:**
- (Verifikation, keine Quelländerung) ggf. Playwright-Test in `playwright/`

- [ ] **Step 1: Gesamtbuild + alle Go-Tests**

Run: `go build ./... && go test ./...`
Expected: Build ok, alle Tests grün.

- [ ] **Step 2: Server starten und manuell prüfen**

Server starten (Standard-Startbefehl des Repos, z.B. `go run .` — bestehende README/Startweise verwenden). Dann:
- `http://localhost:<port>/` öffnen → Button „New spreadsheet" sichtbar.
- Button klicken → Redirect auf `/s/<random>` → Stub-Seite zeigt „Spreadsheet editor … coming soon."
- Ein bestehendes Text-Pad `/p/<name>` öffnen → funktioniert unverändert.

Expected: Sheet-Route lädt die Stub-Shell, Text-Pads unverändert.

- [ ] **Step 3: Persistenz des Typs prüfen (DB)**

Mit der dev-SQLite-DB (`./var/etherpad.db`): nach dem Öffnen einer Tabelle wird der Pad-Datensatz erst in Plan 2 (WebSocket-Connect mit `GetTypedPad`) angelegt. Für Plan 1 reicht der Nachweis über die Unit-Tests aus Task 1/3/4, dass `document_type` korrekt persistiert/geladen wird. (Notiz: In Plan 2 ruft der Sheet-WS-Handler `GetTypedPad(padID, "sheet", authorId)` auf — dann erscheint die Zeile mit `document_type='sheet'`.)

- [ ] **Step 4: Optionaler Playwright-Smoke-Test**

Falls gewünscht, in `playwright/` einen Test ergänzen: Welcome öffnen → „New spreadsheet" klicken → URL matcht `/s/` und `#sheet-root` enthält den Stub-Text. (Muster aus vorhandenen Playwright-Specs übernehmen.)

- [ ] **Step 5: Abschluss-Commit (falls Playwright-Test ergänzt)**

```bash
git add playwright/
git commit -m "test(sheet): playwright smoke test for spreadsheet route"
```

---

## Roadmap: Pläne 2–4 (nach Plan 1, je eigener writing-plans-Zyklus)

Diese werden erst nach dem Landen von Plan 1 im Detail geschrieben, da sie auf dem real entstandenen Code aufsetzen.

**Plan 2 — Spreadsheet-Backend (Modell, Ops, Persistenz, WS-Handler).**
Workbook-/Cell-/Style-Pool-Modell (Spec §3); Op-Typen + Anwendung; Index-Transformation für Struktur-Ops; Tabellen `sheet`, `sheet_cell`, `sheet_op` (Migration 008); eigener Sheet-WebSocket-Message-Handler analog `lib/ws/PadMessageHandler.go`, inkl. Per-Pad-Serialisierung; `GetTypedPad(padID, "sheet", …)` beim Connect; Snapshot/Replay; Konvergenz-Property-Tests. Ergebnis: Server hält ein Workbook, wendet Ops an, persistiert, broadcastet — headless testbar.

**Plan 3 — Frontend-Sheet-Editor (Grid, Collab, Formeln, Präsenz).**
Module `WorkbookState`, `FormulaEngine` (HyperFormula-Wrapper), `SheetCollabClient`, `SheetView` (Grid hinter schmaler Schnittstelle, Spec §5), Toolbar/Formel-Leiste, Remote-Cursor mit Author-Farben (`AuthorDB.ColorId`). Ersetzt den Frontend-Stub aus Plan 1 Task 6. Ergebnis: nutzbare kollaborative Tabelle.

**Plan 4 — xlsx Import/Export.**
`excelize`-Integration (Spec §6): `POST /s/:pad/import`, `GET /s/:pad/export.xlsx`, Mapping excelize↔internes Modell, Roundtrip-Tests, Grenzen-Handling (unbekannte Inhalte überspringen + Warnhinweis). Ergebnis: Excel-Dateien rein und raus.

---

## Self-Review (Planner)

- **Spec-Coverage (Plan 1 / §2):** Dokumenttyp-Feld (Task 1), abwärtskompatibler Default (`"text"`, Task 1 Step 4/6 + Migration-Default Task 2), Routing `/p` unverändert + `/s/:pad` neu (Task 5), Typ-Auswahl auf Welcome (Task 7), Persistenz über `DataStore`-Abstraktion in allen Implementierungen (Task 3, Memory ohne Änderung dokumentiert). §3–§6 sind bewusst Plänen 2–4 zugeordnet (Roadmap).
- **Platzhalter-Scan:** Keine TODO/TBD. Wo eine exakte Signatur erst im Code zu verifizieren ist (Memory-/SQLite-Konstruktor, Manager-Test-Helfer, `utils.RandomVersionString`, templ-Paketname), ist dies explizit als Verifikationshinweis markiert, nicht als „später ausfüllen".
- **Typ-Konsistenz:** Feldname `DocumentType` (Go) / Spaltenname `document_type` (SQL) / JSON `documentType` durchgängig. Spalten-Position „nach `public_status`" konsistent in Struct, `ReadToPadDB`-Scan, allen CreatePad-`Columns`/`Values` und GetPad-SELECTs — kritisch für korrektes Scannen, daher überall identisch platziert. Methodenname `GetTypedPad` einheitlich (Task 4 Def + Task 8 Notiz).
