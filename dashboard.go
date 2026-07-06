package main

import (
	"fmt"
	htmltemplate "html/template"
	"time"
)

type MonthlySpend struct {
	Month         string
	MonthLabel    string
	Total         float64
	TotalBRLLabel string
	Count         int
}

type DashboardData struct {
	Spends       []MonthlySpend
	RateLabel    string
	RateError    string
	HasSpendData bool
}

var dashboardTemplate = htmltemplate.Must(htmltemplate.New("dashboard").Parse(`<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Gastos</title>
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
    {{if .RateLabel}}
    <p style="margin-bottom:16px">Cotação usada: {{.RateLabel}}</p>
    {{else if .RateError}}
    <p style="margin-bottom:16px">Cotação indisponível: {{.RateError}}</p>
    {{end}}
    {{if .HasSpendData}}
    <table>
      <thead>
        <tr>
          <th>Mês</th>
          <th>Compras</th>
          <th>Total</th>
        </tr>
      </thead>
      <tbody>
        {{range .Spends}}
        <tr>
          <td>{{.MonthLabel}}</td>
          <td>{{.Count}}</td>
          <td>{{.TotalBRLLabel}}</td>
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

func formatBRL(value float64) string {
	return fmt.Sprintf("R$ %.2f", value)
}
