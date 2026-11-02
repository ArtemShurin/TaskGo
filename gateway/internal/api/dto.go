package api

import "ledger/store"

type CreateTransactionRequest struct {
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

type TransactionResponse struct {
	ID          int     `json:"id"`
	Amount      float64 `json:"amount"`
	Category    string  `json:"category"`
	Description string  `json:"description"`
	Date        string  `json:"date"`
}

type CreateBudgetRequest struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
}

type BudgetResponse struct {
	Category string  `json:"category"`
	Limit    float64 `json:"limit"`
	Period   string  `json:"period"`
}

type CategoryTotalResponse struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

type ReportSummaryResponse struct {
	From       string                  `json:"from"`
	To         string                  `json:"to"`
	Total      float64                 `json:"total"`
	ByCategory []CategoryTotalResponse `json:"by_category"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (r CreateTransactionRequest) ToDomain() store.Transaction {
	return store.Transaction{
		Amount:      r.Amount,
		Category:    r.Category,
		Description: r.Description,
		Date:        r.Date,
	}
}

func NewTransactionResponse(tx store.Transaction) TransactionResponse {
	return TransactionResponse{
		ID:          tx.ID,
		Amount:      tx.Amount,
		Category:    tx.Category,
		Description: tx.Description,
		Date:        tx.Date,
	}
}

func (r CreateBudgetRequest) ToDomain() store.Budget {
	return store.Budget{
		Category: r.Category,
		Limit:    r.Limit,
		Period:   "month",
	}
}

func NewReportSummaryResponse(r store.ReportSummary) ReportSummaryResponse {
	resp := ReportSummaryResponse{
		From:       r.From,
		To:         r.To,
		Total:      r.Total,
		ByCategory: make([]CategoryTotalResponse, 0, len(r.ByCategory)),
	}
	for _, ct := range r.ByCategory {
		resp.ByCategory = append(resp.ByCategory, CategoryTotalResponse{Category: ct.Category, Total: ct.Total})
	}
	return resp
}

func NewBudgetResponse(b store.Budget) BudgetResponse {
	return BudgetResponse{
		Category: b.Category,
		Limit:    b.Limit,
		Period:   b.Period,
	}
}
