package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"ledger/store"
)

func tryAdd(tx store.Transaction) {
	if err := store.AddTransaction(tx); err != nil {
		fmt.Printf("REJECTED: %s %.2f -> %v\n", tx.Category, tx.Amount, err)
		return
	}
	fmt.Printf("ACCEPTED: %s %.2f\n", tx.Category, tx.Amount)
}

func main() {
	log.Println("Ledger service started")

	seed := []store.Transaction{
		{Amount: 1200.50, Category: "Food", Description: "Groceries", Date: "2026-10-01"},
		{Amount: 350, Category: "Transport", Description: "Taxi ride", Date: "2026-10-02"},
		{Amount: 5000, Category: "Rent", Description: "Monthly rent", Date: "2026-10-03"},
	}
	for _, tx := range seed {
		if err := store.AddTransaction(tx); err != nil {
			log.Printf("failed to add transaction: %v", err)
		}
	}

	fmt.Println("\nbudget checks for 'еда' (limit 5000):")
	tryAdd(store.Transaction{Amount: 3000, Category: "еда", Description: "Супермаркет", Date: "2026-10-04"})
	tryAdd(store.Transaction{Amount: 1500, Category: "еда", Description: "Кафе", Date: "2026-10-05"})
	tryAdd(store.Transaction{Amount: 1000, Category: "еда", Description: "Ресторан", Date: "2026-10-06"}) // 4500+1000 > 5000, должно отклониться

	store.SetBudget(store.Budget{Category: "спорт", Limit: 2000, Period: "month"})
	tryAdd(store.Transaction{Amount: 2500, Category: "спорт", Description: "Абонемент", Date: "2026-10-07"})

	fmt.Println("\nloading budgets.json")
	file, err := os.Open("budgets.json")
	if err != nil {
		log.Printf("failed to open budgets.json: %v", err)
	} else {
		defer file.Close()
		if err := store.LoadBudgets(bufio.NewReader(file)); err != nil {
			log.Printf("failed to load budgets: %v", err)
		} else {
			fmt.Println("budgets loaded")
			tryAdd(store.Transaction{Amount: 2500, Category: "транспорт", Description: "Такси", Date: "2026-10-08"})
		}
	}

	fmt.Println("\ntransactions:")
	for _, tx := range store.ListTransactions() {
		fmt.Printf("%d) %s %.2f - %s (%s)\n", tx.ID, tx.Category, tx.Amount, tx.Description, tx.Date)
	}
}
