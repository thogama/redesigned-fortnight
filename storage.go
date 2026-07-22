package main

import (
	"encoding/csv"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func appendCardTransactionsCSV(transactions []CardTransaction) error {
	existingKeys, err := existingTransactionKeys()
	if err != nil {
		return err
	}

	for _, transaction := range transactions {
		key := cardTransactionKey(transaction)
		if existingKeys[key] {
			continue
		}
		if err := appendCardTransactionCSV(transaction); err != nil {
			return err
		}
		existingKeys[key] = true
	}
	return nil
}

func appendCardTransactionCSV(transaction CardTransaction) error {
	dir := dataDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	month := transaction.Timestamp.In(appLocation()).Format("2006-01")
	fileName := filepath.Join(dir, month+".csv")
	needsHeader := true
	if info, err := os.Stat(fileName); err == nil && info.Size() > 0 {
		needsHeader = false
	}

	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if needsHeader {
		if err := writer.Write([]string{"timestamp", "local", "amount", "currency", "usd_amount", "category"}); err != nil {
			return err
		}
	}
	if err := writer.Write(cardTransactionRecord(transaction)); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func cardTransactionRecord(transaction CardTransaction) []string {
	return []string{
		transaction.Timestamp.UTC().Format(time.RFC3339),
		transaction.Location,
		strconv.FormatFloat(transaction.Amount, 'f', 2, 64),
		transaction.Currency,
		strconv.FormatFloat(transaction.NativeAmountUSD, 'f', 2, 64),
		transaction.Category,
	}
}

func existingTransactionKeys() (map[string]bool, error) {
	keys := map[string]bool{}
	files, err := monthlyDataFiles()
	if err != nil {
		return nil, err
	}

	for _, fileName := range files {
		records, err := readMonthlyRecords(fileName)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if len(records) == 0 {
			continue
		}

		header := headerIndexes(records[0])
		for _, record := range records[1:] {
			key := storedRecordKey(header, record)
			if key != "" {
				keys[key] = true
			}
		}
	}

	return keys, nil
}

func cardTransactionKey(transaction CardTransaction) string {
	return transactionKey(
		transaction.Timestamp.UTC().Format(time.RFC3339),
		transaction.Location,
		strconv.FormatFloat(transaction.Amount, 'f', 2, 64),
		transaction.Currency,
		strconv.FormatFloat(transaction.NativeAmountUSD, 'f', 2, 64),
	)
}

func storedRecordKey(header map[string]int, record []string) string {
	return transactionKey(
		field(header, record, "timestamp"),
		field(header, record, "local"),
		field(header, record, "amount"),
		field(header, record, "currency"),
		field(header, record, "usd_amount"),
	)
}

func transactionKey(timestamp, location, amount, currency, usdAmount string) string {
	timestamp = strings.TrimSpace(timestamp)
	location = strings.TrimSpace(location)
	amount = strings.TrimSpace(amount)
	currency = strings.TrimSpace(currency)
	usdAmount = strings.TrimSpace(usdAmount)
	if timestamp == "" || location == "" || amount == "" {
		return ""
	}

	return strings.Join([]string{timestamp, location, amount, currency, usdAmount}, "\x1f")
}

func readMonthTransactions(month string) ([]StoredTransaction, float64, float64, error) {
	fileName := monthlyFileName(month)
	file, err := os.Open(fileName)
	if os.IsNotExist(err) {
		return nil, 0, 0, nil
	}
	if err != nil {
		return nil, 0, 0, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, 0, 0, err
	}
	if len(records) == 0 {
		return nil, 0, 0, nil
	}

	header := headerIndexes(records[0])
	transactions := make([]StoredTransaction, 0, len(records)-1)
	expenseTotal := 0.0
	refundTotal := 0.0
	for index, record := range records[1:] {
		transaction, ok := storedTransaction(header, record, index)
		if !ok {
			continue
		}
		if transaction.Amount < 0 {
			expenseTotal += math.Abs(transaction.Amount)
		} else if transaction.Amount > 0 {
			refundTotal += transaction.Amount
		}
		transactions = append(transactions, transaction)
	}

	sort.Slice(transactions, func(i, j int) bool {
		return transactions[i].Timestamp.After(transactions[j].Timestamp)
	})

	return transactions, expenseTotal, refundTotal, nil
}

func categorySpends(transactions []StoredTransaction) []CategorySpend {
	totals := make(map[string]float64)
	maxTotal := 0.0
	for _, transaction := range transactions {
		category := baseCategory(transaction.Category)
		if category == "" {
			category = "sem categoria"
		}
		totals[category] += -transaction.Amount
		if totals[category] > maxTotal {
			maxTotal = totals[category]
		}
	}
	if len(totals) == 0 {
		return nil
	}

	spends := make([]CategorySpend, 0, len(totals))
	for category, total := range totals {
		if total <= 0 {
			continue
		}
		percent := 0.0
		if maxTotal > 0 {
			percent = total / maxTotal * 100
		}
		spends = append(spends, CategorySpend{
			Category: category,
			Total:    total,
			Label:    formatBRL(total),
			Percent:  strconv.FormatFloat(percent, 'f', 2, 64) + "%",
		})
	}

	sort.Slice(spends, func(i, j int) bool {
		return spends[i].Total > spends[j].Total
	})

	return spends
}

func filterTransactionsByCategory(transactions []StoredTransaction, category string) []StoredTransaction {
	category = strings.TrimSpace(category)
	if category == "" {
		return transactions
	}

	filtered := make([]StoredTransaction, 0, len(transactions))
	for _, transaction := range transactions {
		if transaction.Category == category {
			filtered = append(filtered, transaction)
		}
	}
	return filtered
}

func addManualTransaction(timestamp time.Time, location string, amount float64, category string, refund bool) error {
	if amount <= 0 {
		return nil
	}

	category = strings.TrimSpace(category)
	if category == "" || category == "automatico" {
		categories, err := classifyExpensesWithOpenAI([]expenseToClassify{{
			Establishment: location,
			Value:         amount,
			LocalDateTime: timestamp.In(appLocation()).Format(time.RFC3339),
		}})
		if err == nil && len(categories) == 1 {
			category = categories[0]
		} else {
			category = classifyExpense(location, amount, timestamp)
		}
	}
	storedAmount := -amount
	if refund {
		storedAmount = amount
		category = refundCategory(category)
	}

	return appendCardTransactionCSV(CardTransaction{
		Timestamp:       timestamp,
		Location:        strings.TrimSpace(location),
		Currency:        "BRL",
		Amount:          storedAmount,
		NativeAmountUSD: 0,
		Category:        category,
	})
}

func deleteMonthTransaction(month string, transactionIndex int) error {
	fileName := monthlyFileName(month)
	records, err := readMonthlyRecords(fileName)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	targetRecordIndex := transactionIndex + 1
	if targetRecordIndex <= 0 || targetRecordIndex >= len(records) {
		return nil
	}

	records = append(records[:targetRecordIndex], records[targetRecordIndex+1:]...)
	return writeMonthlyRecords(fileName, records)
}

func updateMonthTransactionCategory(month string, transactionIndex int, category string) error {
	fileName := monthlyFileName(month)
	records, err := readMonthlyRecords(fileName)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	targetRecordIndex := transactionIndex + 1
	if targetRecordIndex <= 0 || targetRecordIndex >= len(records) {
		return nil
	}

	header := headerIndexes(records[0])
	categoryIndex, ok := header["category"]
	if !ok {
		return nil
	}
	for len(records[targetRecordIndex]) <= categoryIndex {
		records[targetRecordIndex] = append(records[targetRecordIndex], "")
	}
	category = strings.TrimSpace(category)
	records[targetRecordIndex][categoryIndex] = category

	amountIndex, ok := header["amount"]
	if ok && amountIndex < len(records[targetRecordIndex]) {
		amount, err := parseCSVFloat(records[targetRecordIndex][amountIndex])
		if err == nil {
			amount = math.Abs(amount)
			if !isRefundCategory(category) {
				amount = -amount
			}
			records[targetRecordIndex][amountIndex] = strconv.FormatFloat(amount, 'f', 2, 64)
		}
	}

	return writeMonthlyRecords(fileName, records)
}

func readMonthlyRecords(fileName string) ([][]string, error) {
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	closeErr := file.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return records, nil
}

func writeMonthlyRecords(fileName string, records [][]string) error {
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if err := writer.WriteAll(records); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

func readMonthlySpends() ([]MonthlySpend, error) {
	files, err := monthlyDataFiles()
	if err != nil {
		return nil, err
	}

	monthly := make(map[string]*MonthlySpend)
	for _, fileName := range files {
		if err := readMonthlySpendsFile(fileName, monthly); err != nil {
			log.Printf("failed to read spend file %s: %v", fileName, err)
		}
	}

	spends := make([]MonthlySpend, 0, len(monthly))
	for _, spend := range monthly {
		spends = append(spends, *spend)
	}

	sort.Slice(spends, func(i, j int) bool {
		return spends[i].Month > spends[j].Month
	})

	return spends, nil
}

// monthlyDataFiles returns only canonical monthly files. Files such as
// 2026-07-back.csv are backups and must not affect imports or dashboard totals.
func monthlyDataFiles() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(dataDir(), "????-??.csv"))
	if err != nil {
		return nil, err
	}

	monthlyFiles := make([]string, 0, len(files))
	for _, fileName := range files {
		month := strings.TrimSuffix(filepath.Base(fileName), ".csv")
		if _, err := time.Parse("2006-01", month); err == nil {
			monthlyFiles = append(monthlyFiles, fileName)
		}
	}
	return monthlyFiles, nil
}

func readMonthlySpendsFile(fileName string, monthly map[string]*MonthlySpend) error {
	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}

	header := headerIndexes(records[0])
	for _, record := range records[1:] {
		if len(record) == 0 {
			continue
		}

		timestamp, amount, err := storedTransactionValues(header, record)
		if err != nil {
			continue
		}

		month := timestamp.In(appLocation()).Format("2006-01")
		if _, ok := monthly[month]; !ok {
			monthTimestamp, err := time.Parse("2006-01", month)
			if err != nil {
				monthTimestamp = timestamp.In(appLocation())
			}
			monthly[month] = &MonthlySpend{
				Month:      month,
				MonthLabel: monthLabel(monthTimestamp),
			}
		}
		if amount < 0 {
			monthly[month].Total += math.Abs(amount)
			monthly[month].Count++
		} else {
			monthly[month].Total -= amount
		}
	}

	return nil
}

func storedTransactionValues(header map[string]int, record []string) (time.Time, float64, error) {
	timestamp, err := time.Parse(time.RFC3339, field(header, record, "timestamp"))
	if err != nil {
		return time.Time{}, 0, err
	}

	amount, err := parseCSVFloat(field(header, record, "amount"))
	if err != nil {
		return time.Time{}, 0, err
	}

	return timestamp, amount, nil
}

func storedTransaction(header map[string]int, record []string, index int) (StoredTransaction, bool) {
	timestamp, amount, err := storedTransactionValues(header, record)
	if err != nil {
		return StoredTransaction{}, false
	}

	return StoredTransaction{
		Index:           index,
		Timestamp:       timestamp,
		TimestampLabel:  timestamp.In(appLocation()).Format("02/01/2006 15:04"),
		Location:        field(header, record, "local"),
		Amount:          amount,
		AmountLabel:     formatSignedBRL(amount),
		Currency:        field(header, record, "currency"),
		USDAmount:       mustParseCSVFloat(field(header, record, "usd_amount")),
		Category:        field(header, record, "category"),
		CategoryOptions: categoryOptions(field(header, record, "category")),
	}, true
}

func monthlyFileName(month string) string {
	return filepath.Join(dataDir(), month+".csv")
}

func mustParseCSVFloat(value string) float64 {
	parsed, err := parseCSVFloat(value)
	if err != nil {
		return 0
	}
	return parsed
}

func categoryOptions(selected string) []CategoryOption {
	categories := transactionCategories()
	options := make([]CategoryOption, 0, len(categories))
	for _, category := range categories {
		options = append(options, CategoryOption{
			Value:    category,
			Selected: category == selected,
		})
	}
	return options
}

func filterOptions(selected string) []FilterOption {
	options := []FilterOption{{Label: "Todas", Value: "", Selected: selected == ""}}
	for _, category := range transactionCategories() {
		options = append(options, FilterOption{
			Label:    category,
			Value:    category,
			Selected: selected == category,
		})
	}
	return options
}

func expenseCategories() []string {
	return []string{"comida", "contas", "lazer", "uber", "carro"}
}

func transactionCategories() []string {
	categories := append([]string{}, expenseCategories()...)
	for _, category := range expenseCategories() {
		categories = append(categories, refundCategory(category))
	}
	return categories
}

func refundCategory(category string) string {
	category = strings.TrimSpace(category)
	if category == "" || isRefundCategory(category) {
		return category
	}
	return "ressarcimento-" + category
}

func isRefundCategory(category string) bool {
	return strings.HasPrefix(strings.TrimSpace(category), "ressarcimento-")
}

func baseCategory(category string) string {
	category = strings.TrimSpace(category)
	return strings.TrimPrefix(category, "ressarcimento-")
}

func dataDir() string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		return "data"
	}

	return strings.TrimSpace(dir)
}
