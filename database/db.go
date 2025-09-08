package database

import (
	"fmt"
	"time"

	"go.etcd.io/bbolt"
)

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
	return &DB{db: db}, nil
}

// Close closes the database.
func (d *DB) Close() error {
	return d.db.Close()
}

// HasClientSeenImage checks if a client has already seen an image with the given hash.
func (d *DB) HasClientSeenImage(clientName string, hash uint64) (bool, error) {
	var exists bool
	hashStr := fmt.Sprintf("%d", hash)
	err := d.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clientName))
		if b == nil {
			return nil // Bucket doesn't exist, so the image hasn't been seen
		}
		exists = b.Get([]byte(hashStr)) != nil
		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to check for hash: %w", err)
	}
	return exists, nil
}

// MarkImageAsSeen marks an image as seen for a specific client.
func (d *DB) MarkImageAsSeen(clientName string, hash uint64) error {
	hashStr := fmt.Sprintf("%d", hash)
	return d.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(clientName))
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		return b.Put([]byte(hashStr), []byte("1"))
	})
}
