package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"ledger/cache"
	"ledger/db"
	"ledger/dbtest"
	"ledger/store"
)

func TestReportSummary(t *testing.T) {
	ctx := context.Background()

	t.Run("aggregates by category within period", func(t *testing.T) {
		resetStore(t)
		addTransactions(t,
			store.Transaction{Amount: 100.10, Category: "еда", Date: "2025-08-31"},
			store.Transaction{Amount: 450, Category: "еда", Date: "2025-09-01"},
			store.Transaction{Amount: 0.20, Category: "еда", Date: "2025-09-15"},
			store.Transaction{Amount: 300, Category: "такси", Date: "2025-09-30"},
			store.Transaction{Amount: 999, Category: "такси", Date: "2025-10-01"},
		)

		got, err := store.GetReportSummary(ctx, "2025-09-01", "2025-09-30")
		assertErr(t, err, false)

		want := store.ReportSummary{
			From:  "2025-09-01",
			To:    "2025-09-30",
			Total: 750.20,
			ByCategory: []store.CategoryTotal{
				{Category: "еда", Total: 450.20},
				{Category: "такси", Total: 300},
			},
		}
		assertReport(t, got, want)
	})

	t.Run("empty period", func(t *testing.T) {
		resetStore(t)

		got, err := store.GetReportSummary(ctx, "2025-01-01", "2025-01-31")
		assertErr(t, err, false)
		assertReport(t, got, store.ReportSummary{From: "2025-01-01", To: "2025-01-31", ByCategory: []store.CategoryTotal{}})
	})

	t.Run("single day", func(t *testing.T) {
		resetStore(t)
		addTransactions(t, store.Transaction{Amount: 50, Category: "кофе", Date: "2025-09-10"})

		got, err := store.GetReportSummary(ctx, "2025-09-10", "2025-09-10")
		assertErr(t, err, false)
		if got.Total != 50 {
			t.Fatalf("got total %v, want 50", got.Total)
		}
	})

	invalid := []struct {
		name     string
		from, to string
	}{
		{"bad from", "01.09.2025", "2025-09-30"},
		{"bad to", "2025-09-01", ""},
		{"from after to", "2025-09-30", "2025-09-01"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := store.GetReportSummary(ctx, tc.from, tc.to)
			if !errors.Is(err, store.ErrInvalidPeriod) {
				t.Fatalf("got error %v, want %v", err, store.ErrInvalidPeriod)
			}
		})
	}
}

func TestReportCache(t *testing.T) {
	ctx := context.Background()
	from, to := "2025-09-01", "2025-09-30"
	key := store.ReportCacheKey(from, to)

	resetStore(t)
	dbtest.SetupCache(t)
	addTransactions(t, store.Transaction{Amount: 100, Category: "еда", Date: "2025-09-10"})

	t.Run("miss stores result with TTL", func(t *testing.T) {
		got, err := store.GetReportSummary(ctx, from, to)
		assertErr(t, err, false)
		if got.Total != 100 {
			t.Fatalf("got total %v, want 100", got.Total)
		}

		if key != "report:summary:2025-09-01:2025-09-30" {
			t.Fatalf("unexpected cache key %q", key)
		}
		ttl, err := cache.Client.TTL(ctx, key).Result()
		assertErr(t, err, false)
		if ttl <= 0 || ttl > 30*time.Second {
			t.Fatalf("got TTL %v, want (0, 30s]", ttl)
		}
	})

	t.Run("hit returns cached value", func(t *testing.T) {
		addTransactions(t, store.Transaction{Amount: 50, Category: "еда", Date: "2025-09-11"})

		got, err := store.GetReportSummary(ctx, from, to)
		assertErr(t, err, false)
		if got.Total != 100 {
			t.Fatalf("got total %v, want cached 100 (new expense must not be visible until TTL expires)", got.Total)
		}
	})

	t.Run("after expiry reads database again", func(t *testing.T) {
		assertErr(t, cache.Delete(ctx, key), false)

		got, err := store.GetReportSummary(ctx, from, to)
		assertErr(t, err, false)
		if got.Total != 150 {
			t.Fatalf("got total %v, want 150", got.Total)
		}
	})

	t.Run("reset clears report cache", func(t *testing.T) {
		assertErr(t, store.Reset(), false)
		n, err := cache.Client.Exists(ctx, key).Result()
		assertErr(t, err, false)
		if n != 0 {
			t.Fatalf("report cache key still exists after Reset")
		}
	})
}

func TestBudgetsCache(t *testing.T) {
	ctx := context.Background()

	resetStore(t)
	dbtest.SetupCache(t)
	assertErr(t, store.SetBudget(store.Budget{Category: "еда", Limit: 5000}), false)

	t.Run("list fills cache", func(t *testing.T) {
		assertBudgetCount(t, 1)

		ttl, err := cache.Client.TTL(ctx, store.BudgetsCacheKey).Result()
		assertErr(t, err, false)
		if ttl <= 0 || ttl > 30*time.Second {
			t.Fatalf("got TTL %v, want (0, 30s]", ttl)
		}
	})

	t.Run("served from cache", func(t *testing.T) {
		_, err := cache.Client.Set(ctx, store.BudgetsCacheKey,
			`[{"category":"из-кеша","limit":1,"period":"month"}]`, time.Minute).Result()
		assertErr(t, err, false)

		got, err := store.ListBudgets()
		assertErr(t, err, false)
		if len(got) != 1 || got[0].Category != "из-кеша" {
			t.Fatalf("got %+v, want value from cache", got)
		}
	})

	t.Run("set budget invalidates cache", func(t *testing.T) {
		assertErr(t, store.SetBudget(store.Budget{Category: "кино", Limit: 1000}), false)

		got, err := store.ListBudgets()
		assertErr(t, err, false)
		if len(got) != 2 || got[0].Category != "еда" || got[1].Category != "кино" {
			t.Fatalf("got %+v, want fresh budgets еда and кино", got)
		}
	})
}

func TestCacheUnavailableFallsBackToDatabase(t *testing.T) {
	resetStore(t)

	cache.Client = cache.NewClient(cache.Config{Addr: "localhost:1"})
	t.Cleanup(func() {
		cache.Client.Close()
		cache.Client = nil
	})

	assertErr(t, store.SetBudget(store.Budget{Category: "еда", Limit: 5000}), false)
	addTransactions(t, store.Transaction{Amount: 100, Category: "еда", Date: "2025-09-10"})

	start := time.Now()
	assertBudgetCount(t, 1)
	got, err := store.GetReportSummary(context.Background(), "2025-09-01", "2025-09-30")
	assertErr(t, err, false)
	if got.Total != 100 {
		t.Fatalf("got total %v, want 100", got.Total)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("fallback to database took %s, want under 1s", elapsed)
	}
}

func TestConnectionPool(t *testing.T) {
	dbtest.Setup(t)

	stats := db.DB.Stats()
	if stats.MaxOpenConnections != db.MaxOpenConns || db.MaxOpenConns != 10 {
		t.Fatalf("got MaxOpenConnections %d, want 10", stats.MaxOpenConnections)
	}
}

func addTransactions(t *testing.T, txs ...store.Transaction) {
	t.Helper()
	for _, tx := range txs {
		if _, err := store.AddTransaction(tx); err != nil {
			t.Fatalf("add transaction %+v: %v", tx, err)
		}
	}
}

func assertReport(t *testing.T, got, want store.ReportSummary) {
	t.Helper()
	if got.From != want.From || got.To != want.To || got.Total != want.Total || len(got.ByCategory) != len(want.ByCategory) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want.ByCategory {
		if got.ByCategory[i] != want.ByCategory[i] {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
}
