package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

type CardTransaction struct {
	Timestamp       time.Time
	Location        string
	Currency        string
	Amount          float64
	ToCurrency      string
	ToAmount        float64
	NativeCurrency  string
	NativeAmount    float64
	NativeAmountUSD float64
	Category        string
}

var locationCategoryRules = []struct {
	category string
	keywords []string
}{
	{category: "uber", keywords: []string{"uber", "99app", "99 pop", "taxi", "táxi"}},
	{category: "carro", keywords: []string{"posto", "petro", "gasolina", "combustivel", "combustível"}},
	{category: "comida", keywords: []string{"ifood", "i food", "restaurante", "supermercado", "mercado", "padaria", "lanchonete", "pizzaria", "pizza", "carnes", "acougue", "açougue"}},
	{category: "contas", keywords: []string{"boleto", "energia", "internet", "telefone", "aluguel", "agua", "água", "vivo", "claro", "tim ", "enel"}},
	{category: "lazer", keywords: []string{"cinema", "ingresso", "shopping", "bar", "show", "spotify", "netflix", "jogo"}},
}

func parseCryptoComCardCSV(reader io.Reader) ([]CardTransaction, error) {
	csvReader := csv.NewReader(reader)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}

	header := headerIndexes(records[0])
	transactions := make([]CardTransaction, 0, len(records)-1)
	for rowIndex, record := range records[1:] {
		transaction, err := parseCryptoComCardRecord(header, record)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", rowIndex+2, err)
		}
		transactions = append(transactions, transaction)
	}

	classifyCardTransactions(transactions)
	return transactions, nil
}

func parseCryptoComCardRecord(header map[string]int, record []string) (CardTransaction, error) {
	timestamp, err := time.ParseInLocation("2006-01-02 15:04:05", field(header, record, "Timestamp (UTC)"), time.UTC)
	if err != nil {
		return CardTransaction{}, err
	}

	amount, err := parseCSVFloat(field(header, record, "Amount"))
	if err != nil {
		return CardTransaction{}, err
	}
	toAmount, err := parseCSVFloat(field(header, record, "To Amount"))
	if err != nil {
		return CardTransaction{}, err
	}
	nativeAmount, err := parseCSVFloat(field(header, record, "Native Amount"))
	if err != nil {
		return CardTransaction{}, err
	}
	nativeAmountUSD, err := parseCSVFloat(field(header, record, "Native Amount (in USD)"))
	if err != nil {
		return CardTransaction{}, err
	}

	return CardTransaction{
		Timestamp:       timestamp,
		Location:        field(header, record, "Transaction Description"),
		Currency:        field(header, record, "Currency"),
		Amount:          amount,
		ToCurrency:      field(header, record, "To Currency"),
		ToAmount:        toAmount,
		NativeCurrency:  field(header, record, "Native Currency"),
		NativeAmount:    nativeAmount,
		NativeAmountUSD: nativeAmountUSD,
	}, nil
}

func classifyCardTransactions(transactions []CardTransaction) {
	unknownIndexes := make([]int, 0)
	unknownExpenses := make([]expenseToClassify, 0)
	for index := range transactions {
		if transactions[index].Amount > 0 {
			// Refunds must not use weak value/time inference: a small credit is
			// not necessarily a transport refund.
			transactions[index].Category = classifyByLocation(transactions[index].Location)
		} else {
			transactions[index].Category = classifyByLocation(transactions[index].Location)
			if transactions[index].Category == "" {
				unknownIndexes = append(unknownIndexes, index)
				unknownExpenses = append(unknownExpenses, expenseToClassify{
					Establishment: transactions[index].Location,
					Value:         math.Abs(transactions[index].Amount),
					LocalDateTime: transactions[index].Timestamp.In(appLocation()).Format(time.RFC3339),
				})
			}
		}
		if transactions[index].Amount > 0 && transactions[index].Category != "" {
			transactions[index].Category = refundCategory(transactions[index].Category)
		}
	}
	if categories, err := classifyExpensesWithOpenAI(unknownExpenses); err == nil && len(categories) == len(unknownIndexes) {
		for index, category := range categories {
			transactions[unknownIndexes[index]].Category = category
		}
	}

	applyTimeBasedCategories(transactions)
}

// classifyExpense combines the establishment/location description, the value and
// the local date/time. Merchant matches are strong evidence; value and local time
// are only used as a fallback so a supermarket on Sunday is still food.
func classifyExpense(establishment string, value float64, timestamp time.Time) string {
	if category := classifyByLocation(establishment); category != "" {
		return category
	}

	normalized := normalizeDescription(establishment)
	// Card statements commonly abbreviate 99 rides to just "99". Restrict the
	// match by value to avoid treating arbitrary merchant numbers as transport.
	if normalized == "99" && value > 0 && value <= 200 {
		return "uber"
	}

	localTimestamp := timestamp.In(appLocation())
	weekday := localTimestamp.Weekday()
	hour := localTimestamp.Hour()
	if weekday >= time.Monday && weekday <= time.Friday && hour >= 6 && hour < 19 && value > 0 && value <= 25 {
		return "uber"
	}
	if weekday == time.Saturday || weekday == time.Sunday {
		return "lazer"
	}
	return ""
}

func classifyByLocation(location string) string {
	normalized := normalizeDescription(location)
	for _, rule := range locationCategoryRules {
		for _, keyword := range rule.keywords {
			if strings.Contains(normalized, keyword) {
				return rule.category
			}
		}
	}

	return ""
}

func normalizeDescription(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(
		"á", "a", "à", "a", "â", "a", "ã", "a",
		"é", "e", "ê", "e", "í", "i", "ó", "o",
		"ô", "o", "õ", "o", "ú", "u", "ç", "c",
	)
	return strings.Join(strings.Fields(replacer.Replace(value)), " ")
}

func applyTimeBasedCategories(transactions []CardTransaction) {
	for index := range transactions {
		transaction := transactions[index]
		if transaction.Amount > 0 {
			continue
		}

		if transactions[index].Category == "" {
			transactions[index].Category = classifyExpense(transaction.Location, math.Abs(transaction.Amount), transaction.Timestamp)
		}
	}

	for index := range transactions {
		if !isFridaySmallConsecutivePurchase(transactions, index) {
			continue
		}
		transactions[index].Category = "comida"
		if index > 0 && isFridayMarketNeighbor(transactions[index-1]) {
			transactions[index-1].Category = "comida"
		}
		if index+1 < len(transactions) && isFridayMarketNeighbor(transactions[index+1]) {
			transactions[index+1].Category = "comida"
		}
	}
}

func isFridaySmallConsecutivePurchase(transactions []CardTransaction, index int) bool {
	transaction := transactions[index]
	if transaction.Timestamp.In(appLocation()).Weekday() != time.Friday || transaction.Amount >= 0 || math.Abs(transaction.NativeAmountUSD) >= 4 {
		return false
	}

	return isCloseFridayPurchase(transaction, transactions, index-1) || isCloseFridayPurchase(transaction, transactions, index+1)
}

func isCloseFridayPurchase(transaction CardTransaction, transactions []CardTransaction, otherIndex int) bool {
	if otherIndex < 0 || otherIndex >= len(transactions) {
		return false
	}

	other := transactions[otherIndex]
	if other.Timestamp.In(appLocation()).Weekday() != time.Friday || other.Amount >= 0 || math.Abs(other.NativeAmountUSD) >= 4 {
		return false
	}

	return absDuration(transaction.Timestamp.Sub(other.Timestamp)) <= 5*time.Minute
}

func isFridayMarketNeighbor(transaction CardTransaction) bool {
	return transaction.Timestamp.In(appLocation()).Weekday() == time.Friday && transaction.Amount < 0 && math.Abs(transaction.NativeAmountUSD) > 8
}

func headerIndexes(header []string) map[string]int {
	indexes := make(map[string]int, len(header))
	for index, name := range header {
		indexes[strings.TrimSpace(name)] = index
	}
	return indexes
}

func field(header map[string]int, record []string, name string) string {
	index, ok := header[name]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

func parseCSVFloat(value string) (float64, error) {
	if value == "" {
		return 0, nil
	}
	return strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
}

func absDuration(value time.Duration) time.Duration {
	if value < 0 {
		return -value
	}
	return value
}

func appLocation() *time.Location {
	return time.FixedZone("America/Fortaleza", -3*60*60)
}
