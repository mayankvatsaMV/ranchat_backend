package main

import (
	"context"
	"log"

	"ranchat/config"
	"ranchat/events"

	// Authentication
	"ranchat/internals/authentication/controller"
	"ranchat/internals/authentication/repository"
	"ranchat/internals/authentication/routes"
	services "ranchat/internals/authentication/service"
	"ranchat/internals/authentication/utils"
	"ranchat/internals/middleware"
	ws "ranchat/internals/websocket"

	// Friendship
	friendshipController "ranchat/internals/friendship/controllers"
	friendshipRepository "ranchat/internals/friendship/repository"
	friendshipRoutes "ranchat/internals/friendship/routes"
	friendshipServices "ranchat/internals/friendship/services"

	// Matchmaking
	matchmakingController "ranchat/internals/match_making/controllers"
	matchmakingDatabase "ranchat/internals/match_making/database"
	matchmakingRepository "ranchat/internals/match_making/repository"
	matchmakingRoutes "ranchat/internals/match_making/routes"
	matchmakingService "ranchat/internals/match_making/services"

	// Presence
	presenceController "ranchat/internals/presence/controller"
	"ranchat/internals/presence/handler"
	presenceRepository "ranchat/internals/presence/repository"
	presenceRoutes "ranchat/internals/presence/routes"
	presenceServices "ranchat/internals/presence/services"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func main() {

	// --------------------------------------------------
	// Configuration
	// --------------------------------------------------

	// Initialize global WS Hub
	wsHub := ws.NewWebSocketHub()

	conf := config.Config{}

	if err := conf.LoadEnvs(); err != nil {
		log.Fatal("Error loading envs:", err)
	}

	utils.InitJWTSecret(conf.JWT_SECRET)

	// --------------------------------------------------
	// Database
	// --------------------------------------------------

	client, db := config.ConnectMongo(conf.MONGO_URI)
	defer config.DisconnectMongo(client)

	// --------------------------------------------------
	// Event Dispatcher
	// --------------------------------------------------

	dispatcher := events.NewEventDispatcher()

	// --------------------------------------------------
	// User / Authentication
	// --------------------------------------------------

	userRepo := repository.NewMongoUserRepo(db)

	userService := services.NewUserService(
		userRepo,
		dispatcher,
	)

	// --------------------------------------------------
	// Redis
	// --------------------------------------------------

	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	if err := redisClient.Ping(context.Background()).Err(); err != nil {
		log.Fatal("Redis connection failed:", err)
	}

	log.Println("Redis connected")

	// --------------------------------------------------
	// Presence
	// --------------------------------------------------

	presenceRepo := presenceRepository.NewRedisPresenceRepository(
		redisClient,
	)

	presenceService := presenceServices.NewPresenceService(
		presenceRepo,
	)

	presenceController := presenceController.NewPresenceController(
		presenceService,
	)

	// --------------------------------------------------
	// User Signup Event Handler
	// --------------------------------------------------

	signupHandler := handler.UserSignupHandler{
		PresenceService: presenceService,
	}

	dispatcher.Subscribe(
		events.UserSignedUp,
		signupHandler.Handle,
	)

	// --------------------------------------------------
	// User Controller
	// --------------------------------------------------

	userController := &controller.UserController{
		Service:         userService,
		PresenceService: presenceService,
	}

	// --------------------------------------------------
	// Matchmaking
	// --------------------------------------------------

	matchmakingDB := matchmakingDatabase.NewMatchmakingMemoryDB()
	matchDB := matchmakingDatabase.NewMatchMemoryDB()

	enqueueChan := matchmakingDatabase.NewEnqueueChan()

	matchmakingRepo := matchmakingRepository.NewMatchMakingRepository(
		matchmakingDB,
		matchDB,
	)

	matchService := &matchmakingService.MatchMakingService{
		Repo:        matchmakingRepo,
		EnqueueChan: enqueueChan,
	}

	matchController := matchmakingController.NewMatchMakingController(
		matchService,
	)

	// --------------------------------------------------
	// Friendship
	// --------------------------------------------------

	friendshipRepo := friendshipRepository.NewMongoFriendshipRepository(
		db,
	)

	friendshipService := friendshipServices.FriendshipService{
		Repo: friendshipRepo,
	}

	friendshipController := friendshipController.NewFriendshipController(
		friendshipService,
		wsHub,
	)

	// --------------------------------------------------
	// Gin Server
	// --------------------------------------------------

	gin.SetMode(gin.ReleaseMode)

	server := gin.New()
	server.Use(gin.Recovery())
	server.Use(gin.Logger())

	// --------------------------------------------------
	// Routes
	// --------------------------------------------------

	routes.RegisterAuthRoutes(
		server,
		userController,
	)

	presenceRoutes.RegisterPresenceRoutes(
		server,
		presenceController,
	)

	matchmakingRoutes.RegisterMatchMakingRoutes(
		server,
		matchController,
	)

	friendshipRoutes.FriendshipRoutes(
		server,
		*friendshipController,
	)
	wsHandler := &ws.WSHandler{Hub: wsHub}
	server.GET("/v1/ws", middleware.AuthMiddleware(), wsHandler.ConnectWS)

	// --------------------------------------------------
	// Start Server
	// --------------------------------------------------

	log.Printf("Server listening on port %s", conf.PORT)

	if err := server.Run(":" + conf.PORT); err != nil {
		log.Fatal("Error starting server:", err)
	}
}
