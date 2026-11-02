package store

import (
	"errors"
	"fmt"
	"time"
)

const maxAmount = 1e12

type Validatable interface {
	Validate() error
}

func (t Transaction) Validate() error {
	if toCents(t.Amount) <= 0 {
		return errors.New("transaction amount must be positive (at least 0.01)")
	}
	if t.Amount >= maxAmount {
		return errors.New("transaction amount is too large")
	}
	if t.Category == "" {
		return errors.New("transaction category cannot be empty")
	}
	if _, err := time.Parse(time.DateOnly, t.Date); err != nil {
		return fmt.Errorf("invalid transaction date %q, expected YYYY-MM-DD", t.Date)
	}
	return nil
}

func (b Budget) Validate() error {
	if toCents(b.Limit) <= 0 {
		return errors.New("budget limit must be positive (at least 0.01)")
	}
	if b.Limit >= maxAmount {
		return errors.New("budget limit is too large")
	}
	if b.Category == "" {
		return errors.New("budget category cannot be empty")
	}
	return nil
}
