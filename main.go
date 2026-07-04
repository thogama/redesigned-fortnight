package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"html"
	"io"
	"log"
	"mime/quotedprintable"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

type CryptoPurchaseEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Value     string    `json:"value"`
	Currency  string    `json:"currency"`
}

var purchasePattern = regexp.MustCompile(`(?i)compra\s+de\s+([0-9]+(?:[.,][0-9]+)?)\s*([A-Z]{3})`)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file loaded")
	}

	router := gin.Default()
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.POST("/webhook/cloudmailin", cloudMailinWebhook)

	router.Run() // listens on 0.0.0.0:8080 by default
}

func cloudMailinWebhook(c *gin.Context) {
	if !validBasicAuth(c) {
		c.Header("WWW-Authenticate", `Basic realm="cloudmailin webhook"`)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid basic auth credentials"})
		return
	}

	var raw map[string]json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	body, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON payload"})
		return
	}

	log.Printf("cloudmailin webhook payload:\n%s", body)

	event, ok := extractCryptoPurchaseEvent(raw)
	if ok {
		eventBody, err := json.MarshalIndent(event, "", "  ")
		if err == nil {
			log.Printf("crypto purchase event:\n%s", eventBody)
		}
		if err := appendDailyEventCSV(event); err != nil {
			log.Printf("failed to write daily event csv: %v", err)
		}
	} else {
		log.Println("crypto purchase event: not found in payload")
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status": "accepted",
	})
}

func validBasicAuth(c *gin.Context) bool {
	expectedUser := os.Getenv("WEBHOOK_BASIC_USER")
	expectedPass := os.Getenv("WEBHOOK_BASIC_PASS")
	if expectedUser == "" || expectedPass == "" {
		return true
	}

	user, pass, ok := c.Request.BasicAuth()
	return ok && user == expectedUser && pass == expectedPass
}

func appendDailyEventCSV(event CryptoPurchaseEvent) error {
	if err := os.MkdirAll("data", 0755); err != nil {
		return err
	}

	fileName := "data/" + event.Timestamp.Local().Format("2006-01-02") + ".csv"
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
		if err := writer.Write([]string{"value", "timestamp"}); err != nil {
			return err
		}
	}
	if err := writer.Write([]string{event.Value, event.Timestamp.Format(time.RFC3339)}); err != nil {
		return err
	}
	writer.Flush()
	return writer.Error()
}

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
