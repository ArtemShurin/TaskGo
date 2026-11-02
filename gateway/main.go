package main

import (
	"context"
	"log"
	"net/http"

	"gateway/internal/api"
	"ledger/cache"
	"ledger/db"
)

func main() {
	dsn := db.DSNFromEnv()
	conn, err := db.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v\nCheck that PostgreSQL is running, migrations are applied and DATABASE_URL or DB_HOST/DB_PORT/DB_USER/DB_PASS/DB_NAME are set correctly", err)
	}
	defer conn.Close()
	log.Printf("connected to database %s", db.Redact(dsn))

	if cfg, err := cache.ConnectFromEnv(context.Background()); err != nil {
		log.Printf("cache disabled, working without Redis: %v", err)
	} else {
		defer cache.Client.Close()
		log.Printf("connected to redis %s (db %d)", cfg.Addr, cfg.DB)
	}

	addr := ":8080"
	log.Printf("Gateway service started on %s", addr)
	if err := http.ListenAndServe(addr, api.NewRouter()); err != nil {
		log.Fatal(err)
	}
}
