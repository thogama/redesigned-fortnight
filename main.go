package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	htmltemplate "html/template"
	"io"
	"log"
	"mime/quotedprintable"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
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

type MonthlySpend struct {
	Month      string
	MonthLabel string
	Total      float64
	TotalLabel string
	Count      int
}

var purchasePattern = regexp.MustCompile(`(?i)compra\s+de\s+([0-9]+(?:[.,][0-9]+)?)\s*([A-Z]{3})`)
var dashboardTemplate = htmltemplate.Must(htmltemplate.New("dashboard").Parse(`<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Gastos Crypto.com</title>
  <style>
    :root { color-scheme: light; font-family: Arial, Helvetica, sans-serif; }
    body { margin: 0; background: #f5f7fb; color: #1d2433; }
    main { max-width: 920px; margin: 0 auto; padding: 32px 20px; }
    header { margin-bottom: 24px; }
    h1 { margin: 0 0 8px; font-size: 28px; line-height: 1.2; }
    p { margin: 0; color: #667085; }
    table { width: 100%; border-collapse: collapse; background: #fff; border: 1px solid #d9e0ea; }
    th, td { padding: 14px 16px; border-bottom: 1px solid #e8edf3; text-align: left; }
    th { background: #eef3f9; font-size: 13px; text-transform: uppercase; color: #46566f; }
    td:last-child, th:last-child { text-align: right; }
    tr:last-child td { border-bottom: 0; }
    .empty { padding: 20px; background: #fff; border: 1px solid #d9e0ea; }
  </style>
</head>
<body>
  <main>
    <header>
      <h1>Gastos por mês</h1>
      <p>Somatório dos eventos de compra recebidos pelo webhook.</p>
    </header>
    {{if .}}
    <table>
      <thead>
        <tr>
          <th>Mês</th>
          <th>Compras</th>
          <th>Total</th>
        </tr>
      </thead>
      <tbody>
        {{range .}}
        <tr>
          <td>{{.MonthLabel}}</td>
          <td>{{.Count}}</td>
          <td>{{.TotalLabel}}</td>
        </tr>
        {{end}}
      </tbody>
    </table>
    {{else}}
    <div class="empty">Nenhum gasto registrado ainda.</div>
    {{end}}
  </main>
</body>
</html>`))

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file loaded")
	}

	router := gin.Default()
	router.GET("/", dashboardHandler)
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.POST("/webhook/cloudmailin", cloudMailinWebhook)

	port := os.Getenv("PORT")
	if port == "" {
		port = "7860"
	}

	if err := router.Run("0.0.0.0:" + port); err != nil {
		log.Fatal(err)
	}
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

func dashboardHandler(c *gin.Context) {
	spends, err := readMonthlySpends()
	if err != nil {
		log.Printf("failed to read monthly spends: %v", err)
		c.String(http.StatusInternalServerError, "failed to read monthly spends")
		return
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTemplate.Execute(c.Writer, spends); err != nil {
		log.Printf("failed to render dashboard: %v", err)
	}
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
	dir := dataDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fileName := filepath.Join(dir, event.Timestamp.Local().Format("2006-01-02")+".csv")
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

func readMonthlySpends() ([]MonthlySpend, error) {
	files, err := filepath.Glob(filepath.Join(dataDir(), "*.csv"))
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
		spend.TotalLabel = fmt.Sprintf("%.2f", spend.Total)
		spends = append(spends, *spend)
	}

	sort.Slice(spends, func(i, j int) bool {
		return spends[i].Month > spends[j].Month
	})

	return spends, nil
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

	for index, record := range records {
		if index == 0 && len(record) >= 2 && record[0] == "value" && record[1] == "timestamp" {
			continue
		}
		if len(record) < 2 {
			continue
		}

		value, err := strconv.ParseFloat(strings.TrimSpace(record[0]), 64)
		if err != nil {
			continue
		}

		timestamp, err := time.Parse(time.RFC3339, strings.TrimSpace(record[1]))
		if err != nil {
			continue
		}

		month := timestamp.Local().Format("2006-01")
		if _, ok := monthly[month]; !ok {
			monthly[month] = &MonthlySpend{
				Month:      month,
				MonthLabel: monthLabel(timestamp.Local()),
			}
		}
		monthly[month].Total += value
		monthly[month].Count++
	}

	return nil
}

func monthLabel(timestamp time.Time) string {
	months := [...]string{
		"Janeiro",
		"Fevereiro",
		"Março",
		"Abril",
		"Maio",
		"Junho",
		"Julho",
		"Agosto",
		"Setembro",
		"Outubro",
		"Novembro",
		"Dezembro",
	}

	return fmt.Sprintf("%s de %d", months[timestamp.Month()-1], timestamp.Year())
}

func dataDir() string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		return "data"
	}

	return dir
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
