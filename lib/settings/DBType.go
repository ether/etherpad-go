package settings

import (
	"fmt"
	"strings"
)

type IDBType string

const (
	SQLITE IDBType = "sqlite"
	MEMORY IDBType = "memory"
)

func ParseDBType(s string) (IDBType, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "sqlite":
		return SQLITE, nil
	case "memory":
		return MEMORY, nil
	default:
		return "", fmt.Errorf("unknown DB type: %q", s)
	}
}

func (dbType IDBType) String() string {
	return string(dbType)
}
