package render

import "time"

// RenderInput is the deterministic input used for invoice rendering.
type RenderInput struct {
	Template TemplateView
	Invoice  InvoiceView
	Customer CustomerView
	Items    []LineItemView
}

type TemplateView struct {
	Name         string
	Locale       string
	Currency     string
	LogoURL      string
	CompanyName  string
	FooterNotes  string
	FooterLegal  string
	PrimaryColor string
	FontFamily   string
}

type InvoiceView struct {
	ID             string
	Number         string
	Status         string
	IssuedAt       *time.Time
	DueAt          *time.Time
	PeriodStart    *time.Time
	PeriodEnd      *time.Time
	SubtotalAmount int64
	Currency       string
}

type CustomerView struct {
	Name  string
	Email string
}

type LineItemView struct {
	Description string
	Quantity    float64
	UnitPrice   int64
	Amount      int64
}

type Renderer interface {
	RenderHTML(input RenderInput) (string, error)
}
