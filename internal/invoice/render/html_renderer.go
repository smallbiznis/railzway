package render

import (
	"bytes"
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"
)

const invoiceHTMLTemplate = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>Invoice {{.Invoice.Number}}</title>
  <style>
    :root {
      --primary: {{.Template.PrimaryColor}};
      --font: "{{.Template.FontFamily}}";
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      padding: 32px;
      font-family: var(--font), "Helvetica Neue", Arial, sans-serif;
      color: #111827;
      background: #ffffff;
    }
    .invoice {
      max-width: 820px;
      margin: 0 auto;
    }
    .header {
      display: flex;
      justify-content: space-between;
      align-items: flex-start;
      border-bottom: 2px solid var(--primary);
      padding-bottom: 16px;
      margin-bottom: 24px;
    }
    .brand {
      display: flex;
      align-items: center;
      gap: 12px;
    }
    .brand img {
      max-height: 48px;
    }
    .meta {
      text-align: right;
      font-size: 14px;
    }
    .meta .label {
      color: #6b7280;
      text-transform: uppercase;
      letter-spacing: 0.04em;
      font-size: 11px;
    }
    .section {
      margin-bottom: 24px;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      font-size: 14px;
    }
    th, td {
      padding: 10px;
      border-bottom: 1px solid #e5e7eb;
      text-align: left;
    }
    th {
      text-transform: uppercase;
      font-size: 11px;
      letter-spacing: 0.04em;
      color: #6b7280;
    }
    .totals {
      margin-top: 12px;
      display: flex;
      justify-content: flex-end;
      font-size: 16px;
    }
    .totals strong {
      margin-left: 12px;
    }
    .footer {
      border-top: 1px solid #e5e7eb;
      padding-top: 16px;
      font-size: 12px;
      color: #6b7280;
    }
  </style>
</head>
<body>
  <div class="invoice">
    <div class="header">
      <div class="brand">
        {{if .Template.LogoURL}}
        <img src="{{.Template.LogoURL}}" alt="Company logo" />
        {{end}}
        <div>
          <div><strong>{{.Template.CompanyName}}</strong></div>
          <div>{{.Customer.Name}}</div>
          <div>{{.Customer.Email}}</div>
        </div>
      </div>
      <div class="meta">
        <div class="label">Invoice</div>
        <div><strong>{{.Invoice.Number}}</strong></div>
        <div>Status: {{.Invoice.Status}}</div>
        <div>Issued: {{formatDate .Invoice.IssuedAt}}</div>
        <div>Due: {{formatDate .Invoice.DueAt}}</div>
      </div>
    </div>

    <div class="section">
      <div class="label">Billing Period</div>
      <div>{{formatDate .Invoice.PeriodStart}} - {{formatDate .Invoice.PeriodEnd}}</div>
    </div>

    <div class="section">
      <table>
        <thead>
          <tr>
            <th>Description</th>
            <th>Quantity</th>
            <th>Unit Price</th>
            <th>Amount</th>
          </tr>
        </thead>
        <tbody>
          {{range .Items}}
          <tr>
            <td>{{.Description}}</td>
            <td>{{formatQuantity .Quantity}}</td>
            <td>{{formatMoney .UnitPrice $.Invoice.Currency}}</td>
            <td>{{formatMoney .Amount $.Invoice.Currency}}</td>
          </tr>
          {{end}}
        </tbody>
      </table>
      <div class="totals">
        <span>Total</span>
        <strong>{{formatMoney .Invoice.SubtotalAmount .Invoice.Currency}}</strong>
      </div>
    </div>

    <div class="footer">
      {{if .Template.FooterNotes}}<div>{{.Template.FooterNotes}}</div>{{end}}
      {{if .Template.FooterLegal}}<div>{{.Template.FooterLegal}}</div>{{end}}
    </div>
  </div>
</body>
</html>
`

var (
	hexColorPattern  = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	fontFamilyFilter = regexp.MustCompile(`^[A-Za-z0-9 \-]+$`)
)

type HTMLRenderer struct {
	tpl *template.Template
}

func NewRenderer() Renderer {
	funcs := template.FuncMap{
		"formatMoney":    formatMoney,
		"formatDate":     formatDate,
		"formatQuantity": formatQuantity,
	}
	return &HTMLRenderer{
		tpl: template.Must(template.New("invoice").Funcs(funcs).Parse(invoiceHTMLTemplate)),
	}
}

func (r *HTMLRenderer) RenderHTML(input RenderInput) (string, error) {
	input.Template.PrimaryColor = sanitizeColor(input.Template.PrimaryColor)
	input.Template.FontFamily = sanitizeFont(input.Template.FontFamily)
	if input.Template.CompanyName == "" {
		input.Template.CompanyName = "Invoice"
	}

	var buf bytes.Buffer
	if err := r.tpl.Execute(&buf, input); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func formatMoney(amount int64, currency string) string {
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "" {
		currency = "USD"
	}
	value := float64(amount) / 100.0
	return fmt.Sprintf("%s %.2f", currency, value)
}

func formatDate(value *time.Time) string {
	if value == nil || value.IsZero() {
		return "-"
	}
	return value.UTC().Format("2006-01-02")
}

func formatQuantity(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
}

func sanitizeColor(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "#111827"
	}
	if hexColorPattern.MatchString(trimmed) {
		return trimmed
	}
	return "#111827"
}

func sanitizeFont(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "Space Grotesk"
	}
	if fontFamilyFilter.MatchString(trimmed) {
		return trimmed
	}
	return "Space Grotesk"
}
