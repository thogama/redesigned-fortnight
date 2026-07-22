package main

import (
	"strings"
	"testing"
	"time"
)

func TestParseCryptoComCardCSVCategorizesByLocation(t *testing.T) {
	transactions, err := parseCryptoComCardCSV(strings.NewReader(`Timestamp (UTC),Transaction Description,Currency,Amount,To Currency,To Amount,Native Currency,Native Amount,Native Amount (in USD),Transaction Kind,Transaction Hash
2026-07-07 20:46:59,Dl *Uber Rides,BRL,-11.35,USD,-2.21,USD,-2.21,-2.21,,
2026-07-07 11:52:41,Ifd*I Food,BRL,-7.95,USD,-1.55,USD,-1.55,-1.55,,
2026-07-05 17:23:29,Supermercado Aruana,BRL,-26.7,USD,-5.17,USD,-5.17,-5.17,,
2026-07-06 22:52:39,Petrox,BRL,-30.35,USD,-5.92,USD,-5.92,-5.92,,
2026-07-07 12:00:00,Refund,BRL,10.00,USD,1.95,USD,1.95,1.95,,
`))
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}

	expected := []string{"uber", "comida", "comida", "carro", ""}
	for index, category := range expected {
		if transactions[index].Category != category {
			t.Fatalf("transaction %d: expected category %q, got %q", index, category, transactions[index].Category)
		}
	}
}

func TestClassifyExpenseUsesEstablishmentValueAndLocalTime(t *testing.T) {
	tests := []struct {
		name          string
		establishment string
		value         float64
		timestamp     string
		want          string
	}{
		{name: "accent insensitive merchant", establishment: "Açougue Central", value: 80, timestamp: "2026-07-05T12:00:00Z", want: "comida"},
		{name: "99 ride within expected value", establishment: "99", value: 16.80, timestamp: "2026-07-06T09:00:00Z", want: "uber"},
		{name: "small weekday daytime expense", establishment: "Pessoa desconhecida", value: 12, timestamp: "2026-07-07T12:00:00Z", want: "uber"},
		{name: "unknown weekend expense", establishment: "Evento local", value: 50, timestamp: "2026-07-05T12:00:00Z", want: "lazer"},
		{name: "strong merchant wins on weekend", establishment: "Supermercado Aruana", value: 50, timestamp: "2026-07-05T12:00:00Z", want: "comida"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			timestamp, err := time.Parse(time.RFC3339, test.timestamp)
			if err != nil {
				t.Fatal(err)
			}
			if got := classifyExpense(test.establishment, test.value, timestamp); got != test.want {
				t.Fatalf("expected %q, got %q", test.want, got)
			}
		})
	}
}

func TestParseCryptoComCardCSVAppliesFridayFoodRules(t *testing.T) {
	transactions, err := parseCryptoComCardCSV(strings.NewReader(`Timestamp (UTC),Transaction Description,Currency,Amount,To Currency,To Amount,Native Currency,Native Amount,Native Amount (in USD),Transaction Kind,Transaction Hash
2026-07-03 20:38:26,Mp*58861866 Helen,BRL,-3.50,USD,-0.68,USD,-0.68,-0.68,,
2026-07-03 20:35:12,Mp*58861866 Helen,BRL,-3.75,USD,-0.73,USD,-0.73,-0.73,,
2026-07-03 20:34:00,Rede Presidente Filial,BRL,-60.00,USD,-11.62,USD,-11.62,-11.62,,
`))
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}

	for index, transaction := range transactions {
		if transaction.Category != "comida" {
			t.Fatalf("transaction %d: expected comida, got %q", index, transaction.Category)
		}
	}
}
