package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"ledger/dbtest"
	"ledger/store"
)

func TestPing(t *testing.T) {
	resetStore(t)

	rec := doRequest(t, http.MethodGet, "/ping", "")
	assertStatus(t, rec, http.StatusOK)

	var body string
	decodeBody(t, rec, &body)
	if body != "pong" {
		t.Fatalf("got body %q, want %q", body, "pong")
	}
}

func TestBudgetsAPI(t *testing.T) {
	t.Run("flow", func(t *testing.T) {
		resetStore(t)

		t.Run("empty list", func(t *testing.T) {
			rec := doRequest(t, http.MethodGet, "/api/budgets", "")
			assertStatus(t, rec, http.StatusOK)
			assertEmptyArray(t, rec)
		})

		t.Run("create", func(t *testing.T) {
			rec := doRequest(t, http.MethodPost, "/api/budgets", `{"category":"еда","limit":5000}`)
			assertStatus(t, rec, http.StatusCreated)

			var got BudgetResponse
			decodeBody(t, rec, &got)
			want := BudgetResponse{Category: "еда", Limit: 5000, Period: "month"}
			if got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		})

		t.Run("list contains created", func(t *testing.T) {
			rec := doRequest(t, http.MethodGet, "/api/budgets", "")
			assertStatus(t, rec, http.StatusOK)

			var got []BudgetResponse
			decodeBody(t, rec, &got)
			want := []BudgetResponse{{Category: "еда", Limit: 5000, Period: "month"}}
			assertBudgets(t, got, want)
		})

		t.Run("update existing", func(t *testing.T) {
			rec := doRequest(t, http.MethodPost, "/api/budgets", `{"category":"еда","limit":7000}`)
			assertStatus(t, rec, http.StatusCreated)

			rec = doRequest(t, http.MethodGet, "/api/budgets", "")
			var got []BudgetResponse
			decodeBody(t, rec, &got)
			want := []BudgetResponse{{Category: "еда", Limit: 7000, Period: "month"}}
			assertBudgets(t, got, want)
		})
	})

	t.Run("invalid", func(t *testing.T) {
		tests := []struct {
			name string
			body string
		}{
			{"bad_json", `{oops`},
			{"zero_limit", `{"category":"еда","limit":0}`},
			{"negative_limit", `{"category":"еда","limit":-1}`},
			{"empty_category", `{"category":"","limit":1000}`},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				resetStore(t)

				rec := doRequest(t, http.MethodPost, "/api/budgets", tc.body)
				assertStatus(t, rec, http.StatusBadRequest)
				assertErrorBody(t, rec, "")

				if budgets, err := store.ListBudgets(); err != nil || len(budgets) != 0 {
					t.Fatalf("invalid budget was saved: got %+v, err %v", budgets, err)
				}
			})
		}
	})
}

func TestTransactionsAPI(t *testing.T) {
	t.Run("flow", func(t *testing.T) {
		resetStore(t)

		t.Run("empty list", func(t *testing.T) {
			rec := doRequest(t, http.MethodGet, "/api/transactions", "")
			assertStatus(t, rec, http.StatusOK)
			assertEmptyArray(t, rec)
		})

		t.Run("create budget", func(t *testing.T) {
			rec := doRequest(t, http.MethodPost, "/api/budgets", `{"category":"еда","limit":5000}`)
			assertStatus(t, rec, http.StatusCreated)
		})

		created := TransactionResponse{ID: 1, Amount: 3000, Category: "еда", Description: "ланч", Date: "2025-09-10"}

		t.Run("ok", func(t *testing.T) {
			rec := doRequest(t, http.MethodPost, "/api/transactions",
				`{"amount":3000,"category":"еда","description":"ланч","date":"2025-09-10"}`)
			assertStatus(t, rec, http.StatusCreated)

			var got TransactionResponse
			decodeBody(t, rec, &got)
			if got != created {
				t.Fatalf("got %+v, want %+v", got, created)
			}
		})

		t.Run("list", func(t *testing.T) {
			rec := doRequest(t, http.MethodGet, "/api/transactions", "")
			assertStatus(t, rec, http.StatusOK)

			var got []TransactionResponse
			decodeBody(t, rec, &got)
			if len(got) != 1 || got[0] != created {
				t.Fatalf("got %+v, want [%+v]", got, created)
			}
		})

		t.Run("exceeded", func(t *testing.T) {
			rec := doRequest(t, http.MethodPost, "/api/transactions",
				`{"amount":2500,"category":"еда","description":"ресторан","date":"2025-09-11"}`)
			assertStatus(t, rec, http.StatusConflict)
			assertErrorBody(t, rec, "budget exceeded")
		})

		t.Run("bad_json", func(t *testing.T) {
			rec := doRequest(t, http.MethodPost, "/api/transactions", `{"amount":100,`)
			assertStatus(t, rec, http.StatusBadRequest)
			assertErrorBody(t, rec, "")
		})

		t.Run("list unchanged after errors", func(t *testing.T) {
			rec := doRequest(t, http.MethodGet, "/api/transactions", "")

			var got []TransactionResponse
			decodeBody(t, rec, &got)
			if len(got) != 1 {
				t.Fatalf("got %d transactions, want 1", len(got))
			}
		})
	})

	t.Run("validation", func(t *testing.T) {
		tests := []struct {
			name string
			body string
		}{
			{"zero_amount", `{"amount":0,"category":"еда","date":"2025-09-10"}`},
			{"negative_amount", `{"amount":-5,"category":"еда","date":"2025-09-10"}`},
			{"empty_category", `{"amount":100,"category":"","date":"2025-09-10"}`},
			{"wrong_date", `{"amount":100,"category":"еда","date":"10.09.2025"}`},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				resetStore(t)

				rec := doRequest(t, http.MethodPost, "/api/transactions", tc.body)
				assertStatus(t, rec, http.StatusBadRequest)
				assertErrorBody(t, rec, "")

				if transactions, err := store.ListTransactions(); err != nil || len(transactions) != 0 {
					t.Fatalf("invalid transaction was saved: got %+v, err %v", transactions, err)
				}
			})
		}
	})
}

func TestReportsAPI(t *testing.T) {
	const summaryURL = "/api/reports/summary?from=2025-09-01&to=2025-09-30"

	t.Run("ok", func(t *testing.T) {
		resetStore(t)
		doRequest(t, http.MethodPost, "/api/transactions", `{"amount":450,"category":"еда","date":"2025-09-10"}`)
		doRequest(t, http.MethodPost, "/api/transactions", `{"amount":0.5,"category":"еда","date":"2025-09-30"}`)
		doRequest(t, http.MethodPost, "/api/transactions", `{"amount":300,"category":"такси","date":"2025-09-01"}`)
		doRequest(t, http.MethodPost, "/api/transactions", `{"amount":999,"category":"такси","date":"2025-10-01"}`)

		rec := doRequest(t, http.MethodGet, summaryURL, "")
		assertStatus(t, rec, http.StatusOK)

		var got ReportSummaryResponse
		decodeBody(t, rec, &got)
		want := ReportSummaryResponse{
			From: "2025-09-01", To: "2025-09-30", Total: 750.5,
			ByCategory: []CategoryTotalResponse{{Category: "еда", Total: 450.5}, {Category: "такси", Total: 300}},
		}
		assertReportResponse(t, got, want)
	})

	t.Run("empty", func(t *testing.T) {
		resetStore(t)

		rec := doRequest(t, http.MethodGet, summaryURL, "")
		assertStatus(t, rec, http.StatusOK)
		if body := strings.TrimSpace(rec.Body.String()); body != `{"from":"2025-09-01","to":"2025-09-30","total":0,"by_category":[]}` {
			t.Fatalf("got body %s", body)
		}
	})

	t.Run("cached", func(t *testing.T) {
		resetStore(t)
		dbtest.SetupCache(t)
		doRequest(t, http.MethodPost, "/api/transactions", `{"amount":100,"category":"еда","date":"2025-09-10"}`)

		first := doRequest(t, http.MethodGet, summaryURL, "")
		assertStatus(t, first, http.StatusOK)

		doRequest(t, http.MethodPost, "/api/transactions", `{"amount":50,"category":"еда","date":"2025-09-11"}`)

		second := doRequest(t, http.MethodGet, summaryURL, "")
		assertStatus(t, second, http.StatusOK)
		if first.Body.String() != second.Body.String() {
			t.Fatalf("second response must come from cache:\nfirst:  %s\nsecond: %s", first.Body.String(), second.Body.String())
		}
	})

	invalid := []struct {
		name string
		url  string
	}{
		{"missing_params", "/api/reports/summary"},
		{"missing_to", "/api/reports/summary?from=2025-09-01"},
		{"bad_date", "/api/reports/summary?from=01.09.2025&to=2025-09-30"},
		{"from_after_to", "/api/reports/summary?from=2025-09-30&to=2025-09-01"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			resetStore(t)

			rec := doRequest(t, http.MethodGet, tc.url, "")
			assertStatus(t, rec, http.StatusBadRequest)
			assertErrorBody(t, rec, "")
		})
	}

	t.Run("method_not_allowed", func(t *testing.T) {
		resetStore(t)

		rec := doRequest(t, http.MethodPost, summaryURL, "")
		assertStatus(t, rec, http.StatusMethodNotAllowed)
		assertErrorBody(t, rec, "method not allowed")
	})
}

func TestRouting(t *testing.T) {
	t.Run("unknown path", func(t *testing.T) {
		resetStore(t)

		rec := doRequest(t, http.MethodGet, "/api/unknown", "")
		assertStatus(t, rec, http.StatusNotFound)
		assertErrorBody(t, rec, "not found")
	})

	t.Run("method not allowed", func(t *testing.T) {
		resetStore(t)

		rec := doRequest(t, http.MethodDelete, "/api/transactions", "")
		assertStatus(t, rec, http.StatusMethodNotAllowed)
		assertErrorBody(t, rec, "method not allowed")
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

func doRequest(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	NewRouter().ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("%s %s: got Content-Type %q, want %q", method, path, ct, "application/json; charset=utf-8")
	}
	return rec
}

func assertStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("got status %d, want %d, body: %s", rec.Code, want, rec.Body.String())
	}
}

func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var got ErrorResponse
	decodeBody(t, rec, &got)
	if got.Error == "" {
		t.Fatalf("expected non-empty error field, body: %s", rec.Body.String())
	}
	if want != "" && got.Error != want {
		t.Fatalf("got error %q, want %q", got.Error, want)
	}
}

func assertEmptyArray(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Fatalf("got body %s, want []", body)
	}
}

func assertBudgets(t *testing.T, got, want []BudgetResponse) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %+v, want %+v", got, want)
		}
	}
}

func assertReportResponse(t *testing.T, got, want ReportSummaryResponse) {
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

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("invalid JSON body %q: %v", rec.Body.String(), err)
	}
}
