package admin

import (
	"time"

	"github.com/gjovanovicst/auth_api/pkg/models"
	"gorm.io/gorm"
)

// DashboardStats holds aggregate counts for the admin dashboard.
type DashboardStats struct {
	TotalUsers        int64
	ActiveUsers       int64
	InactiveUsers     int64
	TotalTenants      int64
	TotalApps         int64
	RecentEventsCount int64 // activity logs in last 24 hours
}

// DashboardService provides aggregated data for the admin dashboard.
// It uses *gorm.DB directly because it queries across multiple domains
// (users, tenants, apps, activity logs) and dedicated repo methods
// would add unnecessary indirection for simple COUNT queries.
type DashboardService struct {
	db *gorm.DB
}

// NewDashboardService creates a new DashboardService.
func NewDashboardService(db *gorm.DB) *DashboardService {
	return &DashboardService{db: db}
}

// GetStats returns aggregate counts for the dashboard stat cards.
func (s *DashboardService) GetStats() (*DashboardStats, error) {
	stats := &DashboardStats{}

	// Count total users
	if err := s.db.Model(&models.User{}).Count(&stats.TotalUsers).Error; err != nil {
		return nil, err
	}

	// Count active users
	if err := s.db.Model(&models.User{}).Where("is_active = ?", true).Count(&stats.ActiveUsers).Error; err != nil {
		return nil, err
	}

	// Count inactive users
	stats.InactiveUsers = stats.TotalUsers - stats.ActiveUsers

	// Count total tenants
	if err := s.db.Model(&models.Tenant{}).Count(&stats.TotalTenants).Error; err != nil {
		return nil, err
	}

	// Count total applications
	if err := s.db.Model(&models.Application{}).Count(&stats.TotalApps).Error; err != nil {
		return nil, err
	}

	// Count activity logs in the last 24 hours
	since := time.Now().Add(-24 * time.Hour)
	if err := s.db.Model(&models.ActivityLog{}).
		Where("timestamp >= ?", since).
		Count(&stats.RecentEventsCount).Error; err != nil {
		return nil, err
	}

	return stats, nil
}

// GetRecentActivity returns the most recent activity log entries.
func (s *DashboardService) GetRecentActivity(limit int) ([]models.ActivityLog, error) {
	var logs []models.ActivityLog
	if err := s.db.Order("timestamp desc").Limit(limit).Find(&logs).Error; err != nil {
		return nil, err
	}
	return logs, nil
}
