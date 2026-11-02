package store

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"ledger/cache"
	"ledger/db"
)

const reportCacheTTL = 30 * time.Second

var ErrInvalidPeriod = errors.New("invalid report period")

type CategoryTotal struct {
	Category string  `json:"category"`
	Total    float64 `json:"total"`
}

type ReportSummary struct {
	From       string          `json:"from"`
	To         string          `json:"to"`
	Total      float64         `json:"total"`
	ByCategory []CategoryTotal `json:"by_category"`
}

func GetReportSummary(ctx context.Context, from, to string) (ReportSummary, error) {
	fromDate, err := time.Parse(time.DateOnly, from)
	if err != nil {
		return ReportSummary{}, fmt.Errorf("%w: from %q, expected YYYY-MM-DD", ErrInvalidPeriod, from)
	}
	toDate, err := time.Parse(time.DateOnly, to)
	if err != nil {
		return ReportSummary{}, fmt.Errorf("%w: to %q, expected YYYY-MM-DD", ErrInvalidPeriod, to)
	}
	if toDate.Before(fromDate) {
		return ReportSummary{}, fmt.Errorf("%w: from %s is after to %s", ErrInvalidPeriod, from, to)
	}

	key := ReportCacheKey(from, to)
	useCache := cache.Enabled()
	if useCache {
		var cached ReportSummary
		hit, err := cache.GetJSON(ctx, key, &cached)
		if err != nil {
			log.Printf("cache: %v", err)
			useCache = false
		}
		if hit {
			log.Printf("cache hit: %s", key)
			return cached, nil
		}
		log.Printf("cache miss: %s", key)
	}

	report, err := queryReportSummary(ctx, from, to)
	if err != nil {
		return ReportSummary{}, err
	}

	if useCache {
		if err := cache.SetJSON(ctx, key, report, reportCacheTTL); err != nil {
			log.Printf("cache: %v", err)
		}
	}
	return report, nil
}

func ReportCacheKey(from, to string) string {
	return "report:summary:" + from + ":" + to
}

func queryReportSummary(ctx context.Context, from, to string) (ReportSummary, error) {
	rows, err := db.DB.QueryContext(ctx,
		`SELECT category, SUM(amount) FROM expenses
		 WHERE date BETWEEN $1 AND $2
		 GROUP BY category ORDER BY category`, from, to)
	if err != nil {
		return ReportSummary{}, fmt.Errorf("report summary: %w", err)
	}
	defer rows.Close()

	report := ReportSummary{From: from, To: to, ByCategory: []CategoryTotal{}}
	var totalCents int64
	for rows.Next() {
		var ct CategoryTotal
		if err := rows.Scan(&ct.Category, &ct.Total); err != nil {
			return ReportSummary{}, fmt.Errorf("scan report row: %w", err)
		}
		totalCents += toCents(ct.Total)
		report.ByCategory = append(report.ByCategory, ct)
	}
	if err := rows.Err(); err != nil {
		return ReportSummary{}, fmt.Errorf("report summary: %w", err)
	}
	report.Total = float64(totalCents) / 100
	return report, nil
}
