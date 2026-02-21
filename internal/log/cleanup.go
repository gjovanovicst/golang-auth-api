package log

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gjovanovicst/auth_api/internal/config"
	"gorm.io/gorm"
)

// CleanupService handles automatic deletion of expired activity logs
type CleanupService struct {
	db              *gorm.DB
	ctx             context.Context
	cancel          context.CancelFunc
	config          *config.LoggingConfig
	ticker          *time.Ticker
	isRunning       bool
	lastCleanupTime time.Time
	totalCleaned    int64
}

// CleanupStats holds statistics about cleanup operations
type CleanupStats struct {
	LastRun         time.Time
	DeletedCount    int64
	Duration        time.Duration
	TotalCleaned    int64
	NextScheduledAt time.Time
}

var cleanupServiceInstance *CleanupService

// InitializeCleanupService creates and starts the cleanup service
func InitializeCleanupService(db *gorm.DB) *CleanupService {
	if cleanupServiceInstance != nil {
		return cleanupServiceInstance
	}

	cfg := config.GetLoggingConfig()

	if !cfg.CleanupEnabled {
		log.Println("Activity log cleanup service is disabled")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	cleanupServiceInstance = &CleanupService{
		db:              db,
		ctx:             ctx,
		cancel:          cancel,
		config:          cfg,
		ticker:          time.NewTicker(cfg.CleanupInterval),
		isRunning:       false,
		lastCleanupTime: time.Time{},
		totalCleaned:    0,
	}

	// Start the cleanup worker
	go cleanupServiceInstance.worker()

	log.Printf("Activity log cleanup service initialized (interval: %v)", cfg.CleanupInterval)

	return cleanupServiceInstance
}

// GetCleanupService returns the singleton cleanup service instance
func GetCleanupService() *CleanupService {
	return cleanupServiceInstance
}

// worker runs the cleanup process on a schedule
func (cs *CleanupService) worker() {
	log.Println("Activity log cleanup worker started")

	// Run initial cleanup after a short delay
	time.Sleep(30 * time.Second)
	cs.runCleanup()

	for {
		select {
		case <-cs.ctx.Done():
			log.Println("Activity log cleanup service shutting down...")
			cs.ticker.Stop()
			return
		case <-cs.ticker.C:
			cs.runCleanup()
		}
	}
}

// runCleanup performs the actual cleanup operation
func (cs *CleanupService) runCleanup() {
	if cs.isRunning {
		log.Println("Cleanup already in progress, skipping...")
		return
	}

	cs.isRunning = true
	defer func() {
		cs.isRunning = false
	}()

	startTime := time.Now()
	log.Println("Starting activity log cleanup...")

	deletedCount, err := cs.deleteExpiredLogs()
	if err != nil {
		log.Printf("Error during cleanup: %v", err)
		return
	}

	duration := time.Since(startTime)
	cs.lastCleanupTime = startTime
	cs.totalCleaned += deletedCount

	log.Printf("Cleanup completed: deleted %d logs in %v (total: %d)",
		deletedCount, duration, cs.totalCleaned)
}

// deleteExpiredLogs deletes logs that have passed their expiration date
func (cs *CleanupService) deleteExpiredLogs() (int64, error) {
	batchSize := cs.config.CleanupBatchSize
	totalDeleted := int64(0)
	now := time.Now().UTC()

	for {
		// Delete in batches to avoid locking the table for too long
		result := cs.db.
			Table("activity_logs").
			Where("expires_at IS NOT NULL AND expires_at < ?", now).
			Limit(batchSize).
			Delete(nil)

		if result.Error != nil {
			return totalDeleted, fmt.Errorf("failed to delete expired logs: %w", result.Error)
		}

		deletedInBatch := result.RowsAffected
		totalDeleted += deletedInBatch

		// If we deleted fewer than batch size, we're done
		if deletedInBatch < int64(batchSize) {
			break
		}

		// Small delay between batches to avoid overwhelming the database
		time.Sleep(100 * time.Millisecond)
	}

	return totalDeleted, nil
}

// DeleteLogsByUserID deletes all logs for a specific user (for GDPR compliance)
func (cs *CleanupService) DeleteLogsByUserID(userID string) (int64, error) {
	result := cs.db.Table("activity_logs").Where("user_id = ?", userID).Delete(nil)

	if result.Error != nil {
		return 0, fmt.Errorf("failed to delete logs for user %s: %w", userID, result.Error)
	}

	log.Printf("Deleted %d activity logs for user %s", result.RowsAffected, userID)
	return result.RowsAffected, nil
}

// GetStats returns statistics about the cleanup service
func (cs *CleanupService) GetStats() CleanupStats {
	nextScheduled := cs.lastCleanupTime.Add(cs.config.CleanupInterval)
	if cs.lastCleanupTime.IsZero() {
		nextScheduled = time.Now().Add(30 * time.Second)
	}

	return CleanupStats{
		LastRun:         cs.lastCleanupTime,
		TotalCleaned:    cs.totalCleaned,
		NextScheduledAt: nextScheduled,
	}
}

// ForceCleanup triggers an immediate cleanup (for manual operations)
func (cs *CleanupService) ForceCleanup() (*CleanupStats, error) {
	if cs.isRunning {
		return nil, fmt.Errorf("cleanup already in progress")
	}

	cs.isRunning = true
	defer func() {
		cs.isRunning = false
	}()

	startTime := time.Now()
	deletedCount, err := cs.deleteExpiredLogs()
	if err != nil {
		return nil, err
	}

	duration := time.Since(startTime)
	cs.lastCleanupTime = startTime
	cs.totalCleaned += deletedCount

	stats := &CleanupStats{
		LastRun:         startTime,
		DeletedCount:    deletedCount,
		Duration:        duration,
		TotalCleaned:    cs.totalCleaned,
		NextScheduledAt: startTime.Add(cs.config.CleanupInterval),
	}

	return stats, nil
}

// Shutdown gracefully shuts down the cleanup service
func (cs *CleanupService) Shutdown() {
	if cs == nil {
		return
	}

	log.Println("Shutting down cleanup service...")
	if cs.cancel != nil {
		cs.cancel()
	}

	if cs.ticker != nil {
		cs.ticker.Stop()
	}

	// Wait for any running cleanup to finish (with timeout)
	timeout := time.After(30 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for cs.isRunning {
		select {
		case <-timeout:
			log.Println("Cleanup service shutdown timeout reached")
			return
		case <-ticker.C:
			// Continue waiting
		}
	}

	log.Println("Cleanup service shutdown complete")
}

// GetExpiredLogCount returns the count of logs that are ready to be cleaned up
func (cs *CleanupService) GetExpiredLogCount() (int64, error) {
	var count int64
	now := time.Now().UTC()

	err := cs.db.Table("activity_logs").
		Where("expires_at IS NOT NULL AND expires_at < ?", now).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count expired logs: %w", err)
	}

	return count, nil
}

// UpdateRetentionForEventType updates the expiration date for all logs of a specific event type
// This is useful when retention policies change
func (cs *CleanupService) UpdateRetentionForEventType(eventType string, newRetentionDays int) (int64, error) {
	// Calculate new expiration based on the log's timestamp.
	// Note: INTERVAL '1 day' * ? allows proper parameterization (cannot parameterize inside INTERVAL literals).
	result := cs.db.Exec(`
		UPDATE activity_logs 
		SET expires_at = timestamp + INTERVAL '1 day' * ?
		WHERE event_type = ? AND expires_at IS NOT NULL
	`, newRetentionDays, eventType)

	if result.Error != nil {
		return 0, fmt.Errorf("failed to update retention for event type %s: %w", eventType, result.Error)
	}

	log.Printf("Updated retention for %d logs of type %s", result.RowsAffected, eventType)
	return result.RowsAffected, nil
}
