package main

import (
	"strings"
	"testing"
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

	expected := []string{"uber", "comida", "lazer", "carro", ""}
	for index, category := range expected {
		if transactions[index].Category != category {
			t.Fatalf("transaction %d: expected category %q, got %q", index, category, transactions[index].Category)
		}
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
