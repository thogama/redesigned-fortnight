package main

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendCardTransactionCSVWritesMonthlyFileWithLocation(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	transaction := CardTransaction{
		Timestamp:       time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
		Location:        "Dl *Uber Rides",
		Currency:        "BRL",
		Amount:          -11.35,
		NativeAmountUSD: -2.21,
		Category:        "uber",
	}

	if err := appendCardTransactionCSV(transaction); err != nil {
		t.Fatalf("append card transaction csv: %v", err)
	}

	file, err := os.Open(filepath.Join(dataDir, "2026-07.csv"))
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected header and one row, got %d records", len(records))
	}
	if got := records[1][1]; got != "Dl *Uber Rides" {
		t.Fatalf("expected location, got %q", got)
	}
	if got := records[1][5]; got != "uber" {
		t.Fatalf("expected category uber, got %q", got)
	}
}

func TestReadMonthlySpendsCountsNegativeCardAmountsAsExpenses(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	transactions := []CardTransaction{
		{
			Timestamp:       time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
			Location:        "Dl *Uber Rides",
			Currency:        "BRL",
			Amount:          -11.35,
			NativeAmountUSD: -2.21,
			Category:        "uber",
		},
		{
			Timestamp:       time.Date(2026, 7, 8, 13, 0, 0, 0, time.UTC),
			Location:        "Refund",
			Currency:        "BRL",
			Amount:          5,
			NativeAmountUSD: 0.98,
			Category:        "ressarcimento-uber",
		},
	}

	if err := appendCardTransactionsCSV(transactions); err != nil {
		t.Fatalf("append card transactions csv: %v", err)
	}

	spends, err := readMonthlySpends()
	if err != nil {
		t.Fatalf("read monthly spends: %v", err)
	}
	if len(spends) != 1 {
		t.Fatalf("expected one monthly spend, got %d", len(spends))
	}
	if spends[0].Count != 1 {
		t.Fatalf("expected one purchase count, got %d", spends[0].Count)
	}
	if spends[0].Total != 6.35 {
		t.Fatalf("expected total 6.35, got %.2f", spends[0].Total)
	}
}

func TestCardTransactionsUseTransactionTimestampMonth(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	transaction := CardTransaction{
		Timestamp:       time.Date(2026, 6, 30, 23, 30, 0, 0, time.UTC),
		Location:        "Dl *Uber Rides",
		Currency:        "BRL",
		Amount:          -10,
		NativeAmountUSD: -2,
		Category:        "uber",
	}

	if err := appendCardTransactionsCSV([]CardTransaction{transaction}); err != nil {
		t.Fatalf("append card transactions csv: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dataDir, "2026-06.csv")); err != nil {
		t.Fatalf("expected timestamp month file: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dataDir, "2026-07.csv")); !os.IsNotExist(err) {
		t.Fatalf("did not expect export file name month, got err %v", err)
	}

	spends, err := readMonthlySpends()
	if err != nil {
		t.Fatalf("read monthly spends: %v", err)
	}
	if len(spends) != 1 || spends[0].Month != "2026-06" {
		t.Fatalf("expected spend grouped by 2026-06, got %+v", spends)
	}
}

func TestAddManualTransactionWritesExpense(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	timestamp := time.Date(2026, 7, 8, 12, 0, 0, 0, appLocation())
	if err := addManualTransaction(timestamp, "Padaria", 12.5, "comida", false); err != nil {
		t.Fatalf("add manual transaction: %v", err)
	}

	transactions, expenseTotal, refundTotal, err := readMonthTransactions("2026-07")
	if err != nil {
		t.Fatalf("read month transactions: %v", err)
	}
	if len(transactions) != 1 {
		t.Fatalf("expected one transaction, got %d", len(transactions))
	}
	if transactions[0].Location != "Padaria" {
		t.Fatalf("expected location Padaria, got %q", transactions[0].Location)
	}
	if transactions[0].Amount != -12.5 {
		t.Fatalf("expected stored amount -12.5, got %.2f", transactions[0].Amount)
	}
	if expenseTotal != 12.5 {
		t.Fatalf("expected expense total 12.5, got %.2f", expenseTotal)
	}
	if refundTotal != 0 {
		t.Fatalf("expected refund total 0, got %.2f", refundTotal)
	}

	file, err := os.Open(filepath.Join(dataDir, "2026-07.csv"))
	if err != nil {
		t.Fatalf("open csv: %v", err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatalf("read csv: %v", err)
	}
	if got := records[1][0]; got != "2026-07-08T15:00:00Z" {
		t.Fatalf("expected UTC timestamp with Z, got %q", got)
	}
}

func TestDeleteMonthTransactionRemovesCSVRow(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	transactions := []CardTransaction{
		{
			Timestamp:       time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
			Location:        "Dl *Uber Rides",
			Currency:        "BRL",
			Amount:          -11.35,
			NativeAmountUSD: -2.21,
			Category:        "uber",
		},
		{
			Timestamp:       time.Date(2026, 7, 8, 13, 0, 0, 0, time.UTC),
			Location:        "Padaria",
			Currency:        "BRL",
			Amount:          -7,
			NativeAmountUSD: 0,
			Category:        "comida",
		},
	}

	if err := appendCardTransactionsCSV(transactions); err != nil {
		t.Fatalf("append card transactions csv: %v", err)
	}
	if err := deleteMonthTransaction("2026-07", 0); err != nil {
		t.Fatalf("delete month transaction: %v", err)
	}

	monthTransactions, expenseTotal, refundTotal, err := readMonthTransactions("2026-07")
	if err != nil {
		t.Fatalf("read month transactions: %v", err)
	}
	if len(monthTransactions) != 1 {
		t.Fatalf("expected one transaction, got %d", len(monthTransactions))
	}
	if monthTransactions[0].Location != "Padaria" {
		t.Fatalf("expected remaining transaction Padaria, got %q", monthTransactions[0].Location)
	}
	if expenseTotal != 7 {
		t.Fatalf("expected expense total 7, got %.2f", expenseTotal)
	}
	if refundTotal != 0 {
		t.Fatalf("expected refund total 0, got %.2f", refundTotal)
	}
}

func TestUpdateMonthTransactionCategory(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	transaction := CardTransaction{
		Timestamp:       time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
		Location:        "Mp*58861866 Helen",
		Currency:        "BRL",
		Amount:          -3.5,
		NativeAmountUSD: -0.68,
		Category:        "uber",
	}

	if err := appendCardTransactionCSV(transaction); err != nil {
		t.Fatalf("append card transaction csv: %v", err)
	}
	if err := updateMonthTransactionCategory("2026-07", 0, "comida"); err != nil {
		t.Fatalf("update month transaction category: %v", err)
	}

	transactions, _, _, err := readMonthTransactions("2026-07")
	if err != nil {
		t.Fatalf("read month transactions: %v", err)
	}
	if len(transactions) != 1 {
		t.Fatalf("expected one transaction, got %d", len(transactions))
	}
	if transactions[0].Category != "comida" {
		t.Fatalf("expected category comida, got %q", transactions[0].Category)
	}
	if transactions[0].Location != "Mp*58861866 Helen" {
		t.Fatalf("expected location to be preserved, got %q", transactions[0].Location)
	}
}

func TestStoredTransactionMarksSelectedCategory(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	timestamp := time.Date(2026, 7, 8, 12, 0, 0, 0, appLocation())
	if err := addManualTransaction(timestamp, "Padaria", 12.5, "comida", false); err != nil {
		t.Fatalf("add manual transaction: %v", err)
	}

	transactions, _, _, err := readMonthTransactions("2026-07")
	if err != nil {
		t.Fatalf("read month transactions: %v", err)
	}

	foundSelected := false
	for _, option := range transactions[0].CategoryOptions {
		if option.Value == "comida" && option.Selected {
			foundSelected = true
		}
	}
	if !foundSelected {
		t.Fatal("expected comida to be selected")
	}
}

func TestCategoryOptionsIncludeCar(t *testing.T) {
	options := categoryOptions("carro")

	foundSelected := false
	for _, option := range options {
		if option.Value == "carro" && option.Selected {
			foundSelected = true
		}
	}
	if !foundSelected {
		t.Fatal("expected carro to be available and selected")
	}
}

func TestUpdateMonthTransactionCategoryToRefundCategoryMakesAmountPositive(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	transaction := CardTransaction{
		Timestamp:       time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC),
		Location:        "Refund",
		Currency:        "BRL",
		Amount:          -20,
		NativeAmountUSD: 0,
		Category:        "comida",
	}

	if err := appendCardTransactionCSV(transaction); err != nil {
		t.Fatalf("append card transaction csv: %v", err)
	}
	if err := updateMonthTransactionCategory("2026-07", 0, "ressarcimento-comida"); err != nil {
		t.Fatalf("update month transaction category: %v", err)
	}

	transactions, expenseTotal, refundTotal, err := readMonthTransactions("2026-07")
	if err != nil {
		t.Fatalf("read month transactions: %v", err)
	}
	if transactions[0].Amount != 20 {
		t.Fatalf("expected positive refund amount, got %.2f", transactions[0].Amount)
	}
	if expenseTotal != 0 {
		t.Fatalf("expected refund out of expense total, got %.2f", expenseTotal)
	}
	if refundTotal != 20 {
		t.Fatalf("expected refund total 20, got %.2f", refundTotal)
	}
}

func TestReadMonthTransactionsShowsRefundWithoutAddingToTotal(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("DATA_DIR", dataDir)

	timestamp := time.Date(2026, 7, 8, 12, 0, 0, 0, appLocation())
	if err := addManualTransaction(timestamp, "Ressarcimento", 20, "contas", true); err != nil {
		t.Fatalf("add manual transaction: %v", err)
	}

	transactions, expenseTotal, refundTotal, err := readMonthTransactions("2026-07")
	if err != nil {
		t.Fatalf("read month transactions: %v", err)
	}
	if len(transactions) != 1 {
		t.Fatalf("expected one transaction, got %d", len(transactions))
	}
	if transactions[0].Amount != 20 {
		t.Fatalf("expected refund amount 20, got %.2f", transactions[0].Amount)
	}
	if transactions[0].Category != "ressarcimento-contas" {
		t.Fatalf("expected refund category ressarcimento-contas, got %q", transactions[0].Category)
	}
	if transactions[0].AmountLabel != "+R$ 20.00" {
		t.Fatalf("expected signed amount label, got %q", transactions[0].AmountLabel)
	}
	if expenseTotal != 0 {
		t.Fatalf("expected expense total 0, got %.2f", expenseTotal)
	}
	if refundTotal != 20 {
		t.Fatalf("expected refund total 20, got %.2f", refundTotal)
	}
}

func TestCategorySpendsGroupsExpensesByCategory(t *testing.T) {
	transactions := []StoredTransaction{
		{Amount: -10, Category: "comida"},
		{Amount: -5, Category: "comida"},
		{Amount: -20, Category: "carro"},
		{Amount: 8, Category: "ressarcimento-comida"},
	}

	spends := categorySpends(transactions)
	if len(spends) != 2 {
		t.Fatalf("expected two category spends, got %d", len(spends))
	}
	if spends[0].Category != "carro" || spends[0].Total != 20 {
		t.Fatalf("expected carro first with 20, got %+v", spends[0])
	}
	if spends[1].Category != "comida" || spends[1].Total != 7 {
		t.Fatalf("expected comida second with 7, got %+v", spends[1])
	}
}

func TestFilterTransactionsByCategory(t *testing.T) {
	transactions := []StoredTransaction{
		{Location: "Padaria", Category: "comida"},
		{Location: "Posto", Category: "carro"},
		{Location: "Internet", Category: "contas"},
	}

	filtered := filterTransactionsByCategory(transactions, "carro")
	if len(filtered) != 1 {
		t.Fatalf("expected one filtered transaction, got %d", len(filtered))
	}
	if filtered[0].Location != "Posto" {
		t.Fatalf("expected Posto, got %q", filtered[0].Location)
	}

	all := filterTransactionsByCategory(transactions, "")
	if len(all) != len(transactions) {
		t.Fatalf("expected all transactions, got %d", len(all))
	}
}
