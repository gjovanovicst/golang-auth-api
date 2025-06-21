package main

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"

	"github.com/gjovanovicst/auth_api/internal/database"
	"github.com/gjovanovicst/auth_api/internal/email"
	"github.com/gjovanovicst/auth_api/internal/middleware"
	"github.com/gjovanovicst/auth_api/internal/redis"
	"github.com/gjovanovicst/auth_api/internal/social"
	"github.com/gjovanovicst/auth_api/internal/user"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	// Initialize Viper for configuration management
	viper.AutomaticEnv() // Read environment variables
	viper.SetDefault("PORT", "8080")
	viper.SetDefault("ACCESS_TOKEN_EXPIRATION_MINUTES", 15)
	viper.SetDefault("REFRESH_TOKEN_EXPIRATION_HOURS", 720)

	// Connect to database
	database.ConnectDatabase()

	// Connect to Redis
	redis.ConnectRedis()

	// Run database migrations
	database.MigrateDatabase()

	// Initialize Services and Handlers
	userRepo := user.NewRepository(database.DB)
	socialRepo := social.NewRepository(database.DB)
	emailService := email.NewService()
	userService := user.NewService(userRepo, emailService)
	socialService := social.NewService(userRepo, socialRepo)
	userHandler := user.NewHandler(userService)
	socialHandler := social.NewHandler(socialService)

	// Setup Gin Router
	r := gin.Default()

	// Public routes
	public := r.Group("/")
	{
		public.POST("/register", userHandler.Register)
		public.POST("/login", userHandler.Login)
		public.POST("/refresh-token", userHandler.RefreshToken)
		public.POST("/forgot-password", userHandler.ForgotPassword)
		public.POST("/reset-password", userHandler.ResetPassword)
		public.GET("/verify-email", userHandler.VerifyEmail)
	}

	// Social authentication routes
	auth := r.Group("/auth")
	{
		// Google OAuth2
		auth.GET("/google/login", socialHandler.GoogleLogin)
		auth.GET("/google/callback", socialHandler.GoogleCallback)

		// Facebook OAuth2
		auth.GET("/facebook/login", socialHandler.FacebookLogin)
		auth.GET("/facebook/callback", socialHandler.FacebookCallback)

		// GitHub OAuth2
		auth.GET("/github/login", socialHandler.GithubLogin)
		auth.GET("/github/callback", socialHandler.GithubCallback)
	}

	// Protected routes (require JWT authentication)
	protected := r.Group("/")
	protected.Use(middleware.AuthMiddleware())
	{
		protected.GET("/profile", userHandler.GetProfile) // Example protected route
		// Add other protected routes here
	}

	// Start the server
	port := viper.GetString("PORT")
	log.Printf("Server starting on port %s", port)
	if err := r.Run(fmt.Sprintf(":%s", port)); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}