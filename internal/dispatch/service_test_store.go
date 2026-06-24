package dispatch

import (
	"github.com/deon7769/deonclaw/internal/store"
)

type testStoreAdapter struct {
	*store.SQLiteStore
}

func openTestStoreAdapter(path string) (*testStoreAdapter, error) {
	db, err := store.OpenSQLite(path)
	if err != nil {
		return nil, err
	}
	return &testStoreAdapter{SQLiteStore: db}, nil
}
