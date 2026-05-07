// Package config handles connecting to MongoDB.
// The global `DB` variable is set once at startup and shared safely across
// all goroutines — the MongoDB driver is designed to be used this way.
package config

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// DB is the global database handle. Use GetCollection() to get a specific collection.
var DB *mongo.Database

// ConnectMongoDB opens the connection to MongoDB and sets up indexes.
// Call this once when the server starts.
func ConnectMongoDB() {
	uri := os.Getenv("MONGO_URI")
	if uri == "" {
		uri = "mongodb://localhost:27017"
	}

	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "ranchat"
	}

	// ── Connection pool tuning for high concurrency ───────────────────────────
	// The pool lets many goroutines share a small set of real TCP connections
	// to MongoDB, which is much cheaper than opening a new connection per request.
	clientOpts := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(100).                 // allow up to 100 simultaneous MongoDB operations
		SetMinPoolSize(5).                   // keep 5 connections warm at all times
		SetMaxConnIdleTime(30 * time.Second) // drop idle connections after 30 s

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		log.Fatalf("MongoDB connect error: %v", err)
	}

	// Ping confirms the server is actually reachable
	if err = client.Ping(ctx, nil); err != nil {
		log.Fatalf("MongoDB ping failed: %v", err)
	}

	DB = client.Database(dbName)
	fmt.Printf("✅ Connected to MongoDB — database: %s\n", dbName)

	EnsureIndexes()
}

// EnsureIndexes creates indexes that make common queries fast.
// It is safe to call on every startup — MongoDB skips indexes that already exist.
func EnsureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	indexes := []mongo.IndexModel{
		{
			// One account per device — enforced at the database level
			Keys:    bson.D{{Key: "deviceId", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_deviceId_unique"),
		},
		{
			// Speeds up matchmaking queries: "find all users where isSearching = true"
			Keys:    bson.D{{Key: "isSearching", Value: 1}},
			Options: options.Index().SetName("idx_isSearching"),
		},
	}

	if _, err := DB.Collection("users").Indexes().CreateMany(ctx, indexes); err != nil {
		log.Fatalf("Failed to create users indexes: %v", err)
	}

	// ── Presence collection indexes ───────────────────────────────────────────
	presenceIndexes := []mongo.IndexModel{
		{
			// One presence document per user — enforced at the database level
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().SetUnique(true).SetName("idx_presence_userId_unique"),
		},
		{
			// Speeds up "find all searching users" queries for matchmaking
			Keys:    bson.D{{Key: "isSearching", Value: 1}},
			Options: options.Index().SetName("idx_presence_isSearching"),
		},
		{
			// Speeds up "who is online?" queries (lastActiveAt > now - 30s)
			Keys:    bson.D{{Key: "lastActiveAt", Value: -1}},
			Options: options.Index().SetName("idx_presence_lastActiveAt"),
		},
	}

	if _, err := DB.Collection("presence").Indexes().CreateMany(ctx, presenceIndexes); err != nil {
		log.Fatalf("Failed to create presence indexes: %v", err)
	}

	// ── friend_requests collection indexes ────────────────────────────────────
	frIndexes := []mongo.IndexModel{
		{
			// Quickly find all pending requests received by a user
			Keys:    bson.D{{Key: "toId", Value: 1}, {Key: "status", Value: 1}},
			Options: options.Index().SetName("idx_fr_toId_status"),
		},
		{
			// Prevent duplicate pending requests in the same match
			Keys:    bson.D{{Key: "matchId", Value: 1}, {Key: "fromId", Value: 1}, {Key: "status", Value: 1}},
			Options: options.Index().SetName("idx_fr_match_from_status"),
		},
	}
	if _, err := DB.Collection("friend_requests").Indexes().CreateMany(ctx, frIndexes); err != nil {
		log.Fatalf("Failed to create friend_requests indexes: %v", err)
	}

	// ── friendships collection indexes ────────────────────────────────────────
	fsIndexes := []mongo.IndexModel{
		{
			// Quickly find all friends of a user (either side of the pair)
			Keys:    bson.D{{Key: "user1Id", Value: 1}},
			Options: options.Index().SetName("idx_fs_user1Id"),
		},
		{
			Keys:    bson.D{{Key: "user2Id", Value: 1}},
			Options: options.Index().SetName("idx_fs_user2Id"),
		},
	}
	if _, err := DB.Collection("friendships").Indexes().CreateMany(ctx, fsIndexes); err != nil {
		log.Fatalf("Failed to create friendships indexes: %v", err)
	}

	fmt.Println("✅ MongoDB indexes ready")
}

// GetCollection returns a handle to the named collection.
// Example: config.GetCollection("users")
func GetCollection(name string) *mongo.Collection {
	return DB.Collection(name)
}
