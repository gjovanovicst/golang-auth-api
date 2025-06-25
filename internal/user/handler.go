package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/internal/log"
	"github.com/gjovanovicst/auth_api/internal/util"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type Handler struct {
	Service *Service
}

func NewHandler(s *Service) *Handler {
	return &Handler{Service: s}
}

// @Summary Register a new user
// @Description Register a new user with email and password
// @Tags Auth
// @Accept json
// @Produce json
// @Param   registration  body      dto.RegisterRequest  true  "User Registration Data"
// @Success 201 {object}  dto.UserResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 409 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /register [post]
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	userID, err := h.Service.RegisterUser(req.Email, req.Password)
	if err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log registration activity
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogRegister(userID, ipAddress, userAgent, req.Email)

	c.JSON(http.StatusCreated, dto.MessageResponse{Message: "User registered successfully. Please check your email for verification."})
}

// @Summary User login
// @Description Authenticate user and issue JWTs
// @Tags Auth
// @Accept json
// @Produce json
// @Param   login  body      dto.LoginRequest  true  "User Login Data"
// @Success 200 {object}  dto.LoginResponse
// @Success 202 {object}  dto.TwoFARequiredResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /login [post]
func (h *Handler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	loginResult, err := h.Service.LoginUser(req.Email, req.Password)
	if err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	// Get client info for logging
	ipAddress, userAgent := util.GetClientInfo(c)

	// Check if 2FA is required
	if loginResult.RequiresTwoFA {
		// Log partial login (2FA required)
		details := map[string]interface{}{
			"requires_2fa": true,
		}
		log.LogLogin(loginResult.UserID, ipAddress, userAgent, details)
		c.JSON(http.StatusAccepted, loginResult.TwoFAResponse)
		return
	}

	// Log successful login
	details := map[string]interface{}{
		"requires_2fa": false,
	}
	log.LogLogin(loginResult.UserID, ipAddress, userAgent, details)

	// Standard login response
	c.JSON(http.StatusOK, dto.LoginResponse{
		AccessToken:  loginResult.AccessToken,
		RefreshToken: loginResult.RefreshToken,
	})
}

// @Summary Refresh access token
// @Description Get new access token using refresh token
// @Tags Auth
// @Accept json
// @Produce json
// @Param   refresh  body      dto.RefreshTokenRequest  true  "Refresh Token"
// @Success 200 {object}  dto.LoginResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /refresh-token [post]
func (h *Handler) RefreshToken(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newAccessToken, newRefreshToken, userID, err := h.Service.RefreshUserToken(req.RefreshToken)
	if err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	// Log token refresh activity
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID)
	if parseErr == nil {
		log.LogTokenRefresh(userUUID, ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, gin.H{
		"access_token":  newAccessToken,
		"refresh_token": newRefreshToken,
	})
}

// @Summary Request password reset
// @Description Initiate password reset process
// @Tags Auth
// @Accept json
// @Produce json
// @Param   email  body      dto.ForgotPasswordRequest  true  "User Email"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /forgot-password [post]
func (h *Handler) ForgotPassword(c *gin.Context) {
	var req dto.ForgotPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Note: We don't log password reset requests for security reasons
	// as it could be used to enumerate valid email addresses
	if err := h.Service.RequestPasswordReset(req.Email); err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If an account with that email exists, a password reset link has been sent."})
}

// @Summary Reset password
// @Description Complete password reset process
// @Tags Auth
// @Accept json
// @Produce json
// @Param   reset  body      dto.ResetPasswordRequest  true  "Reset Token and New Password"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /reset-password [post]
func (h *Handler) ResetPassword(c *gin.Context) {
	var req dto.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID, err := h.Service.ConfirmPasswordReset(req.Token, req.NewPassword)
	if err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	// Log password reset completion
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogPasswordReset(userID, ipAddress, userAgent)

	c.JSON(http.StatusOK, gin.H{"message": "Password has been reset successfully."})
}

// @Summary Verify email
// @Description Verify user's email address
// @Tags Auth
// @Produce json
// @Param   token  query     string  true  "Verification Token"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /verify-email [get]
func (h *Handler) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Verification token is missing"})
		return
	}

	userID, err := h.Service.VerifyEmail(token)
	if err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

	// Log email verification
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogEmailVerify(userID, ipAddress, userAgent)

	c.JSON(http.StatusOK, gin.H{"message": "Email verified successfully!"})
}

// @Summary Get user profile
// @Description Retrieve authenticated user's profile information
// @Tags User
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object}  dto.UserResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /profile [get]
func (h *Handler) GetProfile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	user, err := h.Service.Repo.GetUserByID(userID.(string))
	if err != nil {
		c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "User not found"})
		return
	}

	// Log profile access (optional, can be disabled for performance)
	ipAddress, userAgent := util.GetClientInfo(c)
	log.LogProfileAccess(user.ID, ipAddress, userAgent)

	// Return user profile without sensitive information
	c.JSON(http.StatusOK, dto.UserResponse{
		ID:            user.ID.String(),
		Email:         user.Email,
		EmailVerified: user.EmailVerified,
		TwoFAEnabled:  user.TwoFAEnabled,
		CreatedAt:     user.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     user.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// @Summary User logout
// @Description Logout user and revoke refresh token
// @Tags Auth
// @Security ApiKeyAuth
// @Accept json
// @Produce json
// @Param   logout  body      dto.LogoutRequest  true  "Logout Data"
// @Success 200 {object}  dto.MessageResponse
// @Failure 400 {object}  dto.ErrorResponse
// @Failure 401 {object}  dto.ErrorResponse
// @Failure 500 {object}  dto.ErrorResponse
// @Router /logout [post]
func (h *Handler) Logout(c *gin.Context) {
	// Get userID from context (set by AuthMiddleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "User ID not found in context"})
		return
	}

	var req dto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	validate := validator.New()
	if err := validate.Struct(req); err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.Service.LogoutUser(userID.(string), req.RefreshToken); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

	// Log logout activity
	ipAddress, userAgent := util.GetClientInfo(c)
	userUUID, parseErr := uuid.Parse(userID.(string))
	if parseErr == nil {
		log.LogLogout(userUUID, ipAddress, userAgent)
	}

	c.JSON(http.StatusOK, dto.MessageResponse{Message: "Successfully logged out"})
}
