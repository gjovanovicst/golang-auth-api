package user

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/mail"
	"strings"

	"github.com/gjovanovicst/auth_api/pkg/dto"
)

// ParseCSVImport reads a CSV from r and returns valid rows plus per-row errors.
//
// Expected header row: email, name, first_name, last_name, locale (case-insensitive).
// Column order is flexible — unknown columns are ignored.
// The first row is always treated as the header.
// Rows with empty or invalid email are collected as errors and excluded from the
// returned rows slice.
func ParseCSVImport(r io.Reader) ([]dto.UserImportRow, []dto.UserImportRowError) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // allow variable column count

	var rows []dto.UserImportRow
	var errs []dto.UserImportRowError

	// Read header
	header, err := reader.Read()
	if err != nil {
		errs = append(errs, dto.UserImportRowError{Row: 1, Error: "failed to read CSV header: " + err.Error()})
		return rows, errs
	}

	// Build column index map (case-insensitive)
	colIndex := make(map[string]int, len(header))
	for i, h := range header {
		colIndex[strings.ToLower(strings.TrimSpace(h))] = i
	}

	col := func(record []string, name string) string {
		idx, ok := colIndex[name]
		if !ok || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	rowNum := 1 // header was row 1
	for {
		rowNum++
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			errs = append(errs, dto.UserImportRowError{Row: rowNum, Error: "CSV read error: " + err.Error()})
			continue
		}

		email := col(record, "email")
		if errMsg := validateImportEmail(email); errMsg != "" {
			errs = append(errs, dto.UserImportRowError{Row: rowNum, Email: email, Error: errMsg})
			continue
		}

		rows = append(rows, dto.UserImportRow{
			Email:     strings.ToLower(email),
			Name:      col(record, "name"),
			FirstName: col(record, "first_name"),
			LastName:  col(record, "last_name"),
			Locale:    col(record, "locale"),
		})
	}

	return rows, errs
}

// ParseJSONImport reads a JSON payload from r and returns valid rows plus per-row errors.
//
// Accepts two formats:
//   - Top-level array:  [{...}, ...]
//   - Object with key:  {"users": [{...}, ...]}
//
// Each object must have at minimum an "email" field.
// Optional fields: name, first_name, last_name, locale.
func ParseJSONImport(r io.Reader) ([]dto.UserImportRow, []dto.UserImportRowError) {
	var rows []dto.UserImportRow
	var errs []dto.UserImportRowError

	data, err := io.ReadAll(r)
	if err != nil {
		errs = append(errs, dto.UserImportRowError{Row: 0, Error: "failed to read JSON: " + err.Error()})
		return rows, errs
	}
	data = []byte(strings.TrimSpace(string(data)))

	// Try top-level array first, then object wrapper.
	var rawRows []dto.UserImportRow
	if len(data) > 0 && data[0] == '[' {
		if err := json.Unmarshal(data, &rawRows); err != nil {
			errs = append(errs, dto.UserImportRowError{Row: 0, Error: "invalid JSON array: " + err.Error()})
			return rows, errs
		}
	} else {
		var wrapper struct {
			Users []dto.UserImportRow `json:"users"`
		}
		if err := json.Unmarshal(data, &wrapper); err != nil {
			errs = append(errs, dto.UserImportRowError{Row: 0, Error: "invalid JSON: expected array or {\"users\":[...]} object: " + err.Error()})
			return rows, errs
		}
		rawRows = wrapper.Users
	}

	for i, row := range rawRows {
		rowNum := i + 1
		email := strings.TrimSpace(row.Email)
		if errMsg := validateImportEmail(email); errMsg != "" {
			errs = append(errs, dto.UserImportRowError{Row: rowNum, Email: email, Error: errMsg})
			continue
		}
		rows = append(rows, dto.UserImportRow{
			Email:     strings.ToLower(email),
			Name:      strings.TrimSpace(row.Name),
			FirstName: strings.TrimSpace(row.FirstName),
			LastName:  strings.TrimSpace(row.LastName),
			Locale:    strings.TrimSpace(row.Locale),
		})
	}

	return rows, errs
}

// validateImportEmail returns a non-empty error string if email is blank or malformed.
func validateImportEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return "email is required"
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Sprintf("invalid email address: %q", email)
	}
	return ""
}
