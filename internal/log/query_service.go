package log

import (
	"encoding/json"
	"math"
	"time"

	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/gjovanovicst/auth_api/pkg/errors"
	"github.com/gjovanovicst/auth_api/pkg/models"
	"github.com/google/uuid"
)

type QueryService struct {
	Repo *Repository
}

func NewQueryService(repo *Repository) *QueryService {
	return &QueryService{Repo: repo}
}

// ListUserActivityLogs retrieves activity logs for a specific user with pagination and filtering
func (s *QueryService) ListUserActivityLogs(userID uuid.UUID, req dto.ActivityLogListRequest) (*dto.ActivityLogListResponse, *errors.AppError) {
	// Set default values
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 20 // Default page size
	}
	if req.Limit > 100 {
		req.Limit = 100 // Maximum page size
	}

	// Parse date filters
	var startDate, endDate *time.Time
	var err error

	if req.StartDate != "" {
		if parsed, parseErr := time.Parse("2006-01-02", req.StartDate); parseErr != nil {
			return nil, errors.NewAppError(errors.ErrBadRequest, "Invalid start_date format. Use YYYY-MM-DD")
		} else {
			startDate = &parsed
		}
	}

	if req.EndDate != "" {
		if parsed, parseErr := time.Parse("2006-01-02", req.EndDate); parseErr != nil {
			return nil, errors.NewAppError(errors.ErrBadRequest, "Invalid end_date format. Use YYYY-MM-DD")
		} else {
			endDate = &parsed
		}
	}

	// Validate date range
	if startDate != nil && endDate != nil && startDate.After(*endDate) {
		return nil, errors.NewAppError(errors.ErrBadRequest, "start_date cannot be after end_date")
	}

	// Get logs from repository
	logs, totalCount, err := s.Repo.ListUserActivityLogs(userID, req.Page, req.Limit, req.EventType, startDate, endDate)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to retrieve activity logs")
	}

	// Convert to response format
	responseData := make([]dto.ActivityLogResponse, len(logs))
	for i, log := range logs {
		responseData[i] = s.convertToResponse(log)
	}

	// Calculate pagination metadata
	totalPages := int(math.Ceil(float64(totalCount) / float64(req.Limit)))
	hasNext := req.Page < totalPages
	hasPrevious := req.Page > 1

	pagination := dto.PaginationResponse{
		Page:         req.Page,
		Limit:        req.Limit,
		TotalRecords: totalCount,
		TotalPages:   totalPages,
		HasNext:      hasNext,
		HasPrevious:  hasPrevious,
	}

	return &dto.ActivityLogListResponse{
		Data:       responseData,
		Pagination: pagination,
	}, nil
}

// ListAllActivityLogs retrieves activity logs for all users (admin functionality)
func (s *QueryService) ListAllActivityLogs(req dto.ActivityLogListRequest) (*dto.ActivityLogListResponse, *errors.AppError) {
	// Set default values
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 20 // Default page size
	}
	if req.Limit > 100 {
		req.Limit = 100 // Maximum page size
	}

	// Parse date filters
	var startDate, endDate *time.Time
	var err error

	if req.StartDate != "" {
		if parsed, parseErr := time.Parse("2006-01-02", req.StartDate); parseErr != nil {
			return nil, errors.NewAppError(errors.ErrBadRequest, "Invalid start_date format. Use YYYY-MM-DD")
		} else {
			startDate = &parsed
		}
	}

	if req.EndDate != "" {
		if parsed, parseErr := time.Parse("2006-01-02", req.EndDate); parseErr != nil {
			return nil, errors.NewAppError(errors.ErrBadRequest, "Invalid end_date format. Use YYYY-MM-DD")
		} else {
			endDate = &parsed
		}
	}

	// Validate date range
	if startDate != nil && endDate != nil && startDate.After(*endDate) {
		return nil, errors.NewAppError(errors.ErrBadRequest, "start_date cannot be after end_date")
	}

	// Get logs from repository
	logs, totalCount, err := s.Repo.ListAllActivityLogs(req.Page, req.Limit, req.EventType, startDate, endDate)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to retrieve activity logs")
	}

	// Convert to response format
	responseData := make([]dto.ActivityLogResponse, len(logs))
	for i, log := range logs {
		responseData[i] = s.convertToResponse(log)
	}

	// Calculate pagination metadata
	totalPages := int(math.Ceil(float64(totalCount) / float64(req.Limit)))
	hasNext := req.Page < totalPages
	hasPrevious := req.Page > 1

	pagination := dto.PaginationResponse{
		Page:         req.Page,
		Limit:        req.Limit,
		TotalRecords: totalCount,
		TotalPages:   totalPages,
		HasNext:      hasNext,
		HasPrevious:  hasPrevious,
	}

	return &dto.ActivityLogListResponse{
		Data:       responseData,
		Pagination: pagination,
	}, nil
}

// GetActivityLogByID retrieves a specific activity log by ID
func (s *QueryService) GetActivityLogByID(id uuid.UUID, requestingUserID uuid.UUID) (*dto.ActivityLogResponse, *errors.AppError) {
	log, err := s.Repo.GetActivityLogByID(id)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrNotFound, "Activity log not found")
	}

	// Check if the requesting user has permission to view this log
	// Users can only view their own logs unless they're admin (this would need additional role checking)
	if log.UserID != requestingUserID {
		return nil, errors.NewAppError(errors.ErrForbidden, "Access denied to this activity log")
	}

	response := s.convertToResponse(*log)
	return &response, nil
}

// convertToResponse converts a models.ActivityLog to dto.ActivityLogResponse
func (s *QueryService) convertToResponse(log models.ActivityLog) dto.ActivityLogResponse {
	var details interface{}
	if len(log.Details) > 0 {
		// Try to unmarshal the JSON details into a generic interface
		if err := json.Unmarshal(log.Details, &details); err != nil {
			// If unmarshaling fails, return empty object
			details = map[string]interface{}{}
		}
	} else {
		details = map[string]interface{}{}
	}

	return dto.ActivityLogResponse{
		ID:        log.ID.String(),
		UserID:    log.UserID.String(),
		EventType: log.EventType,
		Timestamp: log.Timestamp.Format(time.RFC3339),
		IPAddress: log.IPAddress,
		UserAgent: log.UserAgent,
		Details:   details,
	}
}
