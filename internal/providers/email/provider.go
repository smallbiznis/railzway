package email

import "context"

type Attachment struct {
	Filename string
	Content  []byte
}

type EmailMessage struct {
	To          []string
	From        string // Optional, overrides config default completely
	SenderName  string // Optional, overrides default name but keeps default address (if From is empty)
	ReplyTo     string
	Subject     string
	HTMLBody    string // For Send()
	Attachments []Attachment
}

type Provider interface {
	Send(ctx context.Context, msg EmailMessage) error
	SendTemplate(ctx context.Context, msg EmailMessage, templateName string, data interface{}) error
}

type NoOpProvider struct{}

func (p *NoOpProvider) Send(ctx context.Context, msg EmailMessage) error {
	return nil
}

func (p *NoOpProvider) SendTemplate(ctx context.Context, msg EmailMessage, templateName string, data interface{}) error {
	return nil
}
