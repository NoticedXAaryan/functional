package main

import (
	"context"
	"flag"
	"github.com/balsuraksha/api/internal/db"
	"log"
	"os"
	"time"
)

func main() {
	path := flag.String("path", "../../db/migrations", "migration directory")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, os.Getenv("DATABASE_URL"), *path)
	if err != nil {
		log.Fatal(err)
	}
	pool.Close()
}
