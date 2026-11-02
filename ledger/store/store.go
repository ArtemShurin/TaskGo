package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"sync"
	"time"

	"ledger/cache"
	"ledger/db"
)

var ErrBudgetExceeded = errors.New("budget exceeded")

const (
	budgetPeriod    = "month"
	BudgetsCacheKey = "budgets:all"
	budgetsCacheTTL = 20 * time.Second
)

type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        string
}

type Budget struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
	Period   string  `json:"period"`
}

var addMu sync.Mutex

func AddTransaction(tx Transaction) (Transaction, error) {
	if err := tx.Validate(); err != nil {
		return Transaction{}, err
	}

	ctx := context.Background()

	addMu.Lock()
	defer addMu.Unlock()

	var limit float64
	err := db.DB.QueryRowContext(ctx,
		`SELECT limit_amount FROM budgets WHERE category = $1`, tx.Category,
	).Scan(&limit)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return Transaction{}, fmt.Errorf("get budget: %w", err)
	default:
		var spent float64
		if err := db.DB.QueryRowContext(ctx,
			`SELECT COALESCE(SUM(amount), 0) FROM expenses WHERE category = $1`, tx.Category,
		).Scan(&spent); err != nil {
			return Transaction{}, fmt.Errorf("get spent amount: %w", err)
		}
		if toCents(spent)+toCents(tx.Amount) > toCents(limit) {
			return Transaction{}, ErrBudgetExceeded
		}
	}

	if err := db.DB.QueryRowContext(ctx,
		`INSERT INTO expenses (amount, category, description, date)
		 VALUES ($1, $2, $3, $4) RETURNING id`,
		tx.Amount, tx.Category, tx.Description, tx.Date,
	).Scan(&tx.ID); err != nil {
		return Transaction{}, fmt.Errorf("insert expense: %w", err)
	}
	return tx, nil
}

func ListTransactions() ([]Transaction, error) {
	rows, err := db.DB.QueryContext(context.Background(),
		`SELECT id, amount, category, description, date
		 FROM expenses ORDER BY date DESC, id DESC`)
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	defer rows.Close()

	result := []Transaction{}
	for rows.Next() {
		var (
			tx          Transaction
			description sql.NullString
			date        time.Time
		)
		if err := rows.Scan(&tx.ID, &tx.Amount, &tx.Category, &description, &date); err != nil {
			return nil, fmt.Errorf("scan expense: %w", err)
		}
		tx.Description = description.String
		tx.Date = date.Format(time.DateOnly)
		result = append(result, tx)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	return result, nil
}

func SetBudget(b Budget) error {
	if err := b.Validate(); err != nil {
		return err
	}
	ctx := context.Background()
	if err := upsertBudget(ctx, db.DB, b); err != nil {
		return fmt.Errorf("upsert budget: %w", err)
	}
	invalidateBudgets(ctx)
	return nil
}

func invalidateBudgets(ctx context.Context) {
	if !cache.Enabled() {
		return
	}
	if err := cache.Delete(ctx, BudgetsCacheKey); err != nil {
		log.Printf("cache: %v", err)
	}
}

func ListBudgets() ([]Budget, error) {
	ctx := context.Background()

	useCache := cache.Enabled()
	if useCache {
		var cached []Budget
		hit, err := cache.GetJSON(ctx, BudgetsCacheKey, &cached)
		if err != nil {
			log.Printf("cache: %v", err)
			useCache = false
		}
		if hit {
			return cached, nil
		}
	}

	result, err := queryBudgets(ctx)
	if err != nil {
		return nil, err
	}

	if useCache {
		if err := cache.SetJSON(ctx, BudgetsCacheKey, result, budgetsCacheTTL); err != nil {
			log.Printf("cache: %v", err)
		}
	}
	return result, nil
}

func queryBudgets(ctx context.Context) ([]Budget, error) {
	rows, err := db.DB.QueryContext(ctx,
		`SELECT category, limit_amount FROM budgets ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()

	result := []Budget{}
	for rows.Next() {
		b := Budget{Period: budgetPeriod}
		if err := rows.Scan(&b.Category, &b.Limit); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		result = append(result, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	return result, nil
}

func Reset() error {
	ctx := context.Background()
	if _, err := db.DB.ExecContext(ctx,
		`TRUNCATE expenses, budgets RESTART IDENTITY`); err != nil {
		return fmt.Errorf("reset: %w", err)
	}
	if cache.Enabled() {
		if err := cache.Delete(ctx, BudgetsCacheKey); err != nil {
			return fmt.Errorf("reset cache: %w", err)
		}
		if err := cache.DeleteByPattern(ctx, "report:summary:*"); err != nil {
			return fmt.Errorf("reset cache: %w", err)
		}
	}
	return nil
}

func LoadBudgets(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("failed to read budgets: %w", err)
	}

	var list []Budget
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("failed to parse budgets JSON: %w", err)
	}

	for _, b := range list {
		if err := b.Validate(); err != nil {
			return fmt.Errorf("invalid budget %q: %w", b.Category, err)
		}
	}

	ctx := context.Background()
	sqlTx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer sqlTx.Rollback()

	for _, b := range list {
		if err := upsertBudget(ctx, sqlTx, b); err != nil {
			return fmt.Errorf("upsert budget %q: %w", b.Category, err)
		}
	}
	if err := sqlTx.Commit(); err != nil {
		return fmt.Errorf("commit budgets: %w", err)
	}
	invalidateBudgets(ctx)
	return nil
}

type execer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func upsertBudget(ctx context.Context, e execer, b Budget) error {
	_, err := e.ExecContext(ctx,
		`INSERT INTO budgets (category, limit_amount) VALUES ($1, $2)
		 ON CONFLICT (category) DO UPDATE SET limit_amount = EXCLUDED.limit_amount`,
		b.Category, b.Limit)
	return err
}

func toCents(v float64) int64 {
	return int64(math.Round(v * 100))
}
