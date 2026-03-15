package log

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"

	"github.com/gjovanovicst/auth_api/pkg/dto"
)

// csvHeaders defines the column order for CSV exports.
var csvHeaders = []string{
	"id",
	"user_id",
	"event_type",
	"timestamp",
	"ip_address",
	"user_agent",
	"severity",
	"is_anomaly",
	"details",
}

// WriteCSV encodes a slice of ActivityLogResponse as CSV into w.
// The first row is the header. The "details" column contains the JSON
// representation of the structured details object.
func WriteCSV(w io.Writer, logs []dto.ActivityLogResponse) error {
	cw := csv.NewWriter(w)

	if err := cw.Write(csvHeaders); err != nil {
		return fmt.Errorf("csv: write header: %w", err)
	}

	for _, entry := range logs {
		detailsStr := "{}"
		if entry.Details != nil {
			if b, err := json.Marshal(entry.Details); err == nil {
				detailsStr = string(b)
			}
		}

		row := []string{
			entry.ID,
			entry.UserID,
			entry.EventType,
			entry.Timestamp,
			entry.IPAddress,
			entry.UserAgent,
			entry.Severity,
			fmt.Sprintf("%t", entry.IsAnomaly),
			detailsStr,
		}

		if err := cw.Write(row); err != nil {
			return fmt.Errorf("csv: write row: %w", err)
		}
	}

	cw.Flush()
	return cw.Error()
}
