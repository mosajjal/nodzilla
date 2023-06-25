package db

import (
	"encoding/json"
	"time"

	"github.com/cockroachdb/pebble"
)

// PebbleDB is a wrapper around a Pebble database.
type PebbleDB struct {
	Path string
	DB   *pebble.DB
}

// NewPebbleDB creates a new PebbleDB instance.
// Note that the database is not opened until Open() is called.
func NewPebbleDB(path string) *PebbleDB {
	return &PebbleDB{Path: path}
}

// Value is what gets saved inside the K/V store. The key is the FQDN
type Value struct {
	FirstObserved int64 `json:"first_observed"`
	TimeAdded     int64 `json:"now"`
}

// Open opens the database located at path.
func (db *PebbleDB) Open() error {
	var err error
	db.DB, err = pebble.Open(db.Path, &pebble.Options{})
	return err
}

// Close closes the database.
func (db *PebbleDB) Close() error {
	return db.DB.Close()
}

// Add adds an entry to the database.
func (db *PebbleDB) Add(entry Entry) error {
	// as a value, we will also store the "Now" timestamp
	value := Value{
		FirstObserved: entry.RegistrationDate.Unix(),
		TimeAdded:     time.Now().Unix(),
	}
	// store the JSON representation of the entry
	j, _ := json.Marshal(value)
	return db.DB.Set([]byte(entry.Domain), j, pebble.Sync)
}

// AddMany adds many entries to the database.
func (db *PebbleDB) AddMany(entries []Entry) error {
	batch := db.DB.NewBatch()
	for _, entry := range entries {
		// as a value, we will also store the "Now" timestamp
		value := Value{
			FirstObserved: entry.RegistrationDate.Unix(),
			TimeAdded:     time.Now().Unix(),
		}
		// store the JSON representation of the entry
		j, _ := json.Marshal(value)
		batch.Set([]byte(entry.Domain), j, pebble.Sync)
	}
	return batch.Commit(pebble.Sync)
}

// Delete deletes an entry from the database.
func (db *PebbleDB) Delete(domain string) error {
	return db.DB.Delete([]byte(domain), pebble.Sync)
}

// DeleteMany deletes many entries from the database.
func (db *PebbleDB) DeleteMany(domains []string) error {
	batch := db.DB.NewBatch()
	for _, domain := range domains {
		batch.Delete([]byte(domain), pebble.Sync)
	}
	return batch.Commit(pebble.Sync)
}

// Query queries the database for entries matching the given query.
func (db *PebbleDB) Query(domain string) (Entry, error) {
	v, _, err := db.DB.Get([]byte(domain))
	// check specifically for "not found" errors
	if err == pebble.ErrNotFound {
		// return epoch 0 time
		return Entry{Domain: domain, RegistrationDate: time.Unix(0, 0)}, nil
	}
	if err != nil {
		return Entry{}, err
	}
	// unmarshal the value
	var value Value
	err = json.Unmarshal(v, &value)
	if err != nil {
		return Entry{}, err
	}
	// return the first observed time
	return Entry{Domain: domain, RegistrationDate: time.Unix(value.FirstObserved, 0)}, nil
}

// QueryMany queries the database for entries matching the given query.
func (db *PebbleDB) QueryMany(domains []string) ([]Entry, error) {
	entries := make([]Entry, 0, len(domains))
	for _, domain := range domains {
		entry, err := db.Query(domain)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
