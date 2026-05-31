// Package geoip provides IP-to-location lookups via a MaxMind-format database file.
package geoip

import (
	"fmt"
	"net"
	"os"

	"github.com/oschwald/maxminddb-golang"
)

// Location holds the geographic information resolved for an IP address.
type Location struct {
	CountryCode string
	CountryName string
	City        string
	Latitude    float64
	Longitude   float64
}

// mmRecord is the struct used to decode MaxMind DB records.
// Fields are mapped via maxminddb struct tags.
type mmRecord struct {
	Country struct {
		ISOCode string            `maxminddb:"iso_code"`
		Names   map[string]string `maxminddb:"names"`
	} `maxminddb:"country"`
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

// DB wraps a MaxMind database reader.
type DB struct {
	reader *maxminddb.Reader
}

// Open opens a MaxMind-format database file at the given path.
func Open(path string) (*DB, error) {
	r, err := maxminddb.Open(path)
	if err != nil {
		return nil, fmt.Errorf("geoip: open %s: %w", path, err)
	}
	return &DB{reader: r}, nil
}

// OpenOptional opens the database at path, returning nil, nil when the path is empty
// or the file does not exist. All other errors are returned.
func OpenOptional(path string) (*DB, error) {
	if path == "" {
		return nil, nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	return Open(path)
}

// Lookup resolves an IP address string to a geographic Location.
// Returns an error if ip cannot be parsed or the DB lookup fails.
func (d *DB) Lookup(ip string) (*Location, error) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return nil, fmt.Errorf("geoip: invalid IP address: %q", ip)
	}

	var record mmRecord
	if err := d.reader.Lookup(parsed, &record); err != nil {
		return nil, fmt.Errorf("geoip: lookup %s: %w", ip, err)
	}

	loc := &Location{
		CountryCode: record.Country.ISOCode,
		Latitude:    record.Location.Latitude,
		Longitude:   record.Location.Longitude,
	}

	if name, ok := record.Country.Names["en"]; ok {
		loc.CountryName = name
	}
	if name, ok := record.City.Names["en"]; ok {
		loc.City = name
	}

	return loc, nil
}

// Close releases the resources held by the database reader.
func (d *DB) Close() error {
	return d.reader.Close()
}
