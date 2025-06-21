## Phase 7: Automatic Swagger Documentation

This phase focuses on integrating automatic Swagger (OpenAPI) documentation into the GoLang RESTful API. Swagger documentation provides an interactive interface for developers to understand, test, and interact with the API endpoints, significantly improving API usability and maintainability. We will leverage tools that generate this documentation directly from the Go source code, ensuring it stays up-to-date with API changes.

### 7.1 Choosing a Swagger Tool

For Go applications, `swag` (Swag: Go to Swagger 2.0) is a popular and widely used tool that automatically generates Swagger 2.0 and OpenAPI 3.0 documentation from Go annotations. It integrates well with Gin and other popular Go web frameworks.

**Tool:** `github.com/swaggo/swag`
**Gin Integration:** `github.com/swaggo/gin-swagger` and `github.com/swaggo/files`

### 7.2 Installation and Setup

1.  **Install `swag` CLI:**
    ```bash
    go install github.com/swaggo/swag/cmd/swag@latest
    ```

2.  **Install Gin-Swagger Dependencies:**
    ```bash
    go get github.com/swaggo/gin-swagger@latest github.com/swaggo/files@latest
    ```

### 7.3 Annotating Go Code

`swag` uses annotations (comments) in your Go code to generate the Swagger specification. These annotations are placed above your main application entry point (`main.go`) and above your handler functions.

**Global Annotations (in `cmd/api/main.go` or a dedicated `docs` package):**

These annotations provide general information about your API, such as title, version, description, and host.

```go
package main

import (
	// ... other imports

	_ "github.com/your_username/your_project/docs" // Import generated docs
	ginSwagger "github.com/swaggo/gin-swagger"
	"github.com/swaggo/files"
)

// @title           Authentication and Authorization API
// @version         1.0
// @description     This is a sample authentication and authorization API built with Go and Gin.
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8080
// @BasePath  /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description Type "Bearer" + your JWT token

func main() {
	// ... existing main function content

	// Add Swagger UI endpoint
	r.GET("/swagger/*any", ginSwagger.WrapHandler(files.Handler))

	// ... rest of your main function
}
```

**Handler Annotations:**

Each handler function needs annotations to describe the API endpoint, parameters, responses, and security requirements.

**Example `internal/user/handler.go` (Register Handler):**

```go
package user

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/your_username/your_project/pkg/dto"
	"github.com/your_username/your_project/pkg/errors"
)

// @Summary Register a new user
// @Description Register a new user with email and password
// @Tags Auth
// @Accept json
// @Produce json
// @Param   registration  body      dto.RegisterRequest  true  "User Registration Data"
// @Success 201 {object}  map[string]string{"message":"User registered successfully. Please verify your email."}
// @Failure 400 {object}  map[string]string{"error":"Bad Request"}
// @Failure 409 {object}  map[string]string{"error":"User already exists"}
// @Failure 500 {object}  map[string]string{"error":"Internal Server Error"}
// @Router /register [post]
func (h *Handler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if appErr := h.Service.RegisterUser(req.Email, req.Password); appErr != nil {
		c.JSON(appErr.Code, gin.H{"error": appErr.Message})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully. Please verify your email."})
}

// @Summary User login
// @Description Authenticate user and issue JWTs
// @Tags Auth
// @Accept json
// @Produce json
// @Param   login  body      dto.LoginRequest  true  "User Login Data"
// @Success 200 {object}  dto.LoginResponse
// @Failure 400 {object}  map[string]string{"error":"Bad Request"}
// @Failure 401 {object}  map[string]string{"error":"Invalid credentials"}
// @Failure 500 {object}  map[string]string{"error":"Internal Server Error"}
// @Router /login [post]
func (h *Handler) Login(c *gin.Context) {
	// ... existing Login handler logic
}

// @Summary Get user profile
// @Description Retrieve authenticated user's profile information
// @Tags User
// @Security ApiKeyAuth
// @Produce json
// @Success 200 {object}  dto.UserProfileResponse
// @Failure 401 {object}  map[string]string{"error":"Unauthorized"}
// @Failure 500 {object}  map[string]string{"error":"Internal Server Error"}
// @Router /profile [get]
func (h *Handler) GetProfile(c *gin.Context) {
	// ... existing GetProfile handler logic
}
```

**DTO Annotations:**

For `swag` to correctly generate models for request and response bodies, you might need to add `json` tags to your DTO structs in `pkg/dto/auth.go`.

```go
package dto

type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type UserProfileResponse struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
}
```

### 7.4 Generating Swagger Documentation

After adding annotations, run the `swag init` command from your project root directory. This command will parse your Go code and generate the `docs` directory containing `docs.go`, `swagger.json`, and `swagger.yaml`.

```bash
swag init
```

**Important:** The `docs` directory and its contents should be committed to your version control system, as `docs.go` is imported by your `main.go`.

### 7.5 Accessing Swagger UI

Once the application is running, you can access the Swagger UI in your browser at `http://localhost:8080/swagger/index.html` (or whatever host and port your API is running on).

### 7.6 Continuous Integration

To ensure documentation is always up-to-date, integrate `swag init` into your CI/CD pipeline. This can be run before building the application, ensuring that the latest API documentation is always part of your deployment.

This phase provides a clear path to automatically generate and serve interactive API documentation, greatly enhancing the developer experience for anyone consuming your API.

