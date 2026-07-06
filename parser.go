package main

import (
	"bytes"
	"encoding/json"
	"html"
	"io"
	"mime/quotedprintable"
	"regexp"
	"strings"
	"time"
)

type CryptoPurchaseEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Value     string    `json:"value"`
	Currency  string    `json:"currency"`
}

var purchasePattern = regexp.MustCompile(`(?i)compra\s+de\s+([0-9]+(?:[.,][0-9]+)?)\s*([A-Z]{3})`)

func extractCryptoPurchaseEvent(payload map[string]json.RawMessage) (CryptoPurchaseEvent, bool) {
	text := strings.Join([]string{
		payloadString(payload, "plain"),
		payloadString(payload, "html"),
	}, "\n")
	text = normalizeEmailText(text)

	matches := purchasePattern.FindStringSubmatch(text)
	if len(matches) != 3 {
		return CryptoPurchaseEvent{}, false
	}

	return CryptoPurchaseEvent{
		Timestamp: time.Now().UTC(),
		Value:     strings.ReplaceAll(matches[1], ",", "."),
		Currency:  strings.ToUpper(matches[2]),
	}, true
}

func payloadString(payload map[string]json.RawMessage, key string) string {
	value, ok := payload[key]
	if !ok {
		return ""
	}

	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return text
	}

	return ""
}

func normalizeEmailText(text string) string {
	text = decodeQuotedPrintable(text)
	text = html.UnescapeString(text)
	text = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(text, " ")
	text = strings.ReplaceAll(text, "\u200c", "")
	text = strings.Join(strings.Fields(text), " ")
	return text
}

func decodeQuotedPrintable(text string) string {
	reader := quotedprintable.NewReader(strings.NewReader(text))
	decoded, err := io.ReadAll(reader)
	if err != nil || !bytes.Contains(decoded, []byte(" ")) {
		return text
	}

	return string(decoded)
}
