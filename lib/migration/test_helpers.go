package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

func newTestLogger(t *testing.T) *zap.SugaredLogger {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("logger init failed: %v", err)
	}
	return logger.Sugar()
}

func insertKV(
	t *testing.T,
	db *sql.DB,
	key string,
	value any,
) {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO store (`key`, `value`) VALUES (?, ?)",
		key,
		string(raw),
	)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
}

func insertKVPostgres(
	t *testing.T,
	db *sql.DB,
	key string,
	value any,
) {
	t.Helper()

	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}

	_, err = db.Exec(
		"INSERT INTO store (key, value) VALUES ($1, $2)",
		key,
		string(raw),
	)
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
}

func insertData(t *testing.T, db *sql.DB, insertCall func(*testing.T, *sql.DB, string, any)) {
	t.Helper()

	insertCall(t, db, "sessionstorage:70Ug69Ie7a4YDb2yoFezKCTzzJSgEcCt", map[string]any{
		"cookie": map[string]any{
			"originalMaxAge": 864000000,
			"expires":        "2025-10-18T21:07:25.978Z",
			"secure":         false,
			"httpOnly":       true,
			"path":           "/",
			"sameSite":       "Lax",
		},
		"connections": nil,
	})

	insertCall(t, db, "sessionstorage:kCQ6XKITKqz3Pwc024eDsaIHHadF9Ko2", map[string]any{
		"cookie": map[string]any{
			"originalMaxAge": 864000000,
			"expires":        "2025-08-12T19:09:27.708Z",
			"secure":         false,
			"httpOnly":       true,
			"path":           "/",
			"sameSite":       "Lax",
		},
		"connections": nil,
	})

	insertCall(t, db, "globalAuthor:a.NNm64XqDYtqujZau", map[string]any{
		"colorId":   43,
		"name":      nil,
		"timestamp": int64(1743970312393),
		"padIDs": map[string]any{
			"test": 1,
		},
	})

	insertCall(t, db, "globalAuthor:a.zIsCqGbIhuqFO0dp", map[string]any{
		"colorId":   3,
		"name":      nil,
		"timestamp": int64(1754165107261),
		"padIDs": map[string]any{
			"bBA-oJE3Wu0mAwE_szE7": 1,
		},
	})

	insertCall(t, db, "globalAuthor:a.kpvBkCBIU7ZJPhhz", map[string]any{
		"colorId":   61,
		"name":      nil,
		"timestamp": int64(1769293372852),
		"padIDs": map[string]any{
			"testpad": 1,
			"test":    1,
		},
	})

	insertCall(t, db, "pad:test", map[string]any{
		"atext": map[string]any{
			"text":    "HalloEtherpad\n",
			"attribs": "*1+d|1+1",
		},
		"pool": map[string]any{
			"numToAttrib": map[string]any{
				"0": []any{"author", "a.NNm64XqDYtqujZau"},
				"1": []any{"author", "a.kpvBkCBIU7ZJPhhz"},
			},
			"nextNum": 2,
		},
		"head":           5,
		"chatHead":       1,
		"publicStatus":   false,
		"savedRevisions": []any{},
	})

	insertCall(t, db, "pad:testpad", map[string]any{
		"atext": map[string]any{
			"text":    "hallo das ist ein pad\n",
			"attribs": "*0+l|1+1",
		},
		"pool": map[string]any{
			"numToAttrib": map[string]any{
				"0": []any{"author", "a.kpvBkCBIU7ZJPhhz"},
			},
			"nextNum": 1,
		},
		"head":         7,
		"chatHead":     -1,
		"publicStatus": false,
		"savedRevisions": []any{
			map[string]any{
				"revNum":    7,
				"savedById": "a.kpvBkCBIU7ZJPhhz",
				"label":     "Revision 7",
				"timestamp": int64(1769268322737),
				"id":        "1157f7c7f568c04236e3",
			},
		},
	})

	insertCall(t, db, "pad:test:revs:0", map[string]any{
		"changeset": "Z:1>k+k$Welcome to Etherpad!",
		"meta": map[string]any{
			"author":    "a.NNm64XqDYtqujZau",
			"timestamp": int64(1743970312579),
			"pool": map[string]any{
				"numToAttrib": map[string]any{
					"0": []any{"author", "a.NNm64XqDYtqujZau"},
				},
				"attribToNum": map[string]any{
					"author,a.NNm64XqDYtqujZau": 0,
				},
				"nextNum": 1,
			},
			"atext": map[string]any{
				"text":    "Welcome to Etherpad!\n",
				"attribs": "|1+l",
			},
		},
	})

	insertCall(t, db, "pad:test:revs:1", map[string]any{
		"changeset": "Z:l<j-k*1+1$H",
		"meta": map[string]any{
			"author":    "a.kpvBkCBIU7ZJPhhz",
			"timestamp": int64(1769293369504),
		},
	})

	insertCall(t, db, "pad:test:chat:0", map[string]any{
		"text":     "hallo eine Nachricht",
		"authorId": "a.kpvBkCBIU7ZJPhhz",
		"time":     int64(1769290382564),
	})

	insertCall(t, db, "pad:test:chat:1", map[string]any{
		"text":     "Eine weiter",
		"authorId": "a.kpvBkCBIU7ZJPhhz",
		"time":     int64(1769290385972),
	})

	insertCall(t, db, "pad2readonly:testpad", "r.1d99de0f761b68fc6b2e5b8b224f250f")
	insertCall(t, db, "readonly2pad:r.1d99de0f761b68fc6b2e5b8b224f250f", "testpad")

	insertCall(t, db, "token2author:t.b7xN2ym2xeNwB5l3YFY9", "a.kpvBkCBIU7ZJPhhz")
}

func startMigratorPipeline(t *testing.T, oldDB *SQLDatabase, newDB db.DataStore) {
	t.Helper()

	logger := newTestLogger(t)

	m := NewMigrator(oldDB, newDB, logger)

	if err := m.MigrateAuthors(); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	assert.NoError(t, m.MigratePads())

	assert.NoError(t, m.MigrateRevisions())
	assert.NoError(t, m.MigratePadChats())

	assert.NoError(t, m.MigratePad2Readonly())

	assert.NoError(t, m.MigrateToken2Author())

	author, err := newDB.GetAuthor("a.kpvBkCBIU7ZJPhhz")
	if err != nil {
		t.Fatalf("author not found: %v", err)
	}

	if author.ColorId != "#e9afc2" {
		t.Fatalf("expected PostgresUser, got %s", author.ColorId)
	}

	savedPad, err := newDB.GetPad("test")
	assert.NoError(t, err)
	if savedPad.Head != 5 {
		t.Fatalf("expected RevNum 5, got %d", savedPad.Head)
	}
	if savedPad.ChatHead != 1 {
		t.Fatalf("expected ChatHead 1, got %d", savedPad.ChatHead)
	}
	if savedPad.PublicStatus != false {
		t.Fatalf("expected PublicStatus false, got %v", savedPad.PublicStatus)
	}
	if len(savedPad.SavedRevisions) != 0 {
		t.Fatalf("expected 0 SavedRevisions, got %d", len(savedPad.SavedRevisions))
	}
	if savedPad.ATextText != "HalloEtherpad\n" {
		t.Fatalf("unexpected AText: %s", savedPad.ATextText)
	}

	revisionsSaved, err := newDB.GetRevisions("test", 0, 1)
	assert.NoError(t, err)
	if len(*revisionsSaved) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(*revisionsSaved))
	}
	if (*revisionsSaved)[0].Changeset != "Z:1>k+k$Welcome to Etherpad!" {
		t.Fatalf("unexpected changeset: %s", (*revisionsSaved)[0].Changeset)
	}
	if (*revisionsSaved)[1].Changeset != "Z:l<j-k*1+1$H" {
		t.Fatalf("unexpected changeset: %s", (*revisionsSaved)[1].Changeset)
	}

	if (*revisionsSaved)[0].AuthorId == nil || *(*revisionsSaved)[0].AuthorId != "a.NNm64XqDYtqujZau" {
		t.Fatalf("unexpected author: %v", (*revisionsSaved)[0].AuthorId)
	}

	if (*revisionsSaved)[1].AuthorId == nil || *(*revisionsSaved)[1].AuthorId != "a.kpvBkCBIU7ZJPhhz" {
		t.Fatalf("unexpected author: %v", (*revisionsSaved)[1].AuthorId)
	}

	chatMessages, err := newDB.GetChatsOfPad("test", 0, 1)
	assert.NoError(t, err)
	if len(*chatMessages) != 2 {
		t.Fatalf("expected 2 chat messages, got %d", len(*chatMessages))
	}
	if (*chatMessages)[0].Message != "hallo eine Nachricht" {
		t.Fatalf("unexpected chat message: %s", (*chatMessages)[0].Message)
	}
	if (*chatMessages)[0].AuthorId == nil || *(*chatMessages)[0].AuthorId != "a.kpvBkCBIU7ZJPhhz" {
		t.Fatalf("unexpected chat author: %v", (*chatMessages)[0].AuthorId)
	}

	if (*chatMessages)[1].Message != "Eine weiter" {
		t.Fatalf("unexpected chat message: %s", (*chatMessages)[1].Message)
	}
	if (*chatMessages)[1].AuthorId == nil || *(*chatMessages)[1].AuthorId != "a.kpvBkCBIU7ZJPhhz" {
		t.Fatalf("unexpected chat author: %v", (*chatMessages)[1].AuthorId)
	}

	readonlyPad, err := newDB.GetReadonlyPad("testpad")
	assert.NoError(t, err)
	if *readonlyPad != "r.1d99de0f761b68fc6b2e5b8b224f250f" {
		t.Fatalf("unexpected readonly pad: %s", *readonlyPad)
	}

	padFromReadonly, err := newDB.GetPadByReadOnlyId("r.1d99de0f761b68fc6b2e5b8b224f250f")
	assert.NoError(t, err)
	if *padFromReadonly != "testpad" {
		t.Fatalf("unexpected pad from readonly: %s", *padFromReadonly)
	}

}

func startMySQL(t *testing.T) (*sql.DB, func()) {
	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "mysql:9.6",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root",
			"MYSQL_DATABASE":      "etherpad",
		},
		WaitingFor: wait.ForLog("ready for connections"),
	}

	container, err := testcontainers.GenericContainer(ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
	if err != nil {
		t.Fatal(err)
	}

	host, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, "3306")

	dsn := fmt.Sprintf(
		"root:root@tcp(%s:%s)/etherpad?parseTime=true",
		host, port.Port(),
	)

	deadline := time.Now().Add(2 * time.Minute)
	for time.Now().Before(deadline) {
		db, err := sql.Open("mysql", dsn)
		if err == nil {
			err = db.Ping()
			if err == nil {
				db.Close()
				break
			}
			db.Close()
		}
		time.Sleep(1 * time.Second)
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.Exec(`
		CREATE TABLE store (
			` + "`key`" + ` VARCHAR(255) PRIMARY KEY,
			` + "`value`" + ` LONGTEXT NOT NULL
		)
	`)
	if err != nil {
		t.Fatal(err)
	}

	cleanup := func() {
		db.Close()
		container.Terminate(ctx)
	}

	return db, cleanup
}

func startPostgres(t *testing.T) (*sql.DB, func()) {
	t.Helper()

	ctx := context.Background()

	req := testcontainers.ContainerRequest{
		Image:        "postgres:16-alpine",
		ExposedPorts: []string{"5432/tcp"},
		Env: map[string]string{
			"POSTGRES_USER":     "etherpad",
			"POSTGRES_PASSWORD": "etherpad",
			"POSTGRES_DB":       "etherpad",
		},
		WaitingFor: wait.
			ForListeningPort("5432/tcp").
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}

	port, err := container.MappedPort(ctx, "5432")
	if err != nil {
		t.Fatal(err)
	}

	dsn := fmt.Sprintf(
		"postgres://etherpad:etherpad@%s:%s/etherpad?sslmode=disable",
		host,
		port.Port(),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open postgres connection: %v", err)
	}

	// Wait until DB is actually ready
	for i := 0; i < 10; i++ {
		if err := db.Ping(); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Create Etherpad store table
	_, err = db.Exec(`
		CREATE TABLE store (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		);
	`)
	if err != nil {
		db.Close()
		container.Terminate(ctx)
		t.Fatalf("failed to create store table: %v", err)
	}

	cleanup := func() {
		db.Close()
		_ = container.Terminate(ctx)
	}

	return db, cleanup
}
