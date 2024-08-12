package session

import (
	"github.com/ether/etherpad-go/lib/db"
	"time"
)

type SessionDatabase struct {
}

func (s SessionDatabase) Get(key string) ([]byte, error) {
	println(key)
	return nil, nil
}

func (s SessionDatabase) Set(key string, val []byte, exp time.Duration) error {
	println(key, val, exp)
	//TODO implement me
	return nil
}

func (s SessionDatabase) Delete(key string) error {

	return nil
}

func (s SessionDatabase) Reset() error {
	return nil
}

func (s SessionDatabase) Close() error {
	return nil
}

func NewSessionDatabase(db *db.DataStore) SessionDatabase {
	return SessionDatabase{}
}
