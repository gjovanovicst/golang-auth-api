package health

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	goredis "github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"
)

// ----------------------------------------------------------------------------
// Prometheus registry and metrics
// ----------------------------------------------------------------------------

var (
	registry *prometheus.Registry

	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, path, and status code.",
		},
		[]string{"method", "path", "status_code"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency distributions by method and path.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"method", "path"},
	)

	authLoginSuccessTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_login_success_total",
			Help: "Total number of successful logins.",
		},
		[]string{"app_id"},
	)

	authLoginFailureTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_login_failure_total",
			Help: "Total number of failed login attempts.",
		},
		[]string{"app_id", "reason"},
	)

	authRegisterTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_register_total",
			Help: "Total number of user registrations.",
		},
		[]string{"app_id"},
	)

	authLogoutTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "auth_logout_total",
			Help: "Total number of user logouts.",
		},
		[]string{"app_id"},
	)
)

func init() {
	registry = prometheus.NewRegistry()

	// Standard Go runtime metrics (goroutines, GC, memory, etc.)
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	// Application metrics
	registry.MustRegister(
		httpRequestsTotal,
		httpRequestDuration,
		authLoginSuccessTotal,
		authLoginFailureTotal,
		authRegisterTotal,
		authLogoutTotal,
	)
}

// ----------------------------------------------------------------------------
// Counter helpers — called from domain handlers
// ----------------------------------------------------------------------------

// IncLoginSuccess increments the login success counter for the given app.
func IncLoginSuccess(appID string) {
	authLoginSuccessTotal.WithLabelValues(appID).Inc()
}

// IncLoginFailure increments the login failure counter for the given app and reason.
// Common reasons: "invalid_credentials", "account_locked", "captcha_required", "ip_blocked".
func IncLoginFailure(appID, reason string) {
	authLoginFailureTotal.WithLabelValues(appID, reason).Inc()
}

// IncRegister increments the registration counter for the given app.
func IncRegister(appID string) {
	authRegisterTotal.WithLabelValues(appID).Inc()
}

// IncLogout increments the logout counter for the given app.
func IncLogout(appID string) {
	authLogoutTotal.WithLabelValues(appID).Inc()
}

// ----------------------------------------------------------------------------
// PrometheusMiddleware — records HTTP request counts and latency
// ----------------------------------------------------------------------------

// PrometheusMiddleware is a Gin middleware that instruments every HTTP request.
// It records http_requests_total and http_request_duration_seconds for all routes.
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		path := c.FullPath()
		if path == "" {
			// FullPath is empty for 404s — use the raw path truncated to avoid cardinality explosion
			path = "unmatched"
		}

		duration := time.Since(start).Seconds()
		statusCode := strconv.Itoa(c.Writer.Status())

		httpRequestsTotal.WithLabelValues(c.Request.Method, path, statusCode).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(duration)
	}
}

// ----------------------------------------------------------------------------
// Handler
// ----------------------------------------------------------------------------

// Handler holds the dependencies needed for health and metrics endpoints.
type Handler struct {
	db       *gorm.DB
	rdb      *goredis.Client
	smtpAddr string // optional: "host:port" — empty means unconfigured
}

// NewHandler constructs a Handler.
// smtpAddr may be empty when no SMTP server is configured; the health check will
// report SMTP as "unconfigured" instead of "down".
func NewHandler(db *gorm.DB, rdb *goredis.Client, smtpAddr string) *Handler {
	return &Handler{db: db, rdb: rdb, smtpAddr: smtpAddr}
}

// ----------------------------------------------------------------------------
// GET /health
// ----------------------------------------------------------------------------

// Health godoc
// @Summary      System health check
// @Description  Checks the connectivity and latency of PostgreSQL, Redis, and SMTP. Returns 200 when all configured components are up, 503 when any are down.
// @Tags         Health
// @Produce      json
// @Success      200  {object}  dto.HealthResponse
// @Failure      503  {object}  dto.HealthResponse
// @Router       /health [get]
func (h *Handler) Health(c *gin.Context) {
	checks := make(map[string]dto.ComponentStatus)
	allHealthy := true

	// --- Database check ---
	checks["database"] = checkDatabase(h.db)
	if checks["database"].Status == "down" {
		allHealthy = false
	}

	// --- Redis check ---
	checks["redis"] = checkRedis(h.rdb)
	if checks["redis"].Status == "down" {
		allHealthy = false
	}

	// --- SMTP check ---
	checks["smtp"] = checkSMTP(h.smtpAddr)
	if checks["smtp"].Status == "down" {
		allHealthy = false
	}

	// Derive overall status
	overallStatus := "healthy"
	if !allHealthy {
		overallStatus = "unhealthy"
	}

	httpStatus := http.StatusOK
	if !allHealthy {
		httpStatus = http.StatusServiceUnavailable
	}

	c.JSON(httpStatus, dto.HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	})
}

func checkDatabase(db *gorm.DB) dto.ComponentStatus {
	start := time.Now()
	sqlDB, err := db.DB()
	if err != nil {
		return dto.ComponentStatus{Status: "down", Error: err.Error()}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return dto.ComponentStatus{Status: "down", Error: err.Error()}
	}
	return dto.ComponentStatus{
		Status:    "up",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

func checkRedis(rdb *goredis.Client) dto.ComponentStatus {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return dto.ComponentStatus{Status: "down", Error: err.Error()}
	}
	return dto.ComponentStatus{
		Status:    "up",
		LatencyMs: time.Since(start).Milliseconds(),
	}
}

func checkSMTP(addr string) dto.ComponentStatus {
	if addr == "" {
		return dto.ComponentStatus{Status: "unconfigured"}
	}
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		return dto.ComponentStatus{Status: "down", Host: addr, Error: err.Error()}
	}
	conn.Close()
	return dto.ComponentStatus{
		Status:    "up",
		LatencyMs: time.Since(start).Milliseconds(),
		Host:      addr,
	}
}

// ----------------------------------------------------------------------------
// GET /metrics
// ----------------------------------------------------------------------------

// Metrics godoc
// @Summary      Prometheus metrics
// @Description  Exposes application and runtime metrics in Prometheus exposition format. Requires Admin API Key authentication.
// @Tags         Health
// @Produce      text/plain
// @Security     AdminApiKey
// @Success      200  {string}  string  "Prometheus text format metrics"
// @Failure      401  {object}  dto.ErrorResponse
// @Router       /metrics [get]
func (h *Handler) Metrics(c *gin.Context) {
	// Collect DB connection pool stats at scrape time so they reflect current state
	h.collectDBPoolMetrics()

	// Collect active session count from Redis
	h.collectActiveSessionsMetric()

	// Serve Prometheus text format
	promhttp.HandlerFor(registry, promhttp.HandlerOpts{
		EnableOpenMetrics: false,
	}).ServeHTTP(c.Writer, c.Request)
}

// ----------------------------------------------------------------------------
// Dynamic gauges — registered lazily to avoid duplicate registration panics
// ----------------------------------------------------------------------------

var (
	dbOpenConnections   *prometheus.GaugeVec
	dbInUseConns        *prometheus.GaugeVec
	dbIdleConns         *prometheus.GaugeVec
	activeSessionsGauge prometheus.Gauge
	poolMetricsOnce     = &onceFlag{}
	sessionMetricOnce   = &onceFlag{}
)

type onceFlag struct {
	done bool
}

func (h *Handler) collectDBPoolMetrics() {
	if !poolMetricsOnce.done {
		dbOpenConnections = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "db_connections_open",
			Help: "Number of open database connections.",
		}, []string{})
		dbInUseConns = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "db_connections_in_use",
			Help: "Number of database connections currently in use.",
		}, []string{})
		dbIdleConns = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle database connections.",
		}, []string{})
		// Ignore registration errors (e.g., already registered from a previous call)
		_ = registry.Register(dbOpenConnections)
		_ = registry.Register(dbInUseConns)
		_ = registry.Register(dbIdleConns)
		poolMetricsOnce.done = true
	}

	sqlDB, err := h.db.DB()
	if err != nil {
		return
	}
	stats := sqlDB.Stats()
	dbOpenConnections.WithLabelValues().Set(float64(stats.OpenConnections))
	dbInUseConns.WithLabelValues().Set(float64(stats.InUse))
	dbIdleConns.WithLabelValues().Set(float64(stats.Idle))
}

func (h *Handler) collectActiveSessionsMetric() {
	if !sessionMetricOnce.done {
		activeSessionsGauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "active_sessions_total",
			Help: "Number of active user sessions tracked in Redis.",
		})
		_ = registry.Register(activeSessionsGauge)
		sessionMetricOnce.done = true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Count active session hashes: "app:*:session:*" — one key per live session
	var cursor uint64
	var count int64
	for {
		var keys []string
		var err error
		keys, cursor, err = h.rdb.Scan(ctx, cursor, "app:*:session:*", 100).Result()
		if err != nil {
			break
		}
		count += int64(len(keys))
		if cursor == 0 {
			break
		}
	}
	activeSessionsGauge.Set(float64(count))
}

// ----------------------------------------------------------------------------
// Exported data methods — used by the admin GUI monitoring page
// ----------------------------------------------------------------------------

// GetHealthData runs all health checks and returns the same structured data
// that the /health HTTP endpoint returns. Safe to call concurrently.
func (h *Handler) GetHealthData() dto.HealthResponse {
	checks := make(map[string]dto.ComponentStatus)
	allHealthy := true

	checks["database"] = checkDatabase(h.db)
	if checks["database"].Status == "down" {
		allHealthy = false
	}

	checks["redis"] = checkRedis(h.rdb)
	if checks["redis"].Status == "down" {
		allHealthy = false
	}

	checks["smtp"] = checkSMTP(h.smtpAddr)
	if checks["smtp"].Status == "down" {
		allHealthy = false
	}

	overallStatus := "healthy"
	if !allHealthy {
		overallStatus = "unhealthy"
	}

	return dto.HealthResponse{
		Status:    overallStatus,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Checks:    checks,
	}
}

// MetricsSummary holds key application and runtime metrics in a typed,
// template-friendly structure. Counters are totals since process start.
type MetricsSummary struct {
	// Auth counters — summed across all app_id label values
	LoginSuccessTotal    float64
	LoginFailureTotal    float64
	LoginFailureByReason map[string]float64 // reason -> count
	RegistrationTotal    float64
	LogoutTotal          float64

	// Infrastructure gauges (current values)
	ActiveSessions float64
	DBOpenConns    float64
	DBInUseConns   float64
	DBIdleConns    float64

	// Go runtime
	Goroutines float64
}

// GetMetricsSummary refreshes the dynamic gauges (DB pool, active sessions) and
// then reads all relevant metrics from the Prometheus registry into a
// MetricsSummary that the GUI template can range over directly.
func (h *Handler) GetMetricsSummary() MetricsSummary {
	// Refresh dynamic gauges so the values are current
	h.collectDBPoolMetrics()
	h.collectActiveSessionsMetric()

	var summary MetricsSummary
	summary.LoginFailureByReason = make(map[string]float64)

	mfs, err := registry.Gather()
	if err != nil {
		return summary
	}

	for _, mf := range mfs {
		name := mf.GetName()
		switch name {
		case "auth_login_success_total":
			for _, m := range mf.GetMetric() {
				summary.LoginSuccessTotal += m.GetCounter().GetValue()
			}
		case "auth_login_failure_total":
			for _, m := range mf.GetMetric() {
				val := m.GetCounter().GetValue()
				summary.LoginFailureTotal += val
				// Extract the "reason" label value
				for _, lp := range m.GetLabel() {
					if lp.GetName() == "reason" {
						summary.LoginFailureByReason[lp.GetValue()] += val
					}
				}
			}
		case "auth_register_total":
			for _, m := range mf.GetMetric() {
				summary.RegistrationTotal += m.GetCounter().GetValue()
			}
		case "auth_logout_total":
			for _, m := range mf.GetMetric() {
				summary.LogoutTotal += m.GetCounter().GetValue()
			}
		case "active_sessions_total":
			for _, m := range mf.GetMetric() {
				summary.ActiveSessions = m.GetGauge().GetValue()
			}
		case "db_connections_open":
			for _, m := range mf.GetMetric() {
				summary.DBOpenConns = m.GetGauge().GetValue()
			}
		case "db_connections_in_use":
			for _, m := range mf.GetMetric() {
				summary.DBInUseConns = m.GetGauge().GetValue()
			}
		case "db_connections_idle":
			for _, m := range mf.GetMetric() {
				summary.DBIdleConns = m.GetGauge().GetValue()
			}
		case "go_goroutines":
			for _, m := range mf.GetMetric() {
				summary.Goroutines = m.GetGauge().GetValue()
			}
		}
	}

	return summary
}

// ----------------------------------------------------------------------------
// SMTP address resolver — called from main.go during startup
// ----------------------------------------------------------------------------

// ResolveSMTPAddr queries the database for the global default SMTP configuration
// and returns "host:port". Returns an empty string when no config is found.
func ResolveSMTPAddr(db *gorm.DB) string {
	type smtpRow struct {
		SMTPHost string
		SMTPPort int
	}
	var row smtpRow
	err := db.Raw(
		`SELECT smtp_host, smtp_port FROM email_server_configs
		 WHERE app_id IS NULL AND is_active = true AND is_default = true
		 LIMIT 1`,
	).Scan(&row).Error
	if err != nil || row.SMTPHost == "" {
		// Try any active global config as fallback
		err = db.Raw(
			`SELECT smtp_host, smtp_port FROM email_server_configs
			 WHERE app_id IS NULL AND is_active = true
			 LIMIT 1`,
		).Scan(&row).Error
		if err != nil || row.SMTPHost == "" {
			return ""
		}
	}
	return fmt.Sprintf("%s:%d", row.SMTPHost, row.SMTPPort)
}
