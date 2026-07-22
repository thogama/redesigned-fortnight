package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultOpenAIModel = "gpt-5.6-luna"

type expenseToClassify struct {
	Establishment string  `json:"estabelecimento"`
	Value         float64 `json:"valor_brl"`
	LocalDateTime string  `json:"data_hora_local"`
}

func classifyExpensesWithOpenAI(expenses []expenseToClassify) ([]string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	if apiKey == "" || len(expenses) == 0 {
		return nil, nil
	}

	input, err := json.Marshal(expenses)
	if err != nil {
		return nil, err
	}
	model := strings.TrimSpace(os.Getenv("OPENAI_MODEL"))
	if model == "" {
		model = defaultOpenAIModel
	}
	prompt := `Classifique cada gasto brasileiro, na mesma ordem, usando somente uma destas categorias: uber, carro, comida, contas, lazer. Considere estabelecimento, valor em BRL e data/hora local. Responda exclusivamente com um array JSON de strings, sem markdown. Gastos: ` + string(input)
	body, err := json.Marshal(map[string]any{
		"model":     model,
		"input":     prompt,
		"reasoning": map[string]string{"effort": "none"},
		"store":     false,
	})
	if err != nil {
		return nil, err
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	req, err := http.NewRequest(http.MethodPost, baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("openai returned status %d", response.StatusCode)
	}

	var apiResponse struct {
		Output []struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(responseBody, &apiResponse); err != nil {
		return nil, err
	}
	for _, output := range apiResponse.Output {
		for _, content := range output.Content {
			if content.Type != "output_text" {
				continue
			}
			var categories []string
			if err := json.Unmarshal([]byte(strings.TrimSpace(content.Text)), &categories); err != nil {
				return nil, err
			}
			if len(categories) != len(expenses) {
				return nil, fmt.Errorf("openai returned %d categories for %d expenses", len(categories), len(expenses))
			}
			for _, category := range categories {
				if !isExpenseCategory(category) {
					return nil, fmt.Errorf("openai returned invalid category %q", category)
				}
			}
			return categories, nil
		}
	}
	return nil, fmt.Errorf("openai response did not contain output text")
}

func isExpenseCategory(category string) bool {
	for _, allowed := range expenseCategories() {
		if category == allowed {
			return true
		}
	}
	return false
}
