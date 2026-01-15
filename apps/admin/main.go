package main

import (
	"github.com/bwmarrin/snowflake"
	"github.com/smallbiznis/railzway/internal/apikey"
	"github.com/smallbiznis/railzway/internal/audit"
	"github.com/smallbiznis/railzway/internal/auth"
	authlocal "github.com/smallbiznis/railzway/internal/auth/local"
	authoauth2provider "github.com/smallbiznis/railzway/internal/auth/oauth2provider"
	"github.com/smallbiznis/railzway/internal/auth/session"
	"github.com/smallbiznis/railzway/internal/authorization"
	"github.com/smallbiznis/railzway/internal/billingdashboard"
	"github.com/smallbiznis/railzway/internal/billingoperations"
	"github.com/smallbiznis/railzway/internal/billingoverview"
	"github.com/smallbiznis/railzway/internal/clock"
	"github.com/smallbiznis/railzway/internal/cloudmetrics"
	"github.com/smallbiznis/railzway/internal/config"
	"github.com/smallbiznis/railzway/internal/customer"
	"github.com/smallbiznis/railzway/internal/events"
	"github.com/smallbiznis/railzway/internal/feature"
	"github.com/smallbiznis/railzway/internal/invoice"
	"github.com/smallbiznis/railzway/internal/invoicetemplate"
	"github.com/smallbiznis/railzway/internal/ledger"
	"github.com/smallbiznis/railzway/internal/meter"
	"github.com/smallbiznis/railzway/internal/observability"
	"github.com/smallbiznis/railzway/internal/organization"
	"github.com/smallbiznis/railzway/internal/payment"
	"github.com/smallbiznis/railzway/internal/price"
	"github.com/smallbiznis/railzway/internal/priceamount"
	"github.com/smallbiznis/railzway/internal/pricetier"
	"github.com/smallbiznis/railzway/internal/product"
	"github.com/smallbiznis/railzway/internal/productfeature"
	"github.com/smallbiznis/railzway/internal/providers/email"
	paymentprovider "github.com/smallbiznis/railzway/internal/providers/payment"
	"github.com/smallbiznis/railzway/internal/providers/pdf"
	"github.com/smallbiznis/railzway/internal/ratelimit"
	"github.com/smallbiznis/railzway/internal/rating"
	"github.com/smallbiznis/railzway/internal/reference"
	"github.com/smallbiznis/railzway/internal/server"
	"github.com/smallbiznis/railzway/internal/subscription"
	"github.com/smallbiznis/railzway/internal/usage"
	"github.com/smallbiznis/railzway/pkg/db"
	"go.uber.org/fx"
)

func main() {
	app := fx.New(
		config.Module,
		cloudmetrics.Module,
		observability.Module,
		fx.Provide(RegisterSnowflake),
		db.Module,
		clock.Module,

		// Admin needs almost everything
		authorization.Module,
		audit.Module,
		events.Module,
		auth.Module,
		authlocal.Module,
		authoauth2provider.Module,
		session.Module,
		apikey.Module,
		customer.Module,
		billingdashboard.Module,
		billingoperations.Module,
		email.Module,
		pdf.Module,
		billingoverview.Module,
		invoice.Module,
		invoicetemplate.Module,
		ledger.Module,
		meter.Module,
		organization.Module,
		price.Module,
		priceamount.Module,
		pricetier.Module,
		product.Module,
		productfeature.Module,
		feature.Module,
		payment.Module,
		paymentprovider.Module,
		reference.Module,
		rating.Module,
		ratelimit.Module,
		subscription.Module,
		usage.Module, // Needed for dashboard stats, but maybe not ingestion

		fx.Provide(server.NewEngine),
		fx.Provide(server.NewServer),
		fx.Invoke(func(s *server.Server) {
			s.RegisterAuthRoutes()
			s.RegisterAdminRoutes()
			s.RegisterUIRoutes() // Monolith style: serve the react app
			s.RegisterFallback()
		}),
		fx.Invoke(server.RunHTTP),
	)
	app.Run()
}

func RegisterSnowflake() *snowflake.Node {
	node, err := snowflake.NewNode(1)
	if err != nil {
		panic(err)
	}
	return node
}
