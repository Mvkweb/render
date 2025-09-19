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
	db, err := bbolt.Open(path, 0600, nil)
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
	timestamp := time.Now().Format(time.RFC3339)
	return d.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(clientName))
		if err != nil {
			return err
		}
		return b.Put([]byte(hashStr), []byte(timestamp))
	})
}

// ClearClientHistory removes all records for a given client.
func (d *DB) ClearClientHistory(clientName string) error {
	return d.db.Update(func(tx *bbolt.Tx) error {
		return tx.DeleteBucket([]byte(clientName))
	})
}

// CleanupOldEntries removes entries from the database that are older than the specified maxAge.
func (d *DB) CleanupOldEntries(maxAge time.Duration) error {
	return d.db.Update(func(tx *bbolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bbolt.Bucket) error {
			// Store timestamps with hashes
			toDelete := [][]byte{}
			b.ForEach(func(k, v []byte) error {
				// Parse timestamp from value
				if timestamp, err := time.Parse(time.RFC3339, string(v)); err == nil {
					if time.Since(timestamp) > maxAge {
						toDelete = append(toDelete, k)
					}
				}
				return nil
			})

			for _, key := range toDelete {
				b.Delete(key)
			}
			return nil
		})
	})
}
