package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Transaction represents a single financial transaction.
type Transaction struct {
	ID          int
	Amount      float64
	Category    string
	Description string
	Date        string
}

// Budget represents a spending limit set for a category.
type Budget struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
	Period   string  `json:"period"`
}

var transactions []Transaction

var budgets = map[string]Budget{
	"еда": {Category: "еда", Limit: 5000, Period: "month"},
}

// AddTransaction appends tx to the in-memory store, assigning it a new ID.
// It returns an error if the transaction amount is zero, or if a budget is
// set for tx.Category and adding tx would exceed that budget's limit.
func AddTransaction(tx Transaction) error {
	if tx.Amount == 0 {
		return errors.New("transaction amount cannot be zero")
	}

	if budget, ok := budgets[tx.Category]; ok {
		var spent float64
		for _, existing := range transactions {
			if existing.Category == tx.Category {
				spent += existing.Amount
			}
		}
		if spent+tx.Amount > budget.Limit {
			return errors.New("budget exceeded")
		}
	}

	tx.ID = len(transactions) + 1
	transactions = append(transactions, tx)
	return nil
}

// ListTransactions returns a copy of all stored transactions.
func ListTransactions() []Transaction {
	result := make([]Transaction, len(transactions))
	copy(result, transactions)
	return result
}

// SetBudget adds a new budget or updates the limit for an existing category.
func SetBudget(b Budget) {
	budgets[b.Category] = b
}

// LoadBudgets reads a JSON array of budgets from r and registers each one
// via SetBudget. It returns an error if reading or parsing fails.
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
		SetBudget(b)
	}
	return nil
}
