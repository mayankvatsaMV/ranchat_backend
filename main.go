// main.go — the entry point for the Ranchat backend.
//
// What happens when you run "go run main.go":
//  1. Load environment variables from .env
//  2. Connect to MongoDB (and create indexes)
//  3. Create a Gin HTTP server
//  4. Register all routes
//  5. Start listening for requests
package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"ranchat_backend/config"
	"ranchat_backend/routes"
)

func main() {
	// 1. Load .env file (if present). Environment variables already set in the
	//    system (e.g. on a cloud server) will NOT be overwritten.
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found — using system environment variables")
	}

	// 2. Connect to MongoDB and set up indexes
	config.ConnectMongoDB()

	// 3. Set Gin to release mode in production (less verbose logging)
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	}

	// 4. Create the Gin router
	//    - gin.Recovery() catches any panics and returns a 500 instead of crashing
	//    - gin.Logger() prints each request to stdout
	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	// Health-check endpoint — useful for load balancers / uptime monitors
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "ranchat-backend"})
	})

	// 5. Register all API routes
	routes.RegisterRoutes(r)

	// 6. Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 Ranchat backend running on :%s\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
