package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/smallbiznis/valora/internal/apikey"
	apikeydomain "github.com/smallbiznis/valora/internal/apikey/domain"
	"github.com/smallbiznis/valora/internal/audit"
	auditdomain "github.com/smallbiznis/valora/internal/audit/domain"
	"github.com/smallbiznis/valora/internal/auth"
	authdomain "github.com/smallbiznis/valora/internal/auth/domain"
	authlocal "github.com/smallbiznis/valora/internal/auth/local"
	authoauth "github.com/smallbiznis/valora/internal/auth/oauth"
	authoauth2provider "github.com/smallbiznis/valora/internal/auth/oauth2provider"
	"github.com/smallbiznis/valora/internal/auth/session"
	"github.com/smallbiznis/valora/internal/authorization"
	"github.com/smallbiznis/valora/internal/billingdashboard"
	billingdashboarddomain "github.com/smallbiznis/valora/internal/billingdashboard/domain"
	billingrollup "github.com/smallbiznis/valora/internal/billingdashboard/rollup"
	"github.com/smallbiznis/valora/internal/billingoverview"
	billingoverviewdomain "github.com/smallbiznis/valora/internal/billingoverview/domain"
	"github.com/smallbiznis/valora/internal/cloudmetrics"
	"github.com/smallbiznis/valora/internal/config"
	"github.com/smallbiznis/valora/internal/customer"
	customerdomain "github.com/smallbiznis/valora/internal/customer/domain"
	"github.com/smallbiznis/valora/internal/events"
	"github.com/smallbiznis/valora/internal/invoice"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	"github.com/smallbiznis/valora/internal/invoicetemplate"
	invoicetemplatedomain "github.com/smallbiznis/valora/internal/invoicetemplate/domain"
	"github.com/smallbiznis/valora/internal/ledger"
	"github.com/smallbiznis/valora/internal/meter"
	meterdomain "github.com/smallbiznis/valora/internal/meter/domain"
	"github.com/smallbiznis/valora/internal/observability"
	obsmiddleware "github.com/smallbiznis/valora/internal/observability/logger"
	obsmetrics "github.com/smallbiznis/valora/internal/observability/metrics"
	obstracing "github.com/smallbiznis/valora/internal/observability/tracing"
	"github.com/smallbiznis/valora/internal/organization"
	organizationdomain "github.com/smallbiznis/valora/internal/organization/domain"
	"github.com/smallbiznis/valora/internal/payment"
	paymentdomain "github.com/smallbiznis/valora/internal/payment/domain"
	"github.com/smallbiznis/valora/internal/paymentprovider"
	paymentproviderdomain "github.com/smallbiznis/valora/internal/paymentprovider/domain"
	"github.com/smallbiznis/valora/internal/price"
	pricedomain "github.com/smallbiznis/valora/internal/price/domain"
	"github.com/smallbiznis/valora/internal/priceamount"
	priceamountdomain "github.com/smallbiznis/valora/internal/priceamount/domain"
	"github.com/smallbiznis/valora/internal/pricetier"
	pricetierdomain "github.com/smallbiznis/valora/internal/pricetier/domain"
	"github.com/smallbiznis/valora/internal/product"
	productdomain "github.com/smallbiznis/valora/internal/product/domain"
	"github.com/smallbiznis/valora/internal/publicinvoice"
	publicinvoicedomain "github.com/smallbiznis/valora/internal/publicinvoice/domain"
	"github.com/smallbiznis/valora/internal/ratelimit"
	"github.com/smallbiznis/valora/internal/rating"
	ratingdomain "github.com/smallbiznis/valora/internal/rating/domain"
	"github.com/smallbiznis/valora/internal/reference"
	referencedomain "github.com/smallbiznis/valora/internal/reference/domain"
	"github.com/smallbiznis/valora/internal/scheduler"
	signupdomain "github.com/smallbiznis/valora/internal/signup/domain"
	"github.com/smallbiznis/valora/internal/subscription"
	subscriptiondomain "github.com/smallbiznis/valora/internal/subscription/domain"
	"github.com/smallbiznis/valora/internal/usage"
	usagedomain "github.com/smallbiznis/valora/internal/usage/domain"
	"go.uber.org/fx"
	"gorm.io/gorm"
)

var Module = fx.Module("http.server",
	config.Module,
	cloudmetrics.Module,
	fx.Provide(registerGin),
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
	payment.Module,
	paymentprovider.Module,
	publicinvoice.Module,
	reference.Module,
	rating.Module,
	ratelimit.Module,
	subscription.Module,
	usage.Module,
	fx.Invoke(NewServer),
	fx.Invoke(run),
)

func NewEngine(obsCfg observability.Config, httpMetrics *obsmetrics.HTTPMetrics) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(obsmiddleware.GinMiddleware(obsmiddleware.MiddlewareConfig{
		Debug:           obsCfg.Debug(),
		ErrorClassifier: classifyErrorForLog,
	}))
	r.Use(obstracing.GinMiddleware())
	r.Use(obsmetrics.GinMiddleware(httpMetrics))
	r.Use(ErrorHandlingMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	return r
}

func registerGin(obsCfg observability.Config, httpMetrics *obsmetrics.HTTPMetrics) *gin.Engine {
	return NewEngine(obsCfg, httpMetrics)
}

func run(lc fx.Lifecycle, r *gin.Engine) {
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					panic(err)
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			return srv.Shutdown(shutdownCtx)
		},
	})
}

type Server struct {
	engine                      *gin.Engine
	cfg                         config.Config
	db                          *gorm.DB
	authsvc                     authdomain.Service
	oauthsvc                    authoauth.Service
	sessions                    *session.Manager
	genID                       *snowflake.Node
	apiKeySvc                   apikeydomain.Service
	apiKeyLimiter               *rateLimiter
	authzSvc                    authorization.Service
	auditSvc                    auditdomain.Service
	billingDashboardSvc         billingdashboarddomain.Service
	billingOverviewSvc          billingoverviewdomain.Service
	billingRollup               *billingrollup.Service
	invoiceSvc                  invoicedomain.Service
	meterSvc                    meterdomain.Service
	organizationSvc             organizationdomain.Service
	customerSvc                 customerdomain.Service
	priceSvc                    pricedomain.Service
	priceAmountSvc              priceamountdomain.Service
	priceTierSvc                pricetierdomain.Service
	productSvc                  productdomain.Service
	paymentSvc                  paymentdomain.Service
	paymentProviderSvc          paymentproviderdomain.Service
	invoiceTemplateSvc          invoicetemplatedomain.Service
	refrepo                     referencedomain.Repository
	signupsvc                   signupdomain.Service
	ratingSvc                   ratingdomain.Service
	subscriptionSvc             subscriptiondomain.Service
	usagesvc                    usagedomain.Service
	obsMetrics                  *obsmetrics.Metrics
	usageLimiter                *ratelimit.UsageIngestLimiter
	publicInvoiceSvc            publicinvoicedomain.Service
	publicInvoiceLimiter        *rateLimiter
	publicPaymentIntentLimiter  *rateLimiter
	publicPaymentMethodsLimiter *rateLimiter
	publicPaymentMethodsCache   *paymentMethodsCache

	scheduler *scheduler.Scheduler `optional:"true"`
}

type ServerParams struct {
	fx.In

	Gin                 *gin.Engine
	Cfg                 config.Config
	DB                  *gorm.DB
	Authsvc             authdomain.Service
	OAuthsvc            authoauth.Service
	Sessions            *session.Manager
	GenID               *snowflake.Node
	APIKeySvc           apikeydomain.Service
	AuthzSvc            authorization.Service
	AuditSvc            auditdomain.Service
	BillingDashboardSvc billingdashboarddomain.Service
	BillingOverviewSvc  billingoverviewdomain.Service
	BillingRollup       *billingrollup.Service
	InvoiceSvc          invoicedomain.Service
	MeterSvc            meterdomain.Service
	OrganizationSvc     organizationdomain.Service
	CustomerSvc         customerdomain.Service
	PriceSvc            pricedomain.Service
	PriceAmountSvc      priceamountdomain.Service
	PriceTierSvc        pricetierdomain.Service
	ProductSvc          productdomain.Service
	PaymentSvc          paymentdomain.Service
	PaymentProviderSvc  paymentproviderdomain.Service
	InvoiceTemplateSvc  invoicetemplatedomain.Service
	Refrepo             referencedomain.Repository
	RatingSvc           ratingdomain.Service
	SubscriptionSvc     subscriptiondomain.Service
	Usagesvc            usagedomain.Service
	PublicInvoiceSvc    publicinvoicedomain.Service
	ObsMetrics          *obsmetrics.Metrics           `optional:"true"`
	UsageLimiter        *ratelimit.UsageIngestLimiter `optional:"true"`

	Scheduler *scheduler.Scheduler
}

func NewServer(p ServerParams) *Server {
	svc := &Server{
		engine:                      p.Gin,
		cfg:                         p.Cfg,
		db:                          p.DB,
		authsvc:                     p.Authsvc,
		oauthsvc:                    p.OAuthsvc,
		sessions:                    p.Sessions,
		genID:                       p.GenID,
		apiKeySvc:                   p.APIKeySvc,
		apiKeyLimiter:               newRateLimiter(5, 10*time.Minute),
		authzSvc:                    p.AuthzSvc,
		auditSvc:                    p.AuditSvc,
		billingDashboardSvc:         p.BillingDashboardSvc,
		billingOverviewSvc:          p.BillingOverviewSvc,
		billingRollup:               p.BillingRollup,
		invoiceSvc:                  p.InvoiceSvc,
		meterSvc:                    p.MeterSvc,
		organizationSvc:             p.OrganizationSvc,
		customerSvc:                 p.CustomerSvc,
		priceSvc:                    p.PriceSvc,
		priceAmountSvc:              p.PriceAmountSvc,
		priceTierSvc:                p.PriceTierSvc,
		productSvc:                  p.ProductSvc,
		paymentSvc:                  p.PaymentSvc,
		paymentProviderSvc:          p.PaymentProviderSvc,
		invoiceTemplateSvc:          p.InvoiceTemplateSvc,
		refrepo:                     p.Refrepo,
		ratingSvc:                   p.RatingSvc,
		subscriptionSvc:             p.SubscriptionSvc,
		usagesvc:                    p.Usagesvc,
		obsMetrics:                  p.ObsMetrics,
		usageLimiter:                p.UsageLimiter,
		publicInvoiceSvc:            p.PublicInvoiceSvc,
		publicInvoiceLimiter:        newRateLimiter(30, time.Minute),
		publicPaymentIntentLimiter:  newRateLimiter(5, time.Minute),
		publicPaymentMethodsLimiter: newRateLimiter(30, time.Minute),
		publicPaymentMethodsCache:   newPaymentMethodsCache(2 * time.Minute),
		scheduler:                   p.Scheduler,
	}

	svc.registerAuthRoutes()
	svc.registerAPIRoutes()
	svc.registerAdminRoutes()
	svc.registerPublicRoutes()
	svc.registerUIRoutes()
	svc.registerFallback()
	svc.RegisterDevBillingRoutes()

	return svc
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) registerAuthRoutes() {
	auth := s.engine.Group("/auth")

	auth.POST("/login", s.Login)
	auth.POST("/logout", s.Logout)
	auth.POST("/change-password", s.WebAuthRequired(), s.ChangePassword)
	auth.POST("/forgot", s.Forgot)
	auth.GET("/me", s.Me)

	user := auth.Group("/user", s.WebAuthRequired())
	{
		user.GET("/orgs", s.ListUserOrgs)
		user.POST("/using/:orgId", s.UseOrg)
	}
}

func (s *Server) registerAPIRoutes() {
	api := s.engine.Group("/api")

	api.GET("/countries", s.ListCountries)
	api.GET("/timezones", s.ListTimezones)
	api.GET("/currencies", s.ListCurrencies)

	// -------- Meters --------
	api.GET("/meters", s.APIKeyRequired(), s.ListMeters)
	api.GET("/meters/:id", s.APIKeyRequired(), s.GetMeterByID)

	// -------- Product --------
	api.GET("/products", s.APIKeyRequired(), s.ListProducts)
	api.POST("/products", s.APIKeyRequired(), s.CreateProduct)
	api.GET("/products/:id", s.APIKeyRequired(), s.GetProductByID)

	// -------- Pricing --------
	api.GET("/pricings", s.APIKeyRequired(), s.ListPricings)
	api.GET("/pricings/:id", s.APIKeyRequired(), s.GetPricingByID)

	// -------- Prices --------
	api.GET("/prices", s.APIKeyRequired(), s.ListPrices)
	api.GET("/prices/:id", s.APIKeyRequired(), s.GetPriceByID)

	// -------- Price Amounts --------
	api.GET("/price_amounts", s.APIKeyRequired(), s.ListPriceAmounts)
	api.GET("/price_amounts/:id", s.APIKeyRequired(), s.GetPriceAmountByID)

	// -------- Tiers ---------
	api.GET("/price_tiers", s.APIKeyRequired(), s.ListPriceTiers)
	api.GET("/price_tiers/:id", s.APIKeyRequired(), s.GetPriceTierByID)

	// -------- Subscriptions --------
	// Shared handlers, different gates: API keys use scopes, admin uses RBAC.
	api.GET("/subscriptions", s.APIKeyRequired(), s.ListSubscriptions)
	api.POST("/subscriptions", s.APIKeyRequired(), s.CreateSubscription)
	api.GET("/subscriptions/:id", s.APIKeyRequired(), s.GetSubscriptionByID)
	api.POST("/subscriptions/:id/activate", s.APIKeyRequired(), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionActivate), s.ActivateSubscription)
	api.POST("/subscriptions/:id/pause", s.APIKeyRequired(), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionPause), s.PauseSubscription)
	api.POST("/subscriptions/:id/resume", s.APIKeyRequired(), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionResume), s.ResumeSubscription)
	api.POST("/subscriptions/:id/cancel", s.APIKeyRequired(), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionCancel), s.CancelSubscription)

	// -------- Invoices --------
	api.GET("/invoices", s.APIKeyRequired(), s.ListInvoices)
	api.GET("/invoices/:id", s.APIKeyRequired(), s.GetInvoiceByID)

	// -------- Customers --------
	api.GET("/customers", s.APIKeyRequired(), s.ListCustomers)
	api.POST("/customers", s.APIKeyRequired(), s.CreateCustomer)
	api.GET("/customers/:id", s.APIKeyRequired(), s.GetCustomerByID)

	// -------- Payment Webhooks --------
	api.POST("/payments/webhooks/:provider", s.HandlePaymentWebhook)

	api.POST("/usage", s.APIKeyRequired(), s.UsageIngestRateLimit(), s.IngestUsage)

	if s.cfg.Environment != "production" {
		api.POST("/test/cleanup", s.TestCleanup)
	}
}

func (s *Server) registerAdminRoutes() {
	admin := s.engine.Group("/admin")

	// --- global middlewares ---
	admin.Use(s.WebAuthRequired())
	admin.Use(s.OrgContext())

	admin.GET("/meters", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListMeters)
	admin.POST("/meters", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateMeter)
	admin.GET("/meters/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetMeterByID)
	admin.PATCH("/meters/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.UpdateMeter)
	admin.DELETE("/meters/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.DeleteMeter)

	// -------- Product --------
	admin.GET("/products", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListProducts)
	admin.POST("/products", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateProduct)
	admin.GET("/products/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetProductByID)

	// -------- Pricing --------
	admin.GET("/pricings", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListPricings)
	admin.POST("/pricings", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreatePricing)
	admin.GET("/pricings/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetPricingByID)

	// -------- Prices --------
	admin.GET("/prices", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListPrices)
	admin.POST("/prices", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreatePrice)
	admin.GET("/prices/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetPriceByID)

	// -------- Price Amounts --------
	admin.GET("/price_amounts", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListPriceAmounts)
	admin.POST("/price_amounts", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreatePriceAmount)
	admin.GET("/price_amounts/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetPriceAmountByID)

	// -------- Tiers ---------
	admin.GET("/price_tiers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListPriceTiers)
	admin.POST("/price_tiers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreatePriceTier)
	admin.GET("/price_tiers/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetPriceTierByID)

	// -------- Subscriptions --------
	admin.GET("/subscriptions", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListSubscriptions)
	admin.POST("/subscriptions", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateSubscription)
	admin.GET("/subscriptions/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetSubscriptionByID)
	admin.POST("/subscriptions/:id/activate", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionActivate), s.ActivateSubscription)
	admin.POST("/subscriptions/:id/pause", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionPause), s.PauseSubscription)
	admin.POST("/subscriptions/:id/resume", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionResume), s.ResumeSubscription)
	admin.POST("/subscriptions/:id/cancel", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionCancel), s.CancelSubscription)

	// -------- Invoices --------
	admin.GET("/invoices", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListInvoices)
	admin.GET("/invoices/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetInvoiceByID)
	admin.GET("/invoices/:id/render", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.RenderInvoice)

	// -------- Billing Dashboard --------
	admin.GET("/billing/customers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectBillingDashboard, authorization.ActionBillingDashboardView), s.ListBillingCustomers)
	admin.GET("/billing/cycles", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectBillingDashboard, authorization.ActionBillingDashboardView), s.ListBillingCycles)
	admin.GET("/billing/activity", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectBillingDashboard, authorization.ActionBillingDashboardView), s.ListBillingActivity)
	admin.GET("/billing/overview/mrr", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewMRR)
	admin.GET("/billing/overview/mrr-movement", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewMRRMovement)
	admin.GET("/billing/overview/revenue", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewRevenue)
	admin.GET("/billing/overview/outstanding", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewOutstandingBalance)
	admin.GET("/billing/overview/collection-rate", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewCollectionRate)
	admin.GET("/billing/overview/subscribers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewSubscribers)
	admin.POST("/internal/rebuild-billing-snapshots", s.RequireRole(organizationdomain.RoleOwner), s.RebuildBillingSnapshots)

	// -------- Invoice Templates --------
	admin.GET("/invoice-templates", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListInvoiceTemplates)
	admin.POST("/invoice-templates", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateInvoiceTemplate)
	admin.GET("/invoice-templates/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetInvoiceTemplateByID)
	admin.PATCH("/invoice-templates/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.UpdateInvoiceTemplate)
	admin.POST("/invoice-templates/:id/set-default", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.SetDefaultInvoiceTemplate)

	// -------- Payment Providers --------
	admin.GET("/payment-providers/catalog", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectPaymentProvider, authorization.ActionPaymentProviderManage), s.ListPaymentProviderCatalog)
	admin.GET("/payment-providers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectPaymentProvider, authorization.ActionPaymentProviderManage), s.ListPaymentProviderConfigs)
	admin.POST("/payment-providers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectPaymentProvider, authorization.ActionPaymentProviderManage), s.UpsertPaymentProviderConfig)
	admin.PATCH("/payment-providers/:provider", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectPaymentProvider, authorization.ActionPaymentProviderManage), s.UpdatePaymentProviderStatus)

	// -------- Customers --------
	admin.GET("/customers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListCustomers)
	admin.POST("/customers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateCustomer)
	admin.GET("/customers/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetCustomerByID)

	admin.GET("/audit-logs", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectAuditLog, authorization.ActionAuditLogView), s.ListAuditLogs)
	admin.GET("/api-keys/scopes", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectAPIKey, authorization.ActionAPIKeyView), s.ListAPIKeyScopes)
	admin.GET("/api-keys", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectAPIKey, authorization.ActionAPIKeyView), s.ListAPIKeys)
	admin.POST("/api-keys", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectAPIKey, authorization.ActionAPIKeyCreate), s.CreateAPIKey)
	admin.POST("/api-keys/:key_id/reveal", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectAPIKey, authorization.ActionAPIKeyRotate), s.RevealAPIKey)
	admin.POST("/api-keys/:key_id/revoke", s.RequireRole(organizationdomain.RoleOwner), s.authorizeOrgAction(authorization.ObjectAPIKey, authorization.ActionAPIKeyRevoke), s.RevokeAPIKey)
}

func (s *Server) registerUIRoutes() {
	r := s.engine.Group("/")

	// ---- SPA entry points ----
	r.GET("/", serveIndex)
	r.GET("/login", s.redirectIfLoggedIn(), serveIndex)
	r.GET("/login/:name", s.OAuthLogin)
	r.GET("/invite/:code", serveIndex)
	r.GET("/change-password", s.WebAuthRequired(), serveIndex)

	orgs := r.Group("/orgs", s.WebAuthRequired())
	{
		orgs.GET("", serveIndex)
		org := orgs.Group("/:id")
		{
			products := org.Group("/products")
			{
				products.GET("", serveIndex)
			}

			customers := org.Group("/customers")
			{
				customers.GET("", serveIndex)
			}

			prices := org.Group("/prices")
			{
				prices.GET("", serveIndex)
			}

			subscriptions := org.Group("/subscriptions")
			{
				subscriptions.GET("", serveIndex)
			}

			invoices := org.Group("/invoices")
			{
				invoices.GET("", serveIndex)
			}

			apiKeys := org.Group("/api-keys")
			{
				apiKeys.GET("", serveIndex)
			}

			auditLogs := org.Group("/audit-logs")
			{
				auditLogs.GET("", serveIndex)
			}

			paymentProviders := org.Group("/payment-providers")
			{
				paymentProviders.GET("", serveIndex)
			}

			settings := org.Group("/settings", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin))
			{
				settings.GET("/", serveIndex)
			}
		}
	}
}

func (s *Server) registerFallback() {
	s.engine.NoRoute(func(c *gin.Context) {
		// static assets (vite)
		if fileExists("./public", c.Request.URL.Path) {
			c.File("./public" + c.Request.URL.Path)
			return
		}

		// SPA fallback
		c.File("./public/index.html")
	})
}

func fileExists(publicDir, reqPath string) bool {
	clean := filepath.Clean(reqPath)

	// prevent path traversal
	if clean == "." || clean == "/" || clean == ".." {
		return false
	}

	fullPath := filepath.Join(publicDir, clean)

	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}

	return !info.IsDir()
}
