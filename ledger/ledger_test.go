package main

import (
	"errors"
	"strings"
	"testing"

	"ledger/dbtest"
	"ledger/store"
)

func TestTransactionValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tx      store.Transaction
		wantErr bool
	}{
		{"valid", store.Transaction{Amount: 450, Category: "еда", Description: "ланч", Date: "2025-09-10"}, false},
		{"valid without description", store.Transaction{Amount: 0.01, Category: "еда", Date: "2025-09-10"}, false},
		{"zero amount", store.Transaction{Amount: 0, Category: "еда", Date: "2025-09-10"}, true},
		{"negative amount", store.Transaction{Amount: -100, Category: "еда", Date: "2025-09-10"}, true},
		{"amount below one cent", store.Transaction{Amount: 0.001, Category: "еда", Date: "2025-09-10"}, true},
		{"amount too large", store.Transaction{Amount: 1e12, Category: "еда", Date: "2025-09-10"}, true},
		{"empty category", store.Transaction{Amount: 100, Category: "", Date: "2025-09-10"}, true},
		{"empty date", store.Transaction{Amount: 100, Category: "еда", Date: ""}, true},
		{"wrong date format", store.Transaction{Amount: 100, Category: "еда", Date: "10.09.2025"}, true},
		{"non-existent date", store.Transaction{Amount: 100, Category: "еда", Date: "2025-02-30"}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErr(t, tc.tx.Validate(), tc.wantErr)
		})
	}
}

func TestBudgetValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		budget  store.Budget
		wantErr bool
	}{
		{"positive limit", store.Budget{Category: "еда", Limit: 5000, Period: "month"}, false},
		{"small positive limit", store.Budget{Category: "еда", Limit: 0.01}, false},
		{"zero limit", store.Budget{Category: "еда", Limit: 0}, true},
		{"negative limit", store.Budget{Category: "еда", Limit: -1}, true},
		{"limit below one cent", store.Budget{Category: "еда", Limit: 0.004}, true},
		{"limit too large", store.Budget{Category: "еда", Limit: 1e12}, true},
		{"empty category", store.Budget{Category: "", Limit: 5000}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErr(t, tc.budget.Validate(), tc.wantErr)
		})
	}
}

func TestCheckValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		v       store.Validatable
		wantErr bool
	}{
		{"valid transaction", store.Transaction{Amount: 100, Category: "еда", Date: "2025-09-10"}, false},
		{"invalid transaction", store.Transaction{Amount: -1, Category: "еда", Date: "2025-09-10"}, true},
		{"valid budget", store.Budget{Category: "еда", Limit: 1000}, false},
		{"invalid budget", store.Budget{Category: "еда", Limit: 0}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertErr(t, CheckValid(tc.v), tc.wantErr)
		})
	}
}

func TestBudgetScenario(t *testing.T) {
	resetStore(t)

	t.Run("set budget", func(t *testing.T) {
		assertErr(t, store.SetBudget(store.Budget{Category: "еда", Limit: 5000}), false)
	})

	t.Run("within limit", func(t *testing.T) {
		_, err := store.AddTransaction(store.Transaction{Amount: 3000, Category: "еда", Date: "2025-09-10"})
		assertErr(t, err, false)
		assertTransactionCount(t, 1)
	})

	t.Run("exceeded", func(t *testing.T) {
		_, err := store.AddTransaction(store.Transaction{Amount: 2500, Category: "еда", Date: "2025-09-11"})
		if !errors.Is(err, store.ErrBudgetExceeded) {
			t.Fatalf("got error %v, want %v", err, store.ErrBudgetExceeded)
		}
		assertTransactionCount(t, 1)
	})

	t.Run("exactly at limit", func(t *testing.T) {
		_, err := store.AddTransaction(store.Transaction{Amount: 2000, Category: "еда", Date: "2025-09-12"})
		assertErr(t, err, false)
		assertTransactionCount(t, 2)
	})

	t.Run("other category unaffected", func(t *testing.T) {
		_, err := store.AddTransaction(store.Transaction{Amount: 10000, Category: "аренда", Date: "2025-09-13"})
		assertErr(t, err, false)
		assertTransactionCount(t, 3)
	})
}

func TestStore(t *testing.T) {
	t.Run("assigns sequential IDs", func(t *testing.T) {
		resetStore(t)

		first, err := store.AddTransaction(store.Transaction{Amount: 100, Category: "еда", Date: "2025-09-10"})
		assertErr(t, err, false)
		second, err := store.AddTransaction(store.Transaction{Amount: 200, Category: "еда", Date: "2025-09-11"})
		assertErr(t, err, false)

		if first.ID != 1 || second.ID != 2 {
			t.Fatalf("got IDs %d and %d, want 1 and 2", first.ID, second.ID)
		}
	})

	t.Run("list transactions round-trip and order", func(t *testing.T) {
		resetStore(t)

		inputs := []store.Transaction{
			{Amount: 100.5, Category: "еда", Description: "ланч", Date: "2025-09-10"},
			{Amount: 200, Category: "такси", Date: "2025-09-12"},
			{Amount: 300.25, Category: "еда", Description: "ужин", Date: "2025-09-10"},
		}
		for _, tx := range inputs {
			_, err := store.AddTransaction(tx)
			assertErr(t, err, false)
		}

		got, err := store.ListTransactions()
		assertErr(t, err, false)
		want := []store.Transaction{
			{ID: 2, Amount: 200, Category: "такси", Date: "2025-09-12"},
			{ID: 3, Amount: 300.25, Category: "еда", Description: "ужин", Date: "2025-09-10"},
			{ID: 1, Amount: 100.5, Category: "еда", Description: "ланч", Date: "2025-09-10"},
		}
		if len(got) != len(want) {
			t.Fatalf("got %+v, want %+v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("item %d: got %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("budget check uses exact cents", func(t *testing.T) {
		resetStore(t)

		assertErr(t, store.SetBudget(store.Budget{Category: "кофе", Limit: 0.3}), false)
		_, err := store.AddTransaction(store.Transaction{Amount: 0.1, Category: "кофе", Date: "2025-09-10"})
		assertErr(t, err, false)
		_, err = store.AddTransaction(store.Transaction{Amount: 0.2, Category: "кофе", Date: "2025-09-10"})
		assertErr(t, err, false)
	})

	t.Run("set budget upserts", func(t *testing.T) {
		resetStore(t)

		assertErr(t, store.SetBudget(store.Budget{Category: "кино", Limit: 1000}), false)
		assertErr(t, store.SetBudget(store.Budget{Category: "кино", Limit: 1500}), false)
		assertErr(t, store.SetBudget(store.Budget{Category: "еда", Limit: 5000}), false)

		got, err := store.ListBudgets()
		assertErr(t, err, false)
		want := []store.Budget{
			{Category: "еда", Limit: 5000, Period: "month"},
			{Category: "кино", Limit: 1500, Period: "month"},
		}
		if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	})

	t.Run("rejects invalid transaction", func(t *testing.T) {
		resetStore(t)

		_, err := store.AddTransaction(store.Transaction{Amount: -1, Category: "еда", Date: "2025-09-10"})
		assertErr(t, err, true)
		assertTransactionCount(t, 0)
	})

	t.Run("rejects invalid budget", func(t *testing.T) {
		resetStore(t)

		assertErr(t, store.SetBudget(store.Budget{Category: "кино", Limit: 0}), true)
		assertBudgetCount(t, 0)
	})

	t.Run("load budgets", func(t *testing.T) {
		resetStore(t)

		input := `[{"category":"кино","limit":1000,"period":"month"},{"category":"еда","limit":5000,"period":"month"}]`
		assertErr(t, store.LoadBudgets(strings.NewReader(input)), false)

		got, err := store.ListBudgets()
		assertErr(t, err, false)
		if len(got) != 2 || got[0].Category != "еда" || got[1].Category != "кино" {
			t.Fatalf("got %+v, want budgets еда and кино sorted by category", got)
		}
	})

	t.Run("load budgets bad json", func(t *testing.T) {
		resetStore(t)

		assertErr(t, store.LoadBudgets(strings.NewReader(`{oops`)), true)
	})

	t.Run("load budgets is atomic", func(t *testing.T) {
		resetStore(t)

		input := `[{"category":"кино","limit":1000},{"category":"","limit":500}]`
		assertErr(t, store.LoadBudgets(strings.NewReader(input)), true)
		assertBudgetCount(t, 0)
	})

	t.Run("reset clears data", func(t *testing.T) {
		resetStore(t)

		assertErr(t, store.SetBudget(store.Budget{Category: "кино", Limit: 1000}), false)
		_, err := store.AddTransaction(store.Transaction{Amount: 100, Category: "кино", Date: "2025-09-10"})
		assertErr(t, err, false)

		assertErr(t, store.Reset(), false)

		assertTransactionCount(t, 0)
		assertBudgetCount(t, 0)
	})
}

func resetStore(t *testing.T) {
	t.Helper()
	dbtest.Setup(t)
	if err := store.Reset(); err != nil {
		t.Fatalf("reset store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Reset(); err != nil {
			t.Errorf("reset store: %v", err)
		}
	})
}

func assertTransactionCount(t *testing.T, want int) {
	t.Helper()
	list, err := store.ListTransactions()
	assertErr(t, err, false)
	if len(list) != want {
		t.Fatalf("got %d transactions, want %d", len(list), want)
	}
}

func assertBudgetCount(t *testing.T, want int) {
	t.Helper()
	list, err := store.ListBudgets()
	assertErr(t, err, false)
	if len(list) != want {
		t.Fatalf("got %d budgets, want %d", len(list), want)
	}
}

func assertErr(t *testing.T, err error, wantErr bool) {
	t.Helper()
	if wantErr && err == nil {
		t.Fatal("expected error, got nil")
	}
	if !wantErr && err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}
