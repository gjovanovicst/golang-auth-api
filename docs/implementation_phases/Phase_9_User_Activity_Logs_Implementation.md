## Phase 9: User Activity Logs Implementation

This phase focuses on implementing a robust and scalable system for tracking user activity within the application. User activity logs are crucial for security auditing, compliance, debugging, and understanding user behavior. The design will prioritize efficiency, data integrity, and smart retention policies to manage storage effectively.

### 9.1 Database Choice for Activity Logs

While PostgreSQL is used for core user data, user activity logs, being time-series data with high write volumes and often sequential reads, can benefit from specialized database solutions. For optimal performance and scalability, a dedicated time-series database or a highly optimized relational table is recommended.

**Option 1: PostgreSQL (Optimized Table)**

If keeping the technology stack lean is a priority, PostgreSQL can still be a viable option with proper indexing and partitioning. This approach leverages existing infrastructure and knowledge.

**Pros:**
-   No new database technology to manage.
-   Familiarity with GORM for interaction.

**Cons:**
-   May require manual partitioning for very high volumes.
-   Querying large historical datasets can be slower than specialized solutions.

**Optimization Strategies for PostgreSQL:**
-   **Indexing:** Create indexes on `UserID`, `Timestamp`, and `EventType` for efficient querying.
-   **Partitioning:** Implement time-based table partitioning (e.g., by month or year) to improve query performance and facilitate easier data archival/deletion.
-   **Denormalization:** Store all necessary context directly in the log entry to avoid joins during common queries.

**Option 2: Time-Series Database (e.g., InfluxDB, TimescaleDB)**

Time-series databases are purpose-built for handling large volumes of timestamped data, offering superior write and query performance for logging scenarios.

**Pros:**
-   High write throughput and optimized storage for time-series data.
-   Fast queries for time-based ranges and aggregations.
-   Built-in data retention policies and downsampling capabilities.

**Cons:**
-   Introduces a new database technology to the stack, increasing operational complexity.
-   Requires a separate client library and potentially a different ORM/data access pattern.

**Recommendation:** For this project, given the existing PostgreSQL setup, we will initially proceed with an **optimized PostgreSQL table** for user activity logs. This minimizes additional complexity while still allowing for good performance with proper design. If log volumes become exceptionally high (e.g., millions of events per day), migrating to a dedicated time-series database can be considered as a future enhancement.

### 9.2 Activity Log Schema

The `ActivityLog` model will capture essential details about each user action.

| Field Name      | Data Type       | Description                                       | Constraints/Notes                                |
|-----------------|-----------------|---------------------------------------------------|--------------------------------------------------|
| `ID`            | `UUID`          | Unique identifier for the log entry.              | Primary Key, Auto-generated, Indexed             |
| `UserID`        | `UUID`          | ID of the user performing the action.             | Indexed, Foreign Key to `User` (optional, for performance) |
| `EventType`     | `string`        | Type of activity (e.g., `LOGIN`, `LOGOUT`, `PASSWORD_CHANGE`, `2FA_ENABLE`). | Indexed                                          |
| `Timestamp`     | `time.Time`     | When the activity occurred.                       | Indexed, Primary for time-series queries         |
| `IPAddress`     | `string`        | IP address from which the action originated.      |                                                  |
| `UserAgent`     | `string`        | User-Agent string of the client.                  |                                                  |
| `Details`       | `jsonb`         | JSONB field for flexible, structured additional details (e.g., `{"old_email": "a@b.com", "new_email": "x@y.com"}`). |                                                  |

**GORM Model Definition (GoLang):**

```go
type ActivityLog struct {
    ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
    UserID    uuid.UUID `gorm:"index" json:"user_id"` // Consider not making it a foreign key constraint for performance if high volume
    EventType string    `gorm:"index;not null" json:"event_type"`
    Timestamp time.Time `gorm:"index;not null" json:"timestamp"`
    IPAddress string    `json:"ip_address"`
    UserAgent string    `json:"user_agent"`
    Details   json.RawMessage `gorm:"type:jsonb" json:"details"` // Use json.RawMessage for flexible JSONB
}
```

### 9.3 Log Ingestion and Asynchronous Logging

To avoid impacting the performance of critical API operations, activity logging should be asynchronous.

**Mechanism:**
1.  **Log Service:** Create a dedicated `internal/log/service.go` that exposes a method like `LogActivity(userID, eventType, ipAddress, userAgent, details)`. This service will be responsible for writing logs.
2.  **Go Routines and Channels:** When an event occurs (e.g., user login), the handler or service will send the log data to a channel. A separate Go routine will continuously read from this channel and write the logs to the database.
3.  **Error Handling:** Implement robust error handling for logging. If a log cannot be written, it should ideally be retried or sent to an error queue/dead-letter queue to prevent data loss, without blocking the main request flow.

**Example `internal/log/service.go`:**

```go
package log

import (
	"context"
	"encoding/json"
	"log"
	

