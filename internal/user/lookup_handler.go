package user

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"gorm.io/gorm"
)

// LookupByEmail godoc
// @Summary      Look up a user by email address (internal)
// @Description  Resolves a user by email. Used by Centrora during the invite flow to check
//
//	whether the invited person already has an account. Protected by X-App-API-Key.
//	This endpoint must NEVER be exposed through a public gateway.
//
// @Tags         internal
// @Produce      json
// @Param        id     path      string  true  "Application UUID"
// @Param        email  query     string  true  "Email address to look up"
// @Success      200    {object}  dto.UserLookupResponse
// @Failure      400    {object}  dto.ErrorResponse
// @Failure      404    {object}  map[string]interface{}
// @Failure      500    {object}  dto.ErrorResponse
// @Security     AppApiKey
// @Router       /app/{id}/users/by-email [get]
func (h *Handler) LookupByEmail(c *gin.Context) {
	email := c.Query("email")
	if email == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"code":    "MISSING_PARAMETER",
				"message": "email query parameter is required",
			},
		})
		return
	}

	user, err := h.Service.Repo.FindByEmail(email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{
				"error": gin.H{
					"code":    "USER_NOT_FOUND",
					"message": "No user found with that email address",
				},
			})
			return
		}
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "Failed to look up user"})
		return
	}

	c.JSON(http.StatusOK, dto.UserLookupResponse{
		ID:         user.ID.String(),
		Email:      user.Email,
		Name:       user.Name,
		GivenName:  user.FirstName,
		FamilyName: user.LastName,
		Picture:    user.ProfilePicture,
	})
}
