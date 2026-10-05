package main

import (
	"bufio"
	"fmt"
	"log"
	"os"

	"ledger/store"
)

func main() {
	log.Println("Ledger service started")

	seedTransactions := []store.Transaction{
		{Amount: 1200.50, Category: "Food", Description: "Groceries", Date: "2026-10-01"},
		{Amount: 350, Category: "Transport", Description: "Taxi ride", Date: "2026-10-02"},
		{Amount: 5000, Category: "Rent", Description: "Monthly rent", Date: "2026-10-03"},
	}
	for _, tx := range seedTransactions {
		if err := store.AddTransaction(tx); err != nil {
			log.Printf("failed to add transaction: %v", err)
		}
	}

	fmt.Println("\n--- Budget scenarios for category 'еда' (limit 5000) ---")

	tx1 := store.Transaction{Amount: 3000, Category: "еда", Description: "Супермаркет", Date: "2026-10-04"}
	if err := store.AddTransaction(tx1); err != nil {
		fmt.Printf("REJECTED: %+v -> %v\n", tx1, err)
	} else {
		fmt.Printf("ACCEPTED: %+v\n", tx1)
	}

	tx2 := store.Transaction{Amount: 1500, Category: "еда", Description: "Кафе", Date: "2026-10-05"}
	if err := store.AddTransaction(tx2); err != nil {
		fmt.Printf("REJECTED: %+v -> %v\n", tx2, err)
	} else {
		fmt.Printf("ACCEPTED: %+v\n", tx2)
	}

	tx3 := store.Transaction{Amount: 1000, Category: "еда", Description: "Ресторан", Date: "2026-10-06"}
	if err := store.AddTransaction(tx3); err != nil {
		fmt.Printf("REJECTED: %+v -> %v\n", tx3, err)
	} else {
		fmt.Printf("ACCEPTED: %+v\n", tx3)
	}

	store.SetBudget(store.Budget{Category: "спорт", Limit: 2000, Period: "month"})
	tx4 := store.Transaction{Amount: 2500, Category: "спорт", Description: "Абонемент", Date: "2026-10-07"}
	if err := store.AddTransaction(tx4); err != nil {
		fmt.Printf("REJECTED: %+v -> %v\n", tx4, err)
	} else {
		fmt.Printf("ACCEPTED: %+v\n", tx4)
	}

	fmt.Println("\n--- Loading budgets from budgets.json ---")
	file, err := os.Open("budgets.json")
	if err != nil {
		log.Printf("failed to open budgets.json: %v", err)
	} else {
		defer file.Close()
		reader := bufio.NewReader(file)
		if err := store.LoadBudgets(reader); err != nil {
			log.Printf("failed to load budgets: %v", err)
		} else {
			fmt.Println("Budgets loaded successfully")

			tx5 := store.Transaction{Amount: 2500, Category: "транспорт", Description: "Такси", Date: "2026-10-08"}
			if err := store.AddTransaction(tx5); err != nil {
				fmt.Printf("REJECTED: %+v -> %v\n", tx5, err)
			} else {
				fmt.Printf("ACCEPTED: %+v\n", tx5)
			}
		}
	}

	fmt.Println("\nTransactions list:")
	for _, tx := range store.ListTransactions() {
		fmt.Printf("ID: %d | Amount: %.2f | Category: %s | Description: %s | Date: %s\n",
			tx.ID, tx.Amount, tx.Category, tx.Description, tx.Date)
	}
}
