package main

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func appendPurchaseCSV(event CryptoPurchaseEvent) error {
	dir := dataDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	fileName := filepath.Join(dir, event.Timestamp.Local().Format("2006-01")+".csv")
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
		if err := writer.Write([]string{"value", "timestamp", "category"}); err != nil {
			return err
		}
	}
	if err := writer.Write([]string{event.Value, event.Timestamp.Format(time.RFC3339), ""}); err != nil {
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
		if err := readMonthlySpendsFile(fileName, monthly, fileSlug(fileName)); err != nil {
			log.Printf("failed to read spend file %s: %v", fileName, err)
		}
	}

	spends := make([]MonthlySpend, 0, len(monthly))
	for _, spend := range monthly {
		spends = append(spends, *spend)
	}

	sort.Slice(spends, func(i, j int) bool {
		return spends[i].Month > spends[j].Month
	})

	return spends, nil
}

func readMonthlySpendBySlug(slug string) (MonthlySpend, bool, error) {
	spends, err := readMonthlySpends()
	if err != nil {
		return MonthlySpend{}, false, err
	}

	for _, spend := range spends {
		if spend.Slug == slug {
			return spend, true, nil
		}
	}

	return MonthlySpend{}, false, nil
}

func readMonthlySpendsFile(fileName string, monthly map[string]*MonthlySpend, slug string) error {
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
				Slug:       slug,
				MonthLabel: monthLabel(timestamp.Local()),
			}
		}
		monthly[month].Total += value
		monthly[month].Count++
	}

	return nil
}

func fileSlug(fileName string) string {
	baseName := filepath.Base(fileName)
	return strings.TrimSuffix(baseName, filepath.Ext(baseName))
}

func dataDir() string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		return "data"
	}

	return dir
}
