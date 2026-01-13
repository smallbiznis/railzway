package email

import (
	"github.com/smallbiznis/valora/internal/config"
	"go.uber.org/fx"
)

var Module = fx.Module("providers.email",
	fx.Provide(NewFromConfig),
)

func NewFromConfig(cfg config.Config) Provider {
	// Defaults are already handled in internal/config
	emailCfg := Config{
		Host:     cfg.Email.SMTPHost,
		Port:     cfg.Email.SMTPPort,
		Username: cfg.Email.SMTPUsername,
		Password: cfg.Email.SMTPPassword,
		From:     cfg.Email.SMTPFrom,
	}
	return NewSMTP(emailCfg)
}
