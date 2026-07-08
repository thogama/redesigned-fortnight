package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func importCardCSVHandler(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.String(http.StatusBadRequest, "arquivo CSV nao enviado")
		return
	}

	openedFile, err := file.Open()
	if err != nil {
		c.String(http.StatusBadRequest, "falha ao abrir arquivo CSV")
		return
	}
	defer openedFile.Close()

	transactions, err := parseCryptoComCardCSV(openedFile)
	if err != nil {
		log.Printf("failed to parse card csv: %v", err)
		c.String(http.StatusBadRequest, "falha ao ler CSV da Crypto.com")
		return
	}

	if err := appendCardTransactionsCSV(transactions); err != nil {
		log.Printf("failed to write card csv import: %v", err)
		c.String(http.StatusInternalServerError, "falha ao salvar transacoes")
		return
	}

	c.Redirect(http.StatusSeeOther, "/")
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

func monthHandler(c *gin.Context) {
	month := c.Param("month")
	monthTimestamp, err := time.Parse("2006-01", month)
	if err != nil {
		c.String(http.StatusBadRequest, "mes invalido")
		return
	}

	transactions, expenseTotal, refundTotal, err := readMonthTransactions(month)
	if err != nil {
		log.Printf("failed to read month transactions: %v", err)
		c.String(http.StatusInternalServerError, "falha ao ler compras")
		return
	}
	categorySpends := categorySpends(transactions)
	netTotal := expenseTotal - refundTotal
	filterCategory := c.Query("category")
	filteredTransactions := filterTransactionsByCategory(transactions, filterCategory)

	data := MonthData{
		Month:                  month,
		MonthLabel:             monthLabel(monthTimestamp),
		Transactions:           filteredTransactions,
		HasTransactions:        len(transactions) > 0,
		HasVisibleTransactions: len(filteredTransactions) > 0,
		TotalBRLLabel:          formatBRL(netTotal),
		ExpenseTotalLabel:      formatBRL(expenseTotal),
		RefundTotalLabel:       formatBRL(refundTotal),
		NetTotalLabel:          formatBRL(netTotal),
		CategorySpends:         categorySpends,
		HasChartData:           len(categorySpends) > 0 || refundTotal > 0,
		CategoryOptions:        expenseCategories(),
		FilterOptions:          filterOptions(filterCategory),
		FilterCategory:         filterCategory,
		DefaultDateTime:        month + "-01T12:00",
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := monthTemplate.Execute(c.Writer, data); err != nil {
		log.Printf("failed to render month: %v", err)
	}
}

func addMonthTransactionHandler(c *gin.Context) {
	month := c.Param("month")
	if _, err := time.Parse("2006-01", month); err != nil {
		c.String(http.StatusBadRequest, "mes invalido")
		return
	}

	timestamp, err := time.ParseInLocation("2006-01-02T15:04", c.PostForm("timestamp"), appLocation())
	if err != nil {
		c.String(http.StatusBadRequest, "data invalida")
		return
	}

	amount, err := strconv.ParseFloat(c.PostForm("amount"), 64)
	if err != nil || amount <= 0 {
		c.String(http.StatusBadRequest, "valor invalido")
		return
	}

	refund := c.PostForm("kind") == "refund"
	if err := addManualTransaction(timestamp, c.PostForm("location"), amount, c.PostForm("category"), refund); err != nil {
		log.Printf("failed to add manual transaction: %v", err)
		c.String(http.StatusInternalServerError, "falha ao adicionar compra")
		return
	}

	c.Redirect(http.StatusSeeOther, "/months/"+month)
}

func deleteMonthTransactionHandler(c *gin.Context) {
	month := c.Param("month")
	if _, err := time.Parse("2006-01", month); err != nil {
		c.String(http.StatusBadRequest, "mes invalido")
		return
	}

	transactionIndex, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		c.String(http.StatusBadRequest, "compra invalida")
		return
	}

	if err := deleteMonthTransaction(month, transactionIndex); err != nil {
		log.Printf("failed to delete transaction: %v", err)
		c.String(http.StatusInternalServerError, "falha ao remover compra")
		return
	}

	c.Redirect(http.StatusSeeOther, "/months/"+month)
}

func updateMonthTransactionCategoryHandler(c *gin.Context) {
	month := c.Param("month")
	if _, err := time.Parse("2006-01", month); err != nil {
		c.String(http.StatusBadRequest, "mes invalido")
		return
	}

	transactionIndex, err := strconv.Atoi(c.Param("index"))
	if err != nil {
		c.String(http.StatusBadRequest, "compra invalida")
		return
	}

	if err := updateMonthTransactionCategory(month, transactionIndex, c.PostForm("category")); err != nil {
		log.Printf("failed to update transaction category: %v", err)
		c.String(http.StatusInternalServerError, "falha ao classificar compra")
		return
	}

	c.Redirect(http.StatusSeeOther, "/months/"+month)
}
