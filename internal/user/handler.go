package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gjovanovicst/auth_api/pkg/dto"
	"github.com/go-playground/validator/v10"
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

	if err := h.Service.RegisterUser(req.Email, req.Password); err != nil {
		c.JSON(err.Code, dto.ErrorResponse{Error: err.Message})
		return
	}

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

	// Check if 2FA is required
	if loginResult.RequiresTwoFA {
		c.JSON(http.StatusAccepted, loginResult.TwoFAResponse)
		return
	}

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

	newAccessToken, newRefreshToken, err := h.Service.RefreshUserToken(req.RefreshToken)
	if err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
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

	if err := h.Service.ConfirmPasswordReset(req.Token, req.NewPassword); err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

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

	if err := h.Service.VerifyEmail(token); err != nil {
		c.JSON(err.Code, gin.H{"error": err.Message})
		return
	}

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
