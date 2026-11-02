package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"ledger/cache"
	"ledger/db"
	"ledger/store"
)

func CheckValid(v store.Validatable) error {
	return v.Validate()
}

func main() {
	log.Println("Ledger service started")

	dsn := db.DSNFromEnv()
	conn, err := db.Connect(context.Background(), dsn)
	if err != nil {
		log.Fatalf("failed to connect to database: %v\nCheck that PostgreSQL is running and DATABASE_URL or DB_HOST/DB_PORT/DB_USER/DB_PASS/DB_NAME are set correctly", err)
	}
	defer conn.Close()
	log.Printf("connected to database %s", db.Redact(dsn))

	if cfg, err := cache.ConnectFromEnv(context.Background()); err != nil {
		log.Printf("cache disabled, working without Redis: %v", err)
	} else {
		defer cache.Client.Close()
		log.Printf("connected to redis %s (db %d)", cfg.Addr, cfg.DB)
	}

	fmt.Println("\nloading budgets.json")
	if err := loadBudgetsFile("budgets.json"); err != nil {
		log.Printf("failed to load budgets: %v", err)
	} else {
		fmt.Println("budgets loaded")
	}

	fmt.Println("\nvalidation via Validatable interface:")
	items := []store.Validatable{
		store.Transaction{Amount: 100, Category: "еда", Date: "2026-10-09"},
		store.Transaction{Amount: -50, Category: "еда", Date: "2026-10-09"},
		store.Transaction{Amount: 100, Category: "", Date: "2026-10-09"},
		store.Transaction{Amount: 100, Category: "еда", Date: "09.10.2026"},
		store.Budget{Category: "книги", Limit: 1000, Period: "month"},
		store.Budget{Category: "книги", Limit: 0, Period: "month"},
		store.Budget{Category: "", Limit: 1000, Period: "month"},
	}
	for _, v := range items {
		if err := CheckValid(v); err != nil {
			log.Printf("invalid %T: %v", v, err)
		} else {
			fmt.Printf("valid %T: %+v\n", v, v)
		}
	}

	budgets, err := store.ListBudgets()
	if err != nil {
		log.Fatalf("failed to list budgets: %v", err)
	}
	fmt.Println("\nbudgets:")
	for _, b := range budgets {
		fmt.Printf("%s: %.2f per %s\n", b.Category, b.Limit, b.Period)
	}

	transactions, err := store.ListTransactions()
	if err != nil {
		log.Fatalf("failed to list transactions: %v", err)
	}
	fmt.Println("\ntransactions:")
	for _, tx := range transactions {
		fmt.Printf("%d) %s %.2f - %s (%s)\n", tx.ID, tx.Category, tx.Amount, tx.Description, tx.Date)
	}

	from, to := "2025-01-01", time.Now().Format(time.DateOnly)
	fmt.Printf("\nreport summary %s .. %s (called twice: miss, then hit):\n", from, to)
	var report store.ReportSummary
	for range 2 {
		start := time.Now()
		report, err = store.GetReportSummary(context.Background(), from, to)
		if err != nil {
			log.Fatalf("failed to build report: %v", err)
		}
		fmt.Printf("total %.2f, categories %d, took %s\n", report.Total, len(report.ByCategory), time.Since(start))
	}
	for _, ct := range report.ByCategory {
		fmt.Printf("  %s: %.2f\n", ct.Category, ct.Total)
	}
}

func loadBudgetsFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return store.LoadBudgets(bufio.NewReader(file))
}
