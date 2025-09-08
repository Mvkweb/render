package database

import (
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

var pinBucket = []byte("pins")

// DB is a wrapper around a bbolt database.
type DB struct {
	db *bbolt.DB
}

// Open opens a database file at the given path.
func Open(path string) (*DB, error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(pinBucket)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	return &DB{db: db}, nil
}

// Close closes the database.
func (d *DB) Close() error {
	return d.db.Close()
}

// Exists checks if a key exists in the database.
func (d *DB) Exists(id string) (bool, error) {
	var exists bool
	err := d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(pinBucket)
		exists = b.Get([]byte(id)) != nil
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to check for key: %w", err)
	}
	return exists, nil
}

// Add adds a key to the database.
func (d *DB) Add(id string) error {
	return d.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(pinBucket)
		return b.Put([]byte(id), []byte("1"))
	})
}
