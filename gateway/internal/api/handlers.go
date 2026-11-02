package api

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"ledger/store"
)

func pingHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, "pong")
}

func createTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	tx := req.ToDomain()
	if err := tx.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := store.AddTransaction(tx)
	if errors.Is(err, store.ErrBudgetExceeded) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		log.Printf("failed to add transaction: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, NewTransactionResponse(created))
}

func listTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	transactions, err := store.ListTransactions()
	if err != nil {
		log.Printf("failed to list transactions: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]TransactionResponse, 0, len(transactions))
	for _, tx := range transactions {
		resp = append(resp, NewTransactionResponse(tx))
	}
	writeJSON(w, http.StatusOK, resp)
}

func createBudgetHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	b := req.ToDomain()
	if err := b.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := store.SetBudget(b); err != nil {
		log.Printf("failed to set budget: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, NewBudgetResponse(b))
}

func listBudgetsHandler(w http.ResponseWriter, r *http.Request) {
	budgets, err := store.ListBudgets()
	if err != nil {
		log.Printf("failed to list budgets: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := make([]BudgetResponse, 0, len(budgets))
	for _, b := range budgets {
		resp = append(resp, NewBudgetResponse(b))
	}
	writeJSON(w, http.StatusOK, resp)
}

func reportSummaryHandler(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "query parameters from and to are required (YYYY-MM-DD)")
		return
	}

	report, err := store.GetReportSummary(r.Context(), from, to)
	if errors.Is(err, store.ErrInvalidPeriod) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err != nil {
		log.Printf("failed to build report: %v", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, NewReportSummaryResponse(report))
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "not found")
}

func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, ErrorResponse{Error: msg})
}
