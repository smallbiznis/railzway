package email

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"net/smtp"
	"path/filepath"
	"strings"
)

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type SMTPProvider struct {
	cfg Config
}

func NewSMTP(cfg Config) *SMTPProvider {
	return &SMTPProvider{cfg: cfg}
}

func (p *SMTPProvider) Send(ctx context.Context, msg EmailMessage) error {
	auth := smtp.PlainAuth("", p.cfg.Username, p.cfg.Password, p.cfg.Host)
	addr := fmt.Sprintf("%s:%d", p.cfg.Host, p.cfg.Port)

	// Parse default address from config
	defaultAddr := p.cfg.From
	if start := strings.Index(p.cfg.From, "<"); start != -1 {
		if end := strings.Index(p.cfg.From, ">"); end != -1 && end > start {
			defaultAddr = p.cfg.From[start+1 : end]
		}
	}

	fromStr := p.cfg.From // The Header value
	envelopeSender := defaultAddr // The Envelope Sender

	if msg.From != "" {
		fromStr = msg.From
		// Extract address for envelope if needed
		envelopeSender = msg.From
		if start := strings.Index(msg.From, "<"); start != -1 {
			if end := strings.Index(msg.From, ">"); end != -1 && end > start {
				envelopeSender = msg.From[start+1 : end]
			}
		}
	} else if msg.SenderName != "" {
		fromStr = fmt.Sprintf("%s <%s>", msg.SenderName, defaultAddr)
		// envelopeSender remains defaultAddr
	}

	// Use Multipart MIME if attachments exist
	var body bytes.Buffer
	boundary := "boundary-valora-123456789" // Static boundary for MVP

	// Header
	body.WriteString(fmt.Sprintf("From: %s\r\n", fromStr))
	if msg.ReplyTo != "" {
		body.WriteString(fmt.Sprintf("Reply-To: %s\r\n", msg.ReplyTo))
	}
	body.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ",")))
	body.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	body.WriteString("MIME-Version: 1.0\r\n")
	body.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%s\r\n", boundary))
	body.WriteString("\r\n")

	// HTML Body Part
	body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	body.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	body.WriteString("\r\n")
	body.WriteString(msg.HTMLBody)
	body.WriteString("\r\n")

	// Attachments Parts
	for _, att := range msg.Attachments {
		body.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		body.WriteString(fmt.Sprintf("Content-Type: application/pdf; name=\"%s\"\r\n", att.Filename))
		body.WriteString("Content-Transfer-Encoding: base64\r\n")
		body.WriteString(fmt.Sprintf("Content-Disposition: attachment; filename=\"%s\"\r\n", att.Filename))
		body.WriteString("\r\n")

		encoded := base64.StdEncoding.EncodeToString(att.Content)
		// Split lines for email safety (optional but good practice, verify if smtp handles raw long lines)
		// Go's smtp.SendMail handles dot transparency but not line length limit strictly on body.
		// For MVP, just dumping base64.
		body.WriteString(encoded)
		body.WriteString("\r\n")
	}

	// End
	body.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return smtp.SendMail(addr, auth, envelopeSender, msg.To, body.Bytes())
}

func (p *SMTPProvider) SendTemplate(ctx context.Context, msg EmailMessage, templateName string, data interface{}) error {
	// Locate template file (Assumes running from project root or configured path)
	tmplPath := filepath.Join("internal", "providers", "email", "templates", templateName+".html")

	t, err := template.ParseFiles(tmplPath)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var body bytes.Buffer
	if err := t.Execute(&body, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	msg.HTMLBody = body.String()

	// Extract subject from data if provided and not already set in msg
	if msg.Subject == "" {
		subject := "Notification from Railzway"
		if dataMap, ok := data.(map[string]interface{}); ok {
			if subj, exists := dataMap["subject"]; exists {
				if subjStr, ok := subj.(string); ok {
					subject = subjStr
				}
			} else {
				// Use template-specific defaults
				switch templateName {
				case "invite_member":
					if orgName, ok := dataMap["org_name"].(string); ok {
						subject = fmt.Sprintf("You're invited to join %s", orgName)
					} else {
						subject = "You're invited to join a team"
					}
				case "invoice_new":
					subject = "New invoice from Railzway" // Fallback, usually overridden by service now
				}
			}
		}
		msg.Subject = subject
	}

	return p.Send(ctx, msg)
}
