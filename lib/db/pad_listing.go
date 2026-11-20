package db

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// PadListing narrows ListPadIDs.
type PadListing struct {
	// After: only ids after this one (the last id of the page before).
	After string
	// Prefix: only ids that start with it.
	Prefix string
	// Contains: only ids that contain it, case ignored.
	Contains string
	// DocumentType: only pads of this type ("text", "sheet"); "" for all.
	DocumentType string
	// Limit: at most this many ids (1 to 1000).
	Limit int
}

// PadListingMethods list pad ids page by page, in the database, so a
// caller never reads every pad of a large instance at once.
type PadListingMethods interface {
	// ListPadIDs returns pad ids in order, at most l.Limit of them.
	ListPadIDs(l PadListing) ([]string, error)
}

const maxPadListing = 1000

func (l PadListing) limit() int {
	if l.Limit <= 0 || l.Limit > maxPadListing {
		return maxPadListing
	}
	return l.Limit
}

// likeLiteral escapes the wildcards of LIKE with '!' (likeEscape also
// lowers and wraps).
func likeLiteral(s string) string {
	return strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(s)
}

// padListingSQL builds the query; arg names the n-th placeholder.
func padListingSQL(l PadListing, arg func(n int) string) (string, []any) {
	var where []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", arg(len(args))))
	}
	add("id > ?", l.After)
	if l.Prefix != "" {
		// Not LIKE: SQLite's LIKE ignores case.
		add("SUBSTR(id, 1, "+strconv.Itoa(utf8.RuneCountInString(l.Prefix))+") = ?", l.Prefix)
	}
	if l.Contains != "" {
		add("LOWER(id) LIKE ? ESCAPE '!'", "%"+likeLiteral(strings.ToLower(l.Contains))+"%")
	}
	if l.DocumentType == "text" {
		// Pads from before the column read as text.
		add("(document_type = ? OR document_type = '')", "text")
	} else if l.DocumentType != "" {
		add("document_type = ?", l.DocumentType)
	}
	args = append(args, l.limit())
	q := "SELECT id FROM pad WHERE " + strings.Join(where, " AND ") + " ORDER BY id LIMIT " + arg(len(args))
	return q, args
}

func listPadIDs(r sqlRunner, l PadListing) ([]string, error) {
	q, args := padListingSQL(l, func(int) string { return "?" })
	rows, err := r.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (d SQLiteDB) ListPadIDs(l PadListing) ([]string, error) { return listPadIDs(d.sqlDB, l) }
func (d MysqlDB) ListPadIDs(l PadListing) ([]string, error)  { return listPadIDs(d.sqlDB, l) }

func (d PostgresDB) ListPadIDs(l PadListing) ([]string, error) {
	q, args := padListingSQL(l, func(n int) string { return "$" + strconv.Itoa(n) })
	rows, err := d.pool.Query(context.Background(), q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (m *MemoryDataStore) ListPadIDs(l PadListing) ([]string, error) {
	m.mu.Lock()
	ids := make([]string, 0, len(m.padStore))
	contains := strings.ToLower(l.Contains)
	for id, p := range m.padStore {
		typ := p.DocumentType
		if typ == "" {
			typ = "text"
		}
		if id <= l.After || !strings.HasPrefix(id, l.Prefix) || !strings.Contains(strings.ToLower(id), contains) ||
			(l.DocumentType != "" && typ != l.DocumentType) {
			continue
		}
		ids = append(ids, id)
	}
	m.mu.Unlock()
	sort.Strings(ids)
	return ids[:min(len(ids), l.limit())], nil
}
