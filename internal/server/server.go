package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
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
	"github.com/smallbiznis/valora/internal/cloudmetrics"
	"github.com/smallbiznis/valora/internal/config"
	"github.com/smallbiznis/valora/internal/customer"
	customerdomain "github.com/smallbiznis/valora/internal/customer/domain"
	"github.com/smallbiznis/valora/internal/invoice"
	invoicedomain "github.com/smallbiznis/valora/internal/invoice/domain"
	"github.com/smallbiznis/valora/internal/meter"
	meterdomain "github.com/smallbiznis/valora/internal/meter/domain"
	"github.com/smallbiznis/valora/internal/organization"
	organizationdomain "github.com/smallbiznis/valora/internal/organization/domain"
	"github.com/smallbiznis/valora/internal/price"
	pricedomain "github.com/smallbiznis/valora/internal/price/domain"
	"github.com/smallbiznis/valora/internal/priceamount"
	priceamountdomain "github.com/smallbiznis/valora/internal/priceamount/domain"
	"github.com/smallbiznis/valora/internal/pricetier"
	pricetierdomain "github.com/smallbiznis/valora/internal/pricetier/domain"
	"github.com/smallbiznis/valora/internal/product"
	productdomain "github.com/smallbiznis/valora/internal/product/domain"
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
	auth.Module,
	authlocal.Module,
	authoauth2provider.Module,
	session.Module,
	apikey.Module,
	customer.Module,
	invoice.Module,
	meter.Module,
	organization.Module,
	price.Module,
	priceamount.Module,
	pricetier.Module,
	product.Module,
	reference.Module,
	rating.Module,
	subscription.Module,
	usage.Module,
	fx.Invoke(NewServer),
	fx.Invoke(run),
)

func NewEngine() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(RequestID())
	r.Use(ErrorHandlingMiddleware())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return r
}

func registerGin() *gin.Engine {
	return NewEngine()
}

func run(lc fx.Lifecycle, r *gin.Engine) {
	lc.Append(
		fx.Hook{
			OnStart: func(ctx context.Context) error {
				go func() {
					if err := r.Run(":8080"); err != nil {
						panic(err)
					}
				}()
				return nil
			},
		},
	)
}

type Server struct {
	engine          *gin.Engine
	cfg             config.Config
	db              *gorm.DB
	authsvc         authdomain.Service
	oauthsvc        authoauth.Service
	sessions        *session.Manager
	genID           *snowflake.Node
	apiKeySvc       apikeydomain.Service
	apiKeyLimiter   *rateLimiter
	authzSvc        authorization.Service
	auditSvc        auditdomain.Service
	invoiceSvc      invoicedomain.Service
	meterSvc        meterdomain.Service
	organizationSvc organizationdomain.Service
	customerSvc     customerdomain.Service
	priceSvc        pricedomain.Service
	priceAmountSvc  priceamountdomain.Service
	priceTierSvc    pricetierdomain.Service
	productSvc      productdomain.Service
	refrepo         referencedomain.Repository
	signupsvc       signupdomain.Service
	ratingSvc       ratingdomain.Service
	subscriptionSvc subscriptiondomain.Service
	usagesvc        usagedomain.Service

	scheduler *scheduler.Scheduler
}

type ServerParams struct {
	fx.In

	Gin             *gin.Engine
	Cfg             config.Config
	DB              *gorm.DB
	Authsvc         authdomain.Service
	OAuthsvc        authoauth.Service
	Sessions        *session.Manager
	GenID           *snowflake.Node
	APIKeySvc       apikeydomain.Service
	AuthzSvc        authorization.Service
	AuditSvc        auditdomain.Service
	InvoiceSvc      invoicedomain.Service
	MeterSvc        meterdomain.Service
	OrganizationSvc organizationdomain.Service
	CustomerSvc     customerdomain.Service
	PriceSvc        pricedomain.Service
	PriceAmountSvc  priceamountdomain.Service
	PriceTierSvc    pricetierdomain.Service
	ProductSvc      productdomain.Service
	Refrepo         referencedomain.Repository
	RatingSvc       ratingdomain.Service
	SubscriptionSvc subscriptiondomain.Service
	Usagesvc        usagedomain.Service

	Scheduler *scheduler.Scheduler
}

func NewServer(p ServerParams) *Server {
	svc := &Server{
		engine:          p.Gin,
		cfg:             p.Cfg,
		db:              p.DB,
		authsvc:         p.Authsvc,
		oauthsvc:        p.OAuthsvc,
		sessions:        p.Sessions,
		genID:           p.GenID,
		apiKeySvc:       p.APIKeySvc,
		apiKeyLimiter:   newRateLimiter(5, 10*time.Minute),
		authzSvc:        p.AuthzSvc,
		auditSvc:        p.AuditSvc,
		invoiceSvc:      p.InvoiceSvc,
		meterSvc:        p.MeterSvc,
		organizationSvc: p.OrganizationSvc,
		customerSvc:     p.CustomerSvc,
		priceSvc:        p.PriceSvc,
		priceAmountSvc:  p.PriceAmountSvc,
		priceTierSvc:    p.PriceTierSvc,
		productSvc:      p.ProductSvc,
		refrepo:         p.Refrepo,
		ratingSvc:       p.RatingSvc,
		subscriptionSvc: p.SubscriptionSvc,
		usagesvc:        p.Usagesvc,
		scheduler:       p.Scheduler,
	}

	svc.registerAuthRoutes()
	svc.registerAPIRoutes()
	svc.registerAdminRoutes()
	svc.registerUIRoutes()
	svc.registerFallback()

	if svc.cfg.Environment != "production" {
		svc.RegisterDevBillingRoutes()
	}

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
	api.POST("/meters", s.APIKeyRequired(), s.CreateMeter)
	api.GET("/meters/:id", s.APIKeyRequired(), s.GetMeterByID)
	api.PATCH("/meters/:id", s.APIKeyRequired(), s.UpdateMeter)
	api.DELETE("/meters/:id", s.APIKeyRequired(), s.DeleteMeter)
	// -------- Product --------
	api.GET("/products", s.APIKeyRequired(), s.ListProducts)
	api.POST("/products", s.APIKeyRequired(), s.CreateProduct)
	api.GET("/products/:id", s.APIKeyRequired(), s.GetProductByID)

	// -------- Pricing --------
	api.GET("/pricings", s.APIKeyRequired(), s.ListPricings)
	api.POST("/pricings", s.APIKeyRequired(), s.CreatePricing)
	api.GET("/pricings/:id", s.APIKeyRequired(), s.GetPricingByID)

	// -------- Prices --------
	api.GET("/prices", s.APIKeyRequired(), s.ListPrices)
	api.POST("/prices", s.APIKeyRequired(), s.CreatePrice)
	api.GET("/prices/:id", s.APIKeyRequired(), s.GetPriceByID)

	// -------- Price Amounts --------
	api.GET("/price_amounts", s.APIKeyRequired(), s.ListPriceAmounts)
	api.POST("/price_amounts", s.APIKeyRequired(), s.CreatePriceAmount)
	api.GET("/price_amounts/:id", s.APIKeyRequired(), s.GetPriceAmountByID)

	// -------- Tiers ---------
	api.GET("/price_tiers", s.APIKeyRequired(), s.ListPriceTiers)
	api.POST("/price_tiers", s.APIKeyRequired(), s.CreatePriceTier)
	api.GET("/price_tiers/:id", s.APIKeyRequired(), s.GetPriceTierByID)

	// -------- Subscriptions --------
	api.GET("/subscriptions", s.APIKeyRequired(), s.ListSubscriptions)
	api.POST("/subscriptions", s.APIKeyRequired(), s.CreateSubscription)
	api.GET("/subscriptions/:id", s.APIKeyRequired(), s.GetSubscriptionByID)
	api.POST("/subscriptions/:id/activate", s.APIKeyRequired(), s.ActivateSubscription)
	api.POST("/subscriptions/:id/pause", s.APIKeyRequired(), s.PauseSubscription)
	api.POST("/subscriptions/:id/resume", s.APIKeyRequired(), s.ResumeSubscription)
	api.POST("/subscriptions/:id/cancel", s.APIKeyRequired(), s.CancelSubscription)
	// -------- Invoices --------
	api.GET("/invoices", s.APIKeyRequired(), s.ListInvoices)
	api.GET("/invoices/:id", s.APIKeyRequired(), s.GetInvoiceByID)

	// -------- Customers --------
	api.GET("/customers", s.APIKeyRequired(), s.ListCustomers)
	api.POST("/customers", s.APIKeyRequired(), s.CreateCustomer)
	api.GET("/customers/:id", s.APIKeyRequired(), s.GetCustomerByID)

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
	admin.POST("/subscriptions/:id/activate", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ActivateSubscription)
	admin.POST("/subscriptions/:id/pause", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.PauseSubscription)
	admin.POST("/subscriptions/:id/resume", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ResumeSubscription)
	admin.POST("/subscriptions/:id/cancel", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CancelSubscription)

	// -------- Invoices --------
	admin.GET("/invoices", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListInvoices)
	admin.GET("/invoices/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetInvoiceByID)

	// -------- Customers --------
	admin.GET("/customers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListCustomers)
	admin.POST("/customers", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateCustomer)
	admin.GET("/customers/:id", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.GetCustomerByID)

	admin.GET("/audit-logs", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListAuditLogs)
	admin.GET("/api-keys", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.ListAPIKeys)
	admin.POST("/api-keys", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.CreateAPIKey)
	admin.POST("/api-keys/:key_id/reveal", s.RequireRole(organizationdomain.RoleOwner, organizationdomain.RoleAdmin), s.RevealAPIKey)
	admin.POST("/api-keys/:key_id/revoke", s.RequireRole(organizationdomain.RoleOwner), s.RevokeAPIKey)
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
