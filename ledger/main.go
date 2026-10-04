package main

import (
	"fmt"
	"log"

	"ledger/store"
)

func main() {
	log.Println("Ledger service started")

	testTransactions := []store.Transaction{
		{Amount: 1200.50, Category: "Food", Description: "Groceries", Date: "2026-10-01"},
		{Amount: 350, Category: "Transport", Description: "Taxi ride", Date: "2026-10-02"},
		{Amount: 5000, Category: "Rent", Description: "Monthly rent", Date: "2026-10-03"},
	}

	for _, tx := range testTransactions {
		if err := store.AddTransaction(tx); err != nil {
			log.Printf("failed to add transaction: %v", err)
		}
	}

	// also demonstrate the error path: zero-amount transaction is rejected
	if err := store.AddTransaction(store.Transaction{Category: "Invalid"}); err != nil {
		log.Printf("expected error for zero-amount transaction: %v", err)
	}

	fmt.Println("Transactions list:")
	for _, tx := range store.ListTransactions() {
		fmt.Printf("ID: %d | Amount: %.2f | Category: %s | Description: %s | Date: %s\n",
			tx.ID, tx.Amount, tx.Category, tx.Description, tx.Date)
	}
}
