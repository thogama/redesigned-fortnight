package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

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
		if err := appendPurchaseCSV(event); err != nil {
			log.Printf("failed to write purchase csv: %v", err)
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

	data := DashboardData{
		Spends:       spends,
		HasSpendData: len(spends) > 0,
	}
	for index := range data.Spends {
		data.Spends[index].TotalBRLLabel = formatBRL(data.Spends[index].Total)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := dashboardTemplate.Execute(c.Writer, data); err != nil {
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
