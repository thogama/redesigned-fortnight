package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClassifyExpensesWithOpenAIUsesEnvironmentKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected bearer key, got %q", got)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["model"] != "test-model" {
			t.Fatalf("expected configured model, got %v", body["model"])
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"output":[{"content":[{"type":"output_text","text":"[\"comida\"]"}]}]}`))
	}))
	defer server.Close()

	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("OPENAI_MODEL", "test-model")
	t.Setenv("OPENAI_BASE_URL", server.URL)
	categories, err := classifyExpensesWithOpenAI([]expenseToClassify{{Establishment: "Padaria", Value: 12}})
	if err != nil {
		t.Fatal(err)
	}
	if len(categories) != 1 || categories[0] != "comida" {
		t.Fatalf("unexpected categories: %v", categories)
	}
}

func TestClassifyExpensesWithOpenAIIsDisabledWithoutKey(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "")
	categories, err := classifyExpensesWithOpenAI([]expenseToClassify{{Establishment: "Padaria", Value: 12}})
	if err != nil || categories != nil {
		t.Fatalf("expected disabled classifier, got categories %v and error %v", categories, err)
	}
}
