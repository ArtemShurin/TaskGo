package api

import "net/http"

func NewRouter() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /ping", pingHandler)

	mux.HandleFunc("POST /api/transactions", createTransactionHandler)
	mux.HandleFunc("GET /api/transactions", listTransactionsHandler)
	mux.HandleFunc("/api/transactions", methodNotAllowedHandler)

	mux.HandleFunc("POST /api/budgets", createBudgetHandler)
	mux.HandleFunc("GET /api/budgets", listBudgetsHandler)
	mux.HandleFunc("/api/budgets", methodNotAllowedHandler)

	mux.HandleFunc("GET /api/reports/summary", reportSummaryHandler)
	mux.HandleFunc("/api/reports/summary", methodNotAllowedHandler)

	mux.HandleFunc("/", notFoundHandler)

	return LoggingMiddleware(JSONContentTypeMiddleware(mux))
}
