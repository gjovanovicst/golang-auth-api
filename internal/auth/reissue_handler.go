package auth

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	jwtpkg "github.com/gjovanovicst/auth_api/pkg/jwt"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// ReissueHandler handles the internal token reissue endpoint.
// This endpoint is called exclusively by Centrora after a successful context-switch
// to mint a new access token carrying org context (org_id + org_role) in the JWT claims.
type ReissueHandler struct{}

// NewReissueHandler creates a new ReissueHandler.
func NewReissueHandler() *ReissueHandler {
	return &ReissueHandler{}
}

// ReissueToken godoc
// @Summary      Reissue access token with org context (internal)
// @Description  Mints a new access token embedding org_id and org_role claims.
//
//	Called exclusively by Centrora after a context-switch. Protected by X-App-API-Key.
//	This endpoint must NEVER be exposed through a public gateway.
//
// @Tags         internal
// @Accept       json
// @Produce      json
// @Param        id   path      string                   true  "Application UUID"
// @Param        request body dto.ReissueTokenRequest   true  "Reissue request"
// @Success      200  {object}  dto.ReissueTokenResponse
// @Failure      400  {object}  dto.ErrorResponse
// @Failure      422  {object}  dto.ErrorResponse
// @Failure      500  {object}  dto.ErrorResponse
// @Security     AppApiKey
// @Router       /app/{id}/auth/reissue [post]
func (h *ReissueHandler) ReissueToken(c *gin.Context) {
	var req dto.ReissueTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "INVALID_REQUEST",
				"message": err.Error(),
			},
		})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": gin.H{
				"code":    "VALIDATION_FAILED",
				"message": err.Error(),
			},
		})
		return
	}

	// app_id is already resolved and set in context by AppApiKeyMiddleware + AppRouteGuardMiddleware
	appIDVal, exists := c.Get("app_id")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "App ID missing from context",
			},
		})
		return
	}
	appID, ok := appIDVal.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "INTERNAL_ERROR",
				"message": "App ID has unexpected type in context",
			},
		})
		return
	}

	// Use the original token's app_id when provided so the reissued JWT carries
	// the same app_id that was used to create the Redis session. Without this,
	// validation of the reissued token would look up the session under the
	// calling app's (e.g. Centrora's) app_id and fail with "Session has been revoked".
	jwtAppID := appID.String()
	if req.OriginalAppID != "" {
		jwtAppID = req.OriginalAppID
	}

	log.Printf("[reissue] DEBUG: appID=%s originalAppID=%s jwtAppID=%s sessionID=%s orgID=%s orgRole=%s",
		appID.String(), req.OriginalAppID, jwtAppID, req.SessionID, req.OrgID, req.OrgRole)

	ttl := jwtpkg.DefaultAccessTokenTTL()

	token, err := jwtpkg.GenerateOrgContextToken(
		jwtAppID,
		req.UserID,
		req.SessionID,
		req.OrgID,
		req.OrgRole,
		req.Roles,
		ttl,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    "TOKEN_GENERATION_FAILED",
				"message": "Failed to generate token",
			},
		})
		return
	}

	// Persist org context in the Redis session so subsequent token refreshes
	// re-embed org_id / org_role and the org scope survives access-token rotation.
	persistOrgContextInSession(jwtAppID, req.SessionID, req.OrgID, req.OrgRole)

	c.JSON(http.StatusOK, dto.ReissueTokenResponse{
		AccessToken: token,
		ExpiresIn:   int64(ttl / time.Second),
	})
}

// persistOrgContextInSession stores org_id / org_role in the Redis session hash so
// that subsequent token refreshes can re-embed the same org context and the user
// does not silently lose their active org scope after the access token expires.
// The session key is scoped to the original app_id (not Centrora's) because that
// is where the session was created.
func persistOrgContextInSession(appID, sessionID, orgID, orgRole string) {
	if sessionID == "" {
		log.Printf("[reissue] Warning: sessionID is empty, cannot persist org context")
		return
	}
	if err := redis.SetSessionOrgContext(appID, sessionID, orgID, orgRole); err != nil {
		log.Printf("[reissue] Warning: failed to persist org context in session %s: %v", sessionID, err)
	} else {
		log.Printf("[reissue] Persisted org context in session app=%s session=%s org=%s role=%s", appID, sessionID, orgID, orgRole)
	}
}
