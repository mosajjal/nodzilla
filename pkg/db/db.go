// Package db prvoides a unified interface to add, modify, and delete newly obsereved domains
// from any database.
package db

import "time"

// Entry represents a single domain entry in the database
type Entry struct {
	// Domain name
	Domain string `json:"domain"`
	// Date of first observation
	RegistrationDate time.Time `json:"registration_date"`
}

// NodDB is the interface that must be implemented by any database that is to be used
type NodDB interface {
	// Open opens a connection to the database
	Open() error
	// Close closes the connection to the database
	Close() error
	// Add adds a new domain to the database
	Add(Entry) error
	// AddMany adds multiple new domains to the database
	AddMany([]Entry) error
	// Delete removes a domain from the database
	Delete(string) error
	// DeleteMany removes multiple domains from the database
	DeleteMany([]string) error
	// Query returns the date of first observation for a given domain
	Query(string) (Entry, error)
	// QueryMany returns the date of first observation for multiple domains
	QueryMany([]string) ([]Entry, error)
}
