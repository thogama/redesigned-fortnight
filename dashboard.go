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

type StoredTransaction struct {
	Index           int
	Timestamp       time.Time
	TimestampLabel  string
	Location        string
	Amount          float64
	AmountLabel     string
	Currency        string
	USDAmount       float64
	Category        string
	CategoryOptions []CategoryOption
}

type CategoryOption struct {
	Value    string
	Selected bool
}

type FilterOption struct {
	Label    string
	Value    string
	Selected bool
}

type DashboardData struct {
	Spends       []MonthlySpend
	RateLabel    string
	RateError    string
	HasSpendData bool
}

type MonthData struct {
	Month                  string
	MonthLabel             string
	Transactions           []StoredTransaction
	HasTransactions        bool
	HasVisibleTransactions bool
	TotalBRLLabel          string
	ExpenseTotalLabel      string
	RefundTotalLabel       string
	NetTotalLabel          string
	CategorySpends         []CategorySpend
	HasChartData           bool
	CategoryOptions        []string
	FilterOptions          []FilterOption
	FilterCategory         string
	DefaultDateTime        string
}

type CategorySpend struct {
	Category string
	Total    float64
	Label    string
	Percent  string
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
    form { display: flex; gap: 10px; align-items: center; margin: 0 0 20px; padding: 14px 16px; background: #fff; border: 1px solid #d9e0ea; }
    input { flex: 1; min-width: 0; }
    button, .button { border: 0; background: #1d4ed8; color: #fff; padding: 9px 14px; cursor: pointer; text-decoration: none; display: inline-block; }
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
      <p>Somatório das compras importadas do CSV do cartão Crypto.com.</p>
    </header>
    <form action="/imports/cryptocom-card" method="post" enctype="multipart/form-data">
      <input type="file" name="file" accept=".csv,text/csv" required>
      <button type="submit">Importar CSV</button>
    </form>
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
          <th></th>
        </tr>
      </thead>
      <tbody>
        {{range .Spends}}
        <tr>
          <td>{{.MonthLabel}}</td>
          <td>{{.Count}}</td>
          <td>{{.TotalBRLLabel}}</td>
          <td><a class="button" href="/months/{{.Month}}">Ver compras</a></td>
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

var monthTemplate = htmltemplate.Must(htmltemplate.New("month").Parse(`<!doctype html>
<html lang="pt-BR">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{.MonthLabel}}</title>
  <style>
    :root { color-scheme: light; font-family: Arial, Helvetica, sans-serif; }
    body { margin: 0; background: #f5f7fb; color: #1d2433; }
    main { max-width: 1120px; margin: 0 auto; padding: 32px 20px; }
    header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; margin-bottom: 24px; }
    h1 { margin: 0 0 8px; font-size: 28px; line-height: 1.2; }
    p { margin: 0; color: #667085; }
    a, button { border: 0; background: #1d4ed8; color: #fff; padding: 9px 14px; cursor: pointer; text-decoration: none; display: inline-block; }
    form.add { display: grid; grid-template-columns: 1.2fr 2fr 1fr 1fr 1fr auto; gap: 10px; align-items: end; margin: 0 0 20px; padding: 14px 16px; background: #fff; border: 1px solid #d9e0ea; }
    label { display: grid; gap: 6px; color: #46566f; font-size: 13px; }
    input, select { min-width: 0; padding: 8px 10px; border: 1px solid #cdd6e3; background: #fff; color: #1d2433; }
    form.filter { display: flex; gap: 10px; align-items: end; margin: 0 0 20px; padding: 14px 16px; background: #fff; border: 1px solid #d9e0ea; }
    form.filter label { min-width: 220px; }
    table { width: 100%; border-collapse: collapse; background: #fff; border: 1px solid #d9e0ea; }
    th, td { padding: 12px 14px; border-bottom: 1px solid #e8edf3; text-align: left; }
    th { background: #eef3f9; font-size: 13px; text-transform: uppercase; color: #46566f; }
    td.amount, th.amount, td.actions, th.actions { text-align: right; }
    tr:last-child td { border-bottom: 0; }
    form.category { display: flex; gap: 8px; align-items: center; margin: 0; padding: 0; background: transparent; border: 0; }
    form.category select { min-width: 140px; }
    form.category button { padding: 8px 10px; }
    form.category button[hidden] { display: none; }
    form.delete { margin: 0; padding: 0; background: transparent; border: 0; }
    form.delete button { background: #b42318; }
    .chart { margin: 0 0 20px; padding: 16px; background: #fff; border: 1px solid #d9e0ea; }
    .chart h2 { margin: 0 0 14px; font-size: 16px; line-height: 1.2; }
    .bar-row { display: grid; grid-template-columns: 110px 1fr 100px; gap: 12px; align-items: center; margin: 10px 0; }
    .bar-label { color: #46566f; }
    .bar-track { height: 12px; background: #e8edf3; overflow: hidden; }
    .bar-fill { height: 100%; background: #1d4ed8; }
    .bar-value { text-align: right; }
    .chart-summary { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; margin-top: 16px; padding-top: 14px; border-top: 1px solid #e8edf3; }
    .summary-item { display: grid; gap: 4px; }
    .summary-label { color: #667085; font-size: 13px; }
    .summary-value { font-weight: 700; }
    .empty { padding: 20px; background: #fff; border: 1px solid #d9e0ea; }
    @media (max-width: 760px) {
      header { display: block; }
      header a { margin-top: 14px; }
      form.add { grid-template-columns: 1fr; }
      form.filter { display: grid; grid-template-columns: 1fr; }
      .bar-row { grid-template-columns: 1fr; gap: 6px; }
      .bar-value { text-align: left; }
      .chart-summary { grid-template-columns: 1fr; }
      table { font-size: 14px; }
    }
  </style>
</head>
<body>
  <main>
    <header>
      <div>
        <h1>{{.MonthLabel}}</h1>
        <p>{{.TotalBRLLabel}} em compras importadas e lançamentos manuais.</p>
      </div>
      <a href="/">Voltar</a>
    </header>

    <form class="add" action="/months/{{.Month}}/transactions" method="post">
      <label>Data e hora
        <input type="datetime-local" name="timestamp" value="{{.DefaultDateTime}}" required>
      </label>
      <label>Local
        <input type="text" name="location" placeholder="Ex.: Padaria" required>
      </label>
      <label>Valor
        <input type="number" name="amount" step="0.01" min="0.01" required>
      </label>
      <label>Tipo
        <select name="kind" required>
          <option value="expense">Gasto</option>
          <option value="refund">Ressarcimento</option>
        </select>
      </label>
      <label>Categoria
        <select name="category" required>
          {{range .CategoryOptions}}<option value="{{.}}">{{.}}</option>{{end}}
        </select>
      </label>
      <button type="submit">Adicionar</button>
    </form>

    {{if .HasTransactions}}
    {{if .HasChartData}}
    <section class="chart">
      <h2>Gastos por categoria</h2>
      {{range .CategorySpends}}
      <div class="bar-row">
        <div class="bar-label">{{.Category}}</div>
        <div class="bar-track"><div class="bar-fill" style="width: {{.Percent}}"></div></div>
        <div class="bar-value">{{.Label}}</div>
      </div>
      {{end}}
      <div class="chart-summary">
        <div class="summary-item">
          <span class="summary-label">Gastos</span>
          <span class="summary-value">{{.ExpenseTotalLabel}}</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">Ressarcimentos</span>
          <span class="summary-value">{{.RefundTotalLabel}}</span>
        </div>
        <div class="summary-item">
          <span class="summary-label">Total</span>
          <span class="summary-value">{{.NetTotalLabel}}</span>
        </div>
      </div>
    </section>
    {{end}}
    <form class="filter" action="/months/{{.Month}}" method="get">
      <label>Categoria
        <select name="category" onchange="this.form.submit()">
          {{range .FilterOptions}}<option value="{{.Value}}" {{if .Selected}}selected{{end}}>{{.Label}}</option>{{end}}
        </select>
      </label>
      <noscript><button type="submit">Filtrar</button></noscript>
    </form>
    <table>
      <thead>
        <tr>
          <th>Data</th>
          <th>Local</th>
          <th>Categoria</th>
          <th class="amount">Valor</th>
          <th class="actions"></th>
        </tr>
      </thead>
      <tbody>
        {{range .Transactions}}
        <tr>
          <td>{{.TimestampLabel}}</td>
          <td>{{.Location}}</td>
          <td>
            <form class="category" action="/months/{{$.Month}}/transactions/{{.Index}}/category" method="post">
              <select name="category" data-original="{{.Category}}" required>
                {{range .CategoryOptions}}<option value="{{.Value}}" {{if .Selected}}selected{{end}}>{{.Value}}</option>{{end}}
              </select>
              <button type="submit" hidden>Salvar</button>
            </form>
          </td>
          <td class="amount">{{.AmountLabel}}</td>
          <td class="actions">
            <form class="delete" action="/months/{{$.Month}}/transactions/{{.Index}}/delete" method="post">
              <button type="submit">Remover</button>
            </form>
          </td>
        </tr>
        {{end}}
      </tbody>
    </table>
    {{else}}
    <div class="empty">Nenhuma compra registrada nesse mês.</div>
    {{end}}
  </main>
  <script>
    document.querySelectorAll('form.category select').forEach((select) => {
      const button = select.form.querySelector('button[type="submit"]');
      const sync = () => {
        button.hidden = select.value === select.dataset.original;
      };
      select.addEventListener('change', sync);
      sync();
    });
  </script>
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

func formatSignedBRL(value float64) string {
	if value > 0 {
		return fmt.Sprintf("+R$ %.2f", value)
	}
	return fmt.Sprintf("R$ %.2f", -value)
}
