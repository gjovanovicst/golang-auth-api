package geoip

import (
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

// GeoInfo contains geographic information resolved from an IP address
type GeoInfo struct {
	Country     string  `json:"country"`      // ISO 3166-1 alpha-2 code (e.g., "US")
	CountryName string  `json:"country_name"` // Full country name (e.g., "United States")
	City        string  `json:"city"`         // City name (e.g., "San Francisco")
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
}

// String returns a human-readable location string
func (g *GeoInfo) String() string {
	if g == nil {
		return "Unknown location"
	}
	if g.City != "" && g.CountryName != "" {
		return fmt.Sprintf("%s, %s", g.City, g.CountryName)
	}
	if g.CountryName != "" {
		return g.CountryName
	}
	return "Unknown location"
}

// Service provides GeoIP lookup capabilities using a MaxMind GeoLite2 database.
// If no database is configured, the service operates in disabled mode where all
// lookups return nil gracefully.
type Service struct {
	reader    *geoip2.Reader
	available bool
	mu        sync.RWMutex
}

// NewService creates a new GeoIP service. If dbPath is empty or the database file
// cannot be opened, the service operates in disabled mode (all lookups return nil).
func NewService(dbPath string) *Service {
	s := &Service{
		available: false,
	}

	if dbPath == "" {
		log.Println("GeoIP: No database path configured (GEOIP_DB_PATH). GeoIP lookups disabled.")
		return s
	}

	reader, err := geoip2.Open(dbPath)
	if err != nil {
		log.Printf("GeoIP: Failed to open database at %s: %v. GeoIP lookups disabled.", dbPath, err)
		return s
	}

	s.reader = reader
	s.available = true
	log.Printf("GeoIP: Database loaded successfully from %s", dbPath)
	return s
}

// IsAvailable returns true if the GeoIP database is loaded and lookups will work.
func (s *Service) IsAvailable() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.available
}

// Lookup resolves an IP address to geographic information.
// Returns nil if the service is disabled, the IP is invalid, or the IP is not found
// in the database. Private/loopback IPs always return nil.
func (s *Service) Lookup(ipStr string) *GeoInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.available || s.reader == nil {
		return nil
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil
	}

	// Skip private, loopback, and link-local addresses
	if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return nil
	}

	record, err := s.reader.City(ip)
	if err != nil {
		return nil
	}

	info := &GeoInfo{
		Country:     record.Country.IsoCode,
		CountryName: record.Country.Names["en"],
		Latitude:    record.Location.Latitude,
		Longitude:   record.Location.Longitude,
	}

	if city, ok := record.City.Names["en"]; ok {
		info.City = city
	}

	return info
}

// LookupCountry is a convenience method that returns just the ISO country code for an IP.
// Returns empty string if lookup fails or service is disabled.
func (s *Service) LookupCountry(ipStr string) string {
	info := s.Lookup(ipStr)
	if info == nil {
		return ""
	}
	return info.Country
}

// Close releases the GeoIP database resources.
func (s *Service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.reader != nil {
		err := s.reader.Close()
		s.reader = nil
		s.available = false
		return err
	}
	return nil
}
