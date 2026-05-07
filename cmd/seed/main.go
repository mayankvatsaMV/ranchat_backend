package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"ranchat_backend/config"
	"ranchat_backend/models"
)

var firstNames = []string{
	"Aarav", "Vivaan", "Aditya", "Vihaan", "Arjun", "Sai", "Reyansh", "Ayaan",
	"Krishna", "Ishaan", "Shaurya", "Atharv", "Advik", "Pranav", "Advait",
	"Dhruv", "Kabir", "Ritvik", "Aarush", "Darsh", "Priya", "Ananya", "Pooja",
	"Sneha", "Riya", "Kavya", "Meera", "Divya", "Anjali", "Nisha", "Sara",
	"Isha", "Tanya", "Shreya", "Nikita", "Aisha", "Zara", "Sia", "Mia", "Aria",
	"Rohan", "Rahul", "Raj", "Karan", "Nikhil", "Amit", "Vikram", "Aryan",
	"Dev", "Jay", "Sam", "Alex", "Chris", "Ryan", "Noah", "Liam", "Lucas",
}

var interests = []string{
	"gym", "ai", "coding", "music", "travel", "food", "movies", "gaming",
	"cricket", "football", "yoga", "photography", "art", "reading", "hiking",
	"dancing", "cooking", "anime", "finance", "startups", "tech", "design",
}

var aboutTexts = []string{
	"Just here to vibe",
	"Love coding and coffee ☕",
	"Gym rat and tech geek",
	"Exploring the world one city at a time",
	"Music is my therapy",
	"Foodie at heart 🍕",
	"Building the next big thing",
	"Night owl, deep thinker",
	"Cricket > everything",
	"Life is short, travel more",
	"AI enthusiast and gamer",
	"Design is how it works",
}

func randomInterests(r *rand.Rand) []string {
	count := r.Intn(4) + 1 // 1 to 4 interests
	shuffled := make([]string, len(interests))
	copy(shuffled, interests)
	r.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })
	return shuffled[:count]
}

func main() {
	_ = godotenv.Load()
	config.ConnectMongoDB()

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	col := config.GetCollection("users")
	ctx := context.Background()

	genders := []string{"male", "female", "other"}

	const total = 100_000
	const batchSize = 5_000
	now := time.Now().UTC()
	inserted := 0

	for batch := 0; batch < total/batchSize; batch++ {
		docs := make([]interface{}, 0, batchSize)
		start := batch*batchSize + 1
		end := start + batchSize

		for i := start; i < end; i++ {
			name := firstNames[r.Intn(len(firstNames))]
			gender := genders[r.Intn(len(genders))]
			age := r.Intn(28) + 18

			user := models.User{
				ID:        primitive.NewObjectID(),
				FullName:  fmt.Sprintf("%s %d", name, i),
				Gender:    gender,
				Age:       age,
				About:     aboutTexts[r.Intn(len(aboutTexts))],
				Interests: randomInterests(r),
				DeviceID:  fmt.Sprintf("seed-device-%06d", i),
				LastSeen:  now,
				CreatedAt: now,
				UpdatedAt: now,
				// Note: isSearching, isMatched etc. are in the "presence" collection now
			}
			docs = append(docs, user)
		}

		result, err := col.InsertMany(ctx, docs)
		if err != nil {
			log.Fatalf("❌ Batch %d InsertMany failed: %v", batch+1, err)
		}
		inserted += len(result.InsertedIDs)
		fmt.Printf("  batch %2d/20 → %d users inserted\n", batch+1, inserted)
	}

	fmt.Printf("\n✅ Done — inserted %d users total\n", inserted)
}
