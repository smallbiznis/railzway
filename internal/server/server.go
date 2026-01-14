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
	"github.com/smallbiznis/railzway/internal/apikey"
	apikeydomain "github.com/smallbiznis/railzway/internal/apikey/domain"
	"github.com/smallbiznis/railzway/internal/audit"
	auditdomain "github.com/smallbiznis/railzway/internal/audit/domain"
	"github.com/smallbiznis/railzway/internal/auth"
	authdomain "github.com/smallbiznis/railzway/internal/auth/domain"
	authlocal "github.com/smallbiznis/railzway/internal/auth/local"
	authoauth "github.com/smallbiznis/railzway/internal/auth/oauth"
	authoauth2provider "github.com/smallbiznis/railzway/internal/auth/oauth2provider"
	"github.com/smallbiznis/railzway/internal/auth/session"
	"github.com/smallbiznis/railzway/internal/authorization"
	"github.com/smallbiznis/railzway/internal/billingdashboard"
	billingdashboarddomain "github.com/smallbiznis/railzway/internal/billingdashboard/domain"
	billingrollup "github.com/smallbiznis/railzway/internal/billingdashboard/rollup"
	"github.com/smallbiznis/railzway/internal/billingoperations"
	billingoperationsdomain "github.com/smallbiznis/railzway/internal/billingoperations/domain"
	"github.com/smallbiznis/railzway/internal/billingoverview"
	billingoverviewdomain "github.com/smallbiznis/railzway/internal/billingoverview/domain"
	"github.com/smallbiznis/railzway/internal/cloudmetrics"
	"github.com/smallbiznis/railzway/internal/config"
	"github.com/smallbiznis/railzway/internal/customer"
	customerdomain "github.com/smallbiznis/railzway/internal/customer/domain"
	"github.com/smallbiznis/railzway/internal/events"
	"github.com/smallbiznis/railzway/internal/feature"
	featuredomain "github.com/smallbiznis/railzway/internal/feature/domain"
	"github.com/smallbiznis/railzway/internal/invoice"
	invoicedomain "github.com/smallbiznis/railzway/internal/invoice/domain"
	"github.com/smallbiznis/railzway/internal/invoicetemplate"
	invoicetemplatedomain "github.com/smallbiznis/railzway/internal/invoicetemplate/domain"
	"github.com/smallbiznis/railzway/internal/ledger"
	"github.com/smallbiznis/railzway/internal/meter"
	meterdomain "github.com/smallbiznis/railzway/internal/meter/domain"
	"github.com/smallbiznis/railzway/internal/observability"
	obsmiddleware "github.com/smallbiznis/railzway/internal/observability/logger"
	obsmetrics "github.com/smallbiznis/railzway/internal/observability/metrics"
	obstracing "github.com/smallbiznis/railzway/internal/observability/tracing"
	"github.com/smallbiznis/railzway/internal/organization"
	organizationdomain "github.com/smallbiznis/railzway/internal/organization/domain"
	"github.com/smallbiznis/railzway/internal/payment"
	paymentdomain "github.com/smallbiznis/railzway/internal/payment/domain"
	"github.com/smallbiznis/railzway/internal/price"
	pricedomain "github.com/smallbiznis/railzway/internal/price/domain"
	"github.com/smallbiznis/railzway/internal/priceamount"
	priceamountdomain "github.com/smallbiznis/railzway/internal/priceamount/domain"
	"github.com/smallbiznis/railzway/internal/pricetier"
	pricetierdomain "github.com/smallbiznis/railzway/internal/pricetier/domain"
	"github.com/smallbiznis/railzway/internal/product"
	productdomain "github.com/smallbiznis/railzway/internal/product/domain"
	"github.com/smallbiznis/railzway/internal/productfeature"
	productfeaturedomain "github.com/smallbiznis/railzway/internal/productfeature/domain"

	"github.com/smallbiznis/railzway/internal/providers/email"
	paymentprovider "github.com/smallbiznis/railzway/internal/providers/payment"
	paymentproviderdomain "github.com/smallbiznis/railzway/internal/providers/payment/domain"
	"github.com/smallbiznis/railzway/internal/providers/pdf"
	"github.com/smallbiznis/railzway/internal/publicinvoice"
	publicinvoicedomain "github.com/smallbiznis/railzway/internal/publicinvoice/domain"
	"github.com/smallbiznis/railzway/internal/ratelimit"
	"github.com/smallbiznis/railzway/internal/rating"
	ratingdomain "github.com/smallbiznis/railzway/internal/rating/domain"
	"github.com/smallbiznis/railzway/internal/reference"
	referencedomain "github.com/smallbiznis/railzway/internal/reference/domain"
	"github.com/smallbiznis/railzway/internal/scheduler"
	signupdomain "github.com/smallbiznis/railzway/internal/signup/domain"
	"github.com/smallbiznis/railzway/internal/subscription"
	subscriptiondomain "github.com/smallbiznis/railzway/internal/subscription/domain"
	taxdomain "github.com/smallbiznis/railzway/internal/tax/domain"
	"github.com/smallbiznis/railzway/internal/usage"
	usagedomain "github.com/smallbiznis/railzway/internal/usage/domain"
	"github.com/smallbiznis/railzway/internal/usage/liveevents"
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
	publicinvoice.Module,
	reference.Module,
	rating.Module,
	ratelimit.Module,
	subscription.Module,
	usage.Module,
	fx.Provide(NewServer),
	fx.Invoke(RegisterRoutes),
	fx.Invoke(RunHTTP),
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

func RegisterRoutes(s *Server) {
	s.RegisterAuthRoutes()
	s.RegisterAPIRoutes()
	s.RegisterAdminRoutes()
	s.RegisterUIRoutes()
	s.RegisterFallback()
}

func RunHTTP(lc fx.Lifecycle, r *gin.Engine, cfg config.Config) {
	port := cfg.Port
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:    ":" + port,
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
	billingOperationsSvc        billingoperationsdomain.Service
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
	productFeatureSvc           productfeaturedomain.Service
	featureSvc                  featuredomain.Service
	paymentSvc                  paymentdomain.Service
	paymentProviderSvc          paymentproviderdomain.Service
	invoiceTemplateSvc          invoicetemplatedomain.Service
	refrepo                     referencedomain.Repository
	signupsvc                   signupdomain.Service
	ratingSvc                   ratingdomain.Service
	subscriptionSvc             subscriptiondomain.Service
	usagesvc                    usagedomain.Service
	taxSvc                      taxdomain.Service
	liveMeterEvents             *liveevents.Hub
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

	Gin                  *gin.Engine
	Cfg                  config.Config
	DB                   *gorm.DB
	Authsvc              authdomain.Service              `optional:"true"`
	OAuthsvc             authoauth.Service               `optional:"true"`
	Sessions             *session.Manager                `optional:"true"`
	GenID                *snowflake.Node                 `optional:"true"`
	APIKeySvc            apikeydomain.Service            `optional:"true"`
	AuthzSvc             authorization.Service           `optional:"true"`
	AuditSvc             auditdomain.Service             `optional:"true"`
	BillingDashboardSvc  billingdashboarddomain.Service  `optional:"true"`
	BillingOperationsSvc billingoperationsdomain.Service `optional:"true"`
	BillingOverviewSvc   billingoverviewdomain.Service   `optional:"true"`
	BillingRollup        *billingrollup.Service          `optional:"true"`
	InvoiceSvc           invoicedomain.Service           `optional:"true"`
	MeterSvc             meterdomain.Service             `optional:"true"`
	OrganizationSvc      organizationdomain.Service      `optional:"true"`
	CustomerSvc          customerdomain.Service          `optional:"true"`
	PriceSvc             pricedomain.Service             `optional:"true"`
	PriceAmountSvc       priceamountdomain.Service       `optional:"true"`
	PriceTierSvc         pricetierdomain.Service         `optional:"true"`
	ProductSvc           productdomain.Service           `optional:"true"`
	ProductFeatureSvc    productfeaturedomain.Service    `optional:"true"`
	FeatureSvc           featuredomain.Service           `optional:"true"`
	PaymentSvc           paymentdomain.Service           `optional:"true"`
	PaymentProviderSvc   paymentproviderdomain.Service   `optional:"true"`
	InvoiceTemplateSvc   invoicetemplatedomain.Service   `optional:"true"`
	Refrepo              referencedomain.Repository      `optional:"true"`
	RatingSvc            ratingdomain.Service            `optional:"true"`
	SubscriptionSvc      subscriptiondomain.Service      `optional:"true"`
	Usagesvc             usagedomain.Service             `optional:"true"`
	TaxSvc               taxdomain.Service               `optional:"true"`
	LiveMeterEvents      *liveevents.Hub                 `optional:"true"`
	PublicInvoiceSvc     publicinvoicedomain.Service     `optional:"true"`
	ObsMetrics           *obsmetrics.Metrics             `optional:"true"`
	UsageLimiter         *ratelimit.UsageIngestLimiter   `optional:"true"`

	Scheduler *scheduler.Scheduler `optional:"true"`
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
		billingOperationsSvc:        p.BillingOperationsSvc,
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
		productFeatureSvc:           p.ProductFeatureSvc,
		featureSvc:                  p.FeatureSvc,
		paymentSvc:                  p.PaymentSvc,
		paymentProviderSvc:          p.PaymentProviderSvc,
		invoiceTemplateSvc:          p.InvoiceTemplateSvc,
		refrepo:                     p.Refrepo,
		ratingSvc:                   p.RatingSvc,
		subscriptionSvc:             p.SubscriptionSvc,
		usagesvc:                    p.Usagesvc,
		taxSvc:                      p.TaxSvc,
		liveMeterEvents:             p.LiveMeterEvents,
		obsMetrics:                  p.ObsMetrics,
		usageLimiter:                p.UsageLimiter,
		publicInvoiceSvc:            p.PublicInvoiceSvc,
		publicInvoiceLimiter:        newRateLimiter(30, time.Minute),
		publicPaymentIntentLimiter:  newRateLimiter(5, time.Minute),
		publicPaymentMethodsLimiter: newRateLimiter(30, time.Minute),
		publicPaymentMethodsCache:   newPaymentMethodsCache(2 * time.Minute),
		scheduler:                   p.Scheduler,
	}

	return svc
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) RegisterAuthRoutes() {
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
		user.POST("/orgs", s.CreateOrganization)
		user.POST("/orgs/:id/invites", s.InviteOrganizationMembers)
		user.POST("/orgs/:id/billing-preferences", s.SetOrganizationBillingPreferences)
	}

	// Public invite acceptance (requires auth but not org context)
	auth.POST("/invites/:invite_id/accept", s.WebAuthRequired(), s.AcceptOrganizationInvite)

	// Public invite info (no auth required)
	auth.GET("/invites/:invite_id", s.GetPublicInviteInfo)
	auth.POST("/invites/:invite_id/complete", s.CompleteInvite)

}

func (s *Server) RegisterAPIRoutes() {
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
	api.PUT("/subscriptions/:id/items", s.APIKeyRequired(), s.ReplaceSubscriptionItems)
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

func (s *Server) RegisterAdminRoutes() {
	admin := s.engine.Group("/admin")

	// --- global middlewares ---
	admin.Use(s.WebAuthRequired())
	admin.Use(s.OrgContext())

	// Home / Dashboard
	admin.GET("/home", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleMember, organizationdomain.RoleFinOps), s.GetHomeDashboard)

	admin.GET("/meters", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListMeters)
	admin.POST("/meters", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateMeter)
	admin.GET("/meters/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetMeterByID)
	admin.PATCH("/meters/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.UpdateMeter)
	admin.DELETE("/meters/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.DeleteMeter)

	admin.GET("/meters/:id/live-events",
		s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin),
		s.StreamMeterLiveEvents,
	)

	// -------- Product --------
	admin.GET("/products", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListProducts)
	admin.POST("/products", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateProduct)
	admin.GET("/products/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetProductByID)
	admin.PATCH("/products/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.UpdateProduct)
	admin.POST("/products/:id/archive", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ArchiveProduct)
	admin.GET("/products/:id/features", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListProductFeatures)
	admin.PUT("/products/:id/features", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ReplaceProductFeatures)

	// -------- Features --------
	admin.GET("/features", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListFeatures)
	admin.POST("/features", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateFeature)
	admin.PATCH("/features/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.UpdateFeature)
	admin.POST("/features/:id/archive", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ArchiveFeature)

	// -------- Tax Definitions --------
	admin.GET("/tax-definitions", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListTaxDefinitions)
	admin.POST("/tax-definitions", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateTaxDefinition)
	admin.PATCH("/tax-definitions/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.UpdateTaxDefinition)
	admin.POST("/tax-definitions/:id/disable", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.DisableTaxDefinition)

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
	admin.PUT("/subscriptions/:id/items", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ReplaceSubscriptionItems)
	admin.POST("/subscriptions/:id/activate", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionActivate), s.ActivateSubscription)
	admin.POST("/subscriptions/:id/pause", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionPause), s.PauseSubscription)
	admin.POST("/subscriptions/:id/resume", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionResume), s.ResumeSubscription)
	admin.POST("/subscriptions/:id/cancel", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectSubscription, authorization.ActionSubscriptionCancel), s.CancelSubscription)

	// -------- Invoices --------
	admin.GET("/invoices", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.ListInvoices)
	admin.GET("/invoices/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.GetInvoiceByID)
	admin.GET("/invoices/:id/render", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.RenderInvoice)

	// -------- Billing Dashboard --------
	admin.GET("/billing/customers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingDashboard, authorization.ActionBillingDashboardView), s.ListBillingCustomers)
	admin.GET("/billing/cycles", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingDashboard, authorization.ActionBillingDashboardView), s.ListBillingCycles)
	admin.GET("/billing/activity", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingDashboard, authorization.ActionBillingDashboardView), s.ListBillingActivity)
	admin.GET("/billing/operations", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOperations, authorization.ActionBillingOperationsView), s.GetBillingOperations)
	admin.POST("/billing/operations/actions", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOperations, authorization.ActionBillingOperationsAct), s.PostBillingOperationsAction)
	admin.POST("/billing/operations/assignments", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOperations, authorization.ActionBillingOperationsAct), s.PostBillingOperationsAssignment)
	admin.DELETE("/billing/operations/assignments", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOperations, authorization.ActionBillingOperationsAct), s.ReleaseBillingOperationsAssignment)
	admin.GET("/billing/operations/overdue-invoices", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOperations, authorization.ActionBillingOperationsView), s.GetBillingOperationsOverdueInvoices)
	admin.GET("/billing/operations/outstanding-customers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOperations, authorization.ActionBillingOperationsView), s.GetBillingOperationsOutstandingCustomers)
	admin.GET("/billing/operations/payment-issues", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOperations, authorization.ActionBillingOperationsView), s.GetBillingOperationsPaymentIssues)
	admin.GET("/billing/overview/mrr", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewMRR)
	admin.GET("/billing/overview/mrr-movement", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewMRRMovement)
	admin.GET("/billing/overview/revenue", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewRevenue)
	admin.GET("/billing/overview/outstanding", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewOutstandingBalance)
	admin.GET("/billing/overview/collection-rate", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewCollectionRate)
	admin.GET("/billing/overview/subscribers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.authorizeOrgAction(authorization.ObjectBillingOverview, authorization.ActionBillingOverviewView), s.GetBillingOverviewSubscribers)

	// -------- FinOps Performance --------
	admin.GET("/finops/performance/me", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleMember, organizationdomain.RoleFinOps), s.GetBillingOperationsPerformanceMe)
	admin.GET("/finops/performance/team", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.GetBillingOperationsPerformanceTeam)
	admin.GET("/finops/exposure-analysis", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetExposureAnalysis)

	// -------- Billing Operations IA (Task-Centric Views) --------
	admin.GET("/billing-operations/inbox", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleMember, organizationdomain.RoleFinOps), s.GetBillingOperationsInbox)
	admin.GET("/billing-operations/my-work", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleMember, organizationdomain.RoleFinOps), s.GetBillingOperationsMyWork)
	admin.GET("/billing-operations/recently-resolved", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleMember, organizationdomain.RoleFinOps), s.GetBillingOperationsRecentlyResolved)
	admin.GET("/billing-operations/team", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.GetBillingOperationsTeamView)

	// -------- Billing Operations Actions --------
	admin.POST("/billing-operations/claim", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleMember, organizationdomain.RoleFinOps), s.PostBillingOperationsAssignment)
	admin.POST("/billing-operations/release", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleMember, organizationdomain.RoleFinOps), s.ReleaseBillingOperationsAssignment)
	admin.POST("/billing-operations/resolve", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleMember, organizationdomain.RoleFinOps), s.ResolveBillingOperationsAssignment)

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
	admin.GET("/customers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.ListCustomers)
	admin.POST("/customers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateCustomer)
	admin.GET("/customers/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin, organizationdomain.RoleFinOps), s.GetCustomerByID)

	admin.GET("/audit-logs", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectAuditLog, authorization.ActionAuditLogView), s.ListAuditLogs)
	admin.GET("/api-keys/scopes", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectAPIKey, authorization.ActionAPIKeyView), s.ListAPIKeyScopes)
	admin.GET("/api-keys", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectAPIKey, authorization.ActionAPIKeyView), s.ListAPIKeys)
	admin.POST("/api-keys", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectAPIKey, authorization.ActionAPIKeyCreate), s.CreateAPIKey)
	admin.POST("/api-keys/:key_id/reveal", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.authorizeOrgAction(authorization.ObjectAPIKey, authorization.ActionAPIKeyRotate), s.RevealAPIKey)
	admin.POST("/api-keys/:key_id/revoke", s.RequireRole(organizationdomain.RoleOwner), s.authorizeOrgAction(authorization.ObjectAPIKey, authorization.ActionAPIKeyRevoke), s.RevokeAPIKey)
}

func (s *Server) RegisterUIRoutes() {
	r := s.engine.Group("/")

	// ---- SPA entry points ----
	r.GET("/", s.serveIndex)
	r.GET("/login", s.redirectIfLoggedIn(), s.serveIndex)
	r.GET("/login/:name", s.OAuthLogin)
	r.GET("/invite/:code", s.serveIndex)
	r.GET("/change-password", s.WebAuthRequired(), s.serveIndex)

	orgs := r.Group("/orgs", s.WebAuthRequired())
	{
		orgs.GET("", s.serveIndex)
		org := orgs.Group("/:id")
		{
			home := org.Group("/home")
			{
				home.GET("", s.serveIndex)
			}
			products := org.Group("/products")
			{
				products.GET("", s.serveIndex)
				features := products.Group("/features")
				{
					features.GET("", s.serveIndex)
				}
			}

			taxdefinition := org.Group("/tax-definitions")
			{
				taxdefinition.GET("", s.serveIndex)
			}

			customers := org.Group("/customers")
			{
				customers.GET("", s.serveIndex)
			}

			prices := org.Group("/prices")
			{
				prices.GET("", s.serveIndex)
			}

			subscriptions := org.Group("/subscriptions")
			{
				subscriptions.GET("", s.serveIndex)
			}

			invoices := org.Group("/invoices")
			{
				invoices.GET("", s.serveIndex)
			}

			apiKeys := org.Group("/api-keys")
			{
				apiKeys.GET("", s.serveIndex)
			}

			auditLogs := org.Group("/audit-logs")
			{
				auditLogs.GET("", s.serveIndex)
			}

			paymentProviders := org.Group("/payment-providers")
			{
				paymentProviders.GET("", s.serveIndex)
			}

			settings := org.Group("/settings", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin))
			{
				settings.GET("/", s.serveIndex)
			}
		}
	}
}

func (s *Server) RegisterFallback() {
	staticDir := s.cfg.StaticDir
	s.engine.NoRoute(func(c *gin.Context) {
		// static assets (vite)
		if fileExists(staticDir, c.Request.URL.Path) {
			c.File(filepath.Join(staticDir, c.Request.URL.Path))
			return
		}

		// SPA fallback
		c.File(filepath.Join(staticDir, "index.html"))
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
