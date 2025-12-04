# Optional: Add Dedicated Token Validation Endpoint

If you want a dedicated endpoint for external services to validate tokens, you could add this to your existing Auth API:

## 1. Add Handler Method

Add this to `internal/user/handler.go`:

```go
// ValidateToken godoc
// @Summary      Validate JWT Token
// @Description  Validates a JWT token and returns basic user info
// @Tags         auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} map[string]string
// @Router       /auth/validate [get]
func (h *Handler) ValidateToken(c *gin.Context) {
	// Get user ID from context (set by AuthMiddleware)
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "User ID not found in context",
		})
		return
	}

	// Get user basic info
	user, err := h.UserRepo.GetUserByID(userID.(string))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"valid":  true,
		"userID": user.ID,
		"email":  user.Email,
		"name":   user.Name,
	})
}
```

## 2. Add Route

Add this to `cmd/api/main.go` in the protected routes section:

```go
// Protected routes (require JWT authentication)
protected := r.Group("/")
protected.Use(middleware.AuthMiddleware())
{
	protected.GET("/profile", userHandler.GetProfile)
	protected.GET("/auth/validate", userHandler.ValidateToken) // Add this line
	protected.POST("/logout", userHandler.Logout)
	// ... other routes
}
```

## 3. Usage from Permisio API

Then your Permisio API would call:

```bash
GET /auth/validate
Authorization: Bearer <token>
```

**Response (Success):**
```json
{
  "valid": true,
  "userID": "uuid-here",
  "email": "user@example.com", 
  "name": "User Name"
}
```

**Response (Invalid):**
```json
{
  "error": "Invalid or expired token"
}
```

This endpoint would be lighter than `/profile` since it only returns essential validation data. 