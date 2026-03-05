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

// ExportMaxRows is the hard cap applied to every export request.
const ExportMaxRows = 10_000

type QueryService struct {
	Repo *Repository
}

func NewQueryService(repo *Repository) *QueryService {
	return &QueryService{Repo: repo}
}

// parseDateFilters parses optional start/end date strings (YYYY-MM-DD) and validates the range.
func parseDateFilters(startDateStr, endDateStr string) (*time.Time, *time.Time, *errors.AppError) {
	var startDate, endDate *time.Time

	if startDateStr != "" {
		parsed, err := time.Parse("2006-01-02", startDateStr)
		if err != nil {
			return nil, nil, errors.NewAppError(errors.ErrBadRequest, "Invalid start_date format. Use YYYY-MM-DD")
		}
		startDate = &parsed
	}

	if endDateStr != "" {
		parsed, err := time.Parse("2006-01-02", endDateStr)
		if err != nil {
			return nil, nil, errors.NewAppError(errors.ErrBadRequest, "Invalid end_date format. Use YYYY-MM-DD")
		}
		endDate = &parsed
	}

	if startDate != nil && endDate != nil && startDate.After(*endDate) {
		return nil, nil, errors.NewAppError(errors.ErrBadRequest, "start_date cannot be after end_date")
	}

	return startDate, endDate, nil
}

// ListUserActivityLogs retrieves activity logs for a specific user with pagination and filtering.
func (s *QueryService) ListUserActivityLogs(userID uuid.UUID, req dto.ActivityLogListRequest) (*dto.ActivityLogListResponse, *errors.AppError) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	startDate, endDate, appErr := parseDateFilters(req.StartDate, req.EndDate)
	if appErr != nil {
		return nil, appErr
	}

	logs, totalCount, err := s.Repo.ListUserActivityLogs(userID, req.Page, req.Limit, req.EventType, startDate, endDate)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to retrieve activity logs")
	}

	responseData := make([]dto.ActivityLogResponse, len(logs))
	for i, log := range logs {
		responseData[i] = s.convertToResponse(log)
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(req.Limit)))
	pagination := dto.PaginationResponse{
		Page:         req.Page,
		Limit:        req.Limit,
		TotalRecords: totalCount,
		TotalPages:   totalPages,
		HasNext:      req.Page < totalPages,
		HasPrevious:  req.Page > 1,
	}

	return &dto.ActivityLogListResponse{
		Data:       responseData,
		Pagination: pagination,
	}, nil
}

// ListAllActivityLogs retrieves activity logs for all users (admin) with pagination and filtering.
func (s *QueryService) ListAllActivityLogs(req dto.ActivityLogListRequest) (*dto.ActivityLogListResponse, *errors.AppError) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	startDate, endDate, appErr := parseDateFilters(req.StartDate, req.EndDate)
	if appErr != nil {
		return nil, appErr
	}

	logs, totalCount, err := s.Repo.ListAllActivityLogs(req.Page, req.Limit, req.EventType, startDate, endDate)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrInternal, "Failed to retrieve activity logs")
	}

	responseData := make([]dto.ActivityLogResponse, len(logs))
	for i, log := range logs {
		responseData[i] = s.convertToResponse(log)
	}

	totalPages := int(math.Ceil(float64(totalCount) / float64(req.Limit)))
	pagination := dto.PaginationResponse{
		Page:         req.Page,
		Limit:        req.Limit,
		TotalRecords: totalCount,
		TotalPages:   totalPages,
		HasNext:      req.Page < totalPages,
		HasPrevious:  req.Page > 1,
	}

	return &dto.ActivityLogListResponse{
		Data:       responseData,
		Pagination: pagination,
	}, nil
}

// GetActivityLogByID retrieves a specific activity log by ID.
func (s *QueryService) GetActivityLogByID(id uuid.UUID, requestingUserID uuid.UUID) (*dto.ActivityLogResponse, *errors.AppError) {
	log, err := s.Repo.GetActivityLogByID(id)
	if err != nil {
		return nil, errors.NewAppError(errors.ErrNotFound, "Activity log not found")
	}

	if log.UserID != requestingUserID {
		return nil, errors.NewAppError(errors.ErrForbidden, "Access denied to this activity log")
	}

	response := s.convertToResponse(*log)
	return &response, nil
}

// ExportUserActivityLogs returns up to ExportMaxRows logs for a specific user.
// truncated is true when the result set was capped.
func (s *QueryService) ExportUserActivityLogs(userID uuid.UUID, req dto.ActivityLogExportRequest) ([]dto.ActivityLogResponse, bool, *errors.AppError) {
	startDate, endDate, appErr := parseDateFilters(req.StartDate, req.EndDate)
	if appErr != nil {
		return nil, false, appErr
	}

	// Fetch one extra row so we can detect truncation without a separate COUNT query.
	logs, err := s.Repo.ExportUserActivityLogs(userID, ExportMaxRows+1, req.EventType, startDate, endDate)
	if err != nil {
		return nil, false, errors.NewAppError(errors.ErrInternal, "Failed to export activity logs")
	}

	truncated := len(logs) > ExportMaxRows
	if truncated {
		logs = logs[:ExportMaxRows]
	}

	responseData := make([]dto.ActivityLogResponse, len(logs))
	for i, log := range logs {
		responseData[i] = s.convertToResponse(log)
	}

	return responseData, truncated, nil
}

// ExportAllActivityLogs returns up to ExportMaxRows logs across all users (admin).
// truncated is true when the result set was capped.
func (s *QueryService) ExportAllActivityLogs(req dto.ActivityLogExportRequest) ([]dto.ActivityLogResponse, bool, *errors.AppError) {
	startDate, endDate, appErr := parseDateFilters(req.StartDate, req.EndDate)
	if appErr != nil {
		return nil, false, appErr
	}

	logs, err := s.Repo.ExportAllActivityLogs(ExportMaxRows+1, req.EventType, startDate, endDate)
	if err != nil {
		return nil, false, errors.NewAppError(errors.ErrInternal, "Failed to export activity logs")
	}

	truncated := len(logs) > ExportMaxRows
	if truncated {
		logs = logs[:ExportMaxRows]
	}

	responseData := make([]dto.ActivityLogResponse, len(logs))
	for i, log := range logs {
		responseData[i] = s.convertToResponse(log)
	}

	return responseData, truncated, nil
}

// convertToResponse converts a models.ActivityLog to dto.ActivityLogResponse.
func (s *QueryService) convertToResponse(log models.ActivityLog) dto.ActivityLogResponse {
	var details interface{}
	if len(log.Details) > 0 {
		if err := json.Unmarshal(log.Details, &details); err != nil {
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
		IsAnomaly: log.IsAnomaly,
		Severity:  log.Severity,
	}
}
