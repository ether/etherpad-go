package session

import (
	"github.com/ether/etherpad-go/lib/db"
	"time"
)

type Database struct {
}

func (s Database) Get(key string) ([]byte, error) {
	println(key)
	return nil, nil
}

func (s Database) Set(key string, val []byte, exp time.Duration) error {
	println(key, val, exp)
	//TODO implement me
	return nil
}

func (s Database) Delete(key string) error {

	return nil
}

func (s Database) Reset() error {
	return nil
}

func (s Database) Close() error {
	return nil
}

func NewSessionDatabase(db *db.DataStore) Database {
	return Database{}
}
