package config

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Config struct {
	PORT       string
	MONGO_URI  string
	JWT_SECRET string
	REDIS_URL  string
}

// Load environment variables
func (c *Config) LoadEnvs() error {
	err := godotenv.Load(".env")

	if err != nil {
		return err
	}
	c.PORT = os.Getenv("PORT")
	c.JWT_SECRET = os.Getenv("JWT_SECRET")
	c.MONGO_URI = os.Getenv("MONGO_URI")
	c.REDIS_URL = os.Getenv("REDIS_URL")
	return nil
}

// Connect to MongoDB with connection pool
func ConnectMongo(uri string) (*mongo.Client, *mongo.Database) {
	clientOptions := options.Client().
		ApplyURI(uri).
		SetMaxPoolSize(300).                 // ⚡ Increase max pool size (e.g., from 50 to 300)
		SetMinPoolSize(50).                  // ⚡ Pre-warm idle connections (e.g., from 10 to 50)
		SetMaxConnIdleTime(30 * time.Second) // Idle timeout

	client, err := mongo.Connect(clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Ping(ctx, nil); err != nil {
		log.Fatal(err)
	}

	log.Println("Connected to MongoDB with pool")
	return client, client.Database("ranchat")
}

// Graceful disconnect
func DisconnectMongo(client *mongo.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := client.Disconnect(ctx); err != nil {
		log.Fatal(err)
	}
}
