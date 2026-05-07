package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"ranchat_backend/config"
)

const runs = 10 // number of queries per test
const targetDevice = "seed-device-099999" // last doc = absolute worst case for collection scan

func main() {
	_ = godotenv.Load()
	config.ConnectMongoDB()

	col := config.GetCollection("users")
	ctx := context.Background()

	// ── STEP 1: Drop the deviceId index ──────────────────────────────────────
	fmt.Println("🗑  Dropping deviceId index...")
	_, err := col.Indexes().DropOne(ctx, "idx_deviceId_unique")
	if err != nil {
		log.Printf("⚠️  Could not drop index (may not exist): %v\n", err)
	} else {
		fmt.Println("✅ Index dropped")
	}

	// ── STEP 2: Benchmark WITHOUT index (full collection scan) ────────────────
	fmt.Printf("\n📊 Running %d queries WITHOUT index (collection scan)...\n", runs)
	noIndexAvg := benchmark(ctx, col, runs)

	// ── STEP 3: Re-create the index ───────────────────────────────────────────
	fmt.Println("\n⚡ Creating deviceId index...")
	_, err = col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "deviceId", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("idx_deviceId_unique"),
	})
	if err != nil {
		log.Fatalf("❌ Failed to create index: %v", err)
	}
	fmt.Println("✅ Index created")

	// ── STEP 4: Benchmark WITH index (B-tree lookup) ──────────────────────────
	fmt.Printf("\n📊 Running %d queries WITH index (B-tree lookup)...\n", runs)
	withIndexAvg := benchmark(ctx, col, runs)

	// ── STEP 5: Results ───────────────────────────────────────────────────────
	speedup := noIndexAvg / withIndexAvg
	fmt.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("  ❌ Without index : %.3f ms avg\n", noIndexAvg)
	fmt.Printf("  ✅ With index    : %.3f ms avg\n", withIndexAvg)
	fmt.Printf("  🚀 Speedup       : %.1fx faster\n", speedup)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

func benchmark(ctx context.Context, col *mongo.Collection, n int) float64 {
	var total time.Duration

	for i := 0; i < n; i++ {
		start := time.Now()

		var result bson.M
		err := col.FindOne(ctx, bson.M{"deviceId": targetDevice}).Decode(&result)
		elapsed := time.Since(start)

		if err != nil {
			log.Printf("Query error: %v", err)
			continue
		}
		fmt.Printf("  run %2d: %v\n", i+1, elapsed.Round(time.Microsecond))
		total += elapsed
	}

	return float64(total.Milliseconds()) / float64(n)
}
