package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ExchangeRateResponse struct {
	Result string             `json:"result"`
	Rates  map[string]float64 `json:"rates"`
}

func fetchUSDBRLRate() (float64, error) {
	client := http.Client{Timeout: 5 * time.Second}
	request, err := http.NewRequest(http.MethodGet, "https://open.er-api.com/v6/latest/USD", nil)
	if err != nil {
		return 0, err
	}

	response, err := client.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("exchange rate API returned status %d", response.StatusCode)
	}

	var result ExchangeRateResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return 0, err
	}

	rate, ok := result.Rates["BRL"]
	if !ok || rate <= 0 {
		return 0, fmt.Errorf("USD/BRL rate not found")
	}

	return rate, nil
}
