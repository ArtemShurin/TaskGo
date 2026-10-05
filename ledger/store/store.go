package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

var transactions []Transaction

var budgets = map[string]Budget{
	"еда": {Category: "еда", Limit: 5000, Period: "month"},
}

func AddTransaction(tx Transaction) error {
	if tx.Amount == 0 {
		return errors.New("transaction amount cannot be zero")
	}

	if b, ok := budgets[tx.Category]; ok {
		var sum float64
		for _, t := range transactions {
			if t.Category == tx.Category {
				sum += t.Amount
			}
		}
		if sum+tx.Amount > b.Limit {
			return errors.New("budget exceeded")
		}
	}

	tx.ID = len(transactions) + 1
	transactions = append(transactions, tx)
	return nil
}

func ListTransactions() []Transaction {
	result := make([]Transaction, len(transactions))
	copy(result, transactions)
	return result
}

func SetBudget(b Budget) {
	budgets[b.Category] = b
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
		SetBudget(b)
	}
	return nil
}
