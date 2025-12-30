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
	// r.Use(gin.Recovery())
	r.Use(gin.Logger())
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
	svc.RegisterDevBillingRoutes()

	return svc
}

func (s *Server) Engine() *gin.Engine {
	return s.engine
}

func (s *Server) registerAuthRoutes() {
	auth := s.engine.Group("/auth")
	auth.Use(RequestID())

	auth.POST("/login", s.Login)

	auth.POST("/logout", s.Logout)
	auth.POST("/change-password", s.AuthRequired(), s.ChangePassword)
	auth.POST("/forgot", s.Forgot)
	auth.GET("/me", s.Me)

	user := s.engine.Group("/user", s.AuthRequired())
	{
		user.GET("/orgs", s.ListUserOrgs)
		user.POST("/using/:id", s.UseOrg)
	}
}

func (s *Server) registerAPIRoutes() {
	api := s.engine.Group("/api")

	api.GET("/countries", s.ListCountries)
	api.GET("/timezones", s.ListTimezones)
	api.GET("/currencies", s.ListCurrencies)

	hooks := s.engine.Group("/hooks")
	hooks.Use(RequestID())
	hooks.Use(s.APIKeyRequired())
	hooks.POST("/usage", s.IngestUsage)

	secured := api.Group("")

	// --- global middlewares ---
	secured.Use(RequestID())
	secured.Use(s.AuthRequired())

	user := secured.Group("/user")
	{
		user.GET("/using/:id", s.UseOrg)
		user.GET("/orgs", s.ListUserOrgs)
	}

	secured.Use(s.OrgContext())

	// -------- Meters --------
	secured.GET("/meters", s.ListMeters)
	secured.POST("/meters", s.CreateMeter)
	secured.GET("/meters/:id", s.GetMeterByID)
	secured.PATCH("/meters/:id", s.UpdateMeter)
	secured.DELETE("/meters/:id", s.DeleteMeter)

	// -------- Product --------
	secured.GET("/products", s.ListProducts)
	secured.POST("/products", s.CreateProduct)
	secured.GET("/products/:id", s.GetProductByID)

	// -------- Pricing --------
	secured.GET("/pricings", s.ListPricings)
	secured.POST("/pricings", s.CreatePricing)
	secured.GET("/pricings/:id", s.GetPricingByID)

	// -------- Prices --------
	secured.GET("/prices", s.ListPrices)
	secured.POST("/prices", s.CreatePrice)
	secured.GET("/prices/:id", s.GetPriceByID)

	// -------- Price Amounts --------
	secured.GET("/price_amounts", s.ListPriceAmounts)
	secured.POST("/price_amounts", s.CreatePriceAmount)
	secured.GET("/price_amounts/:id", s.GetPriceAmountByID)

	// -------- Tiers ---------
	secured.GET("/price_tiers", s.ListPriceTiers)
	secured.POST("/price_tiers", s.CreatePriceTier)
	secured.GET("/price_tiers/:id", s.GetPriceTierByID)

	// -------- Subscriptions --------
	secured.GET("/subscriptions", s.ListSubscriptions)
	secured.POST("/subscriptions", s.CreateSubscription)
	secured.GET("/subscriptions/:id", s.GetSubscriptionByID)
	secured.POST("/subscriptions/:id/activate", s.ActivateSubscription)
	secured.POST("/subscriptions/:id/pause", s.PauseSubscription)
	secured.POST("/subscriptions/:id/resume", s.ResumeSubscription)
	secured.POST("/subscriptions/:id/cancel", s.CancelSubscription)

	// -------- Rating --------
	secured.POST("/rating/run", s.RunRatingJob)

	// -------- Invoices --------
	secured.GET("/invoices", s.ListInvoices)
	secured.GET("/invoices/:id", s.GetInvoiceByID)

	// -------- Customers --------
	secured.GET("/customers", s.ListCustomers)
	secured.POST("/customers", s.CreateCustomer)
	secured.GET("/customers/:id", s.GetCustomerByID)

	if s.cfg.Environment != "production" {
		secured.POST("/test/cleanup", s.TestCleanup)
	}
}

func (s *Server) registerUIRoutes() {
	r := s.engine.Group("/")

	// --- middlewares ---
	r.Use(RequestID())

	// ---- SPA entry points ----
	r.GET("/", serveIndex)
	r.GET("/login", s.redirectIfLoggedIn(), serveIndex)
	r.GET("/login/:name", s.OAuthLogin)
	r.GET("/invite/:code", serveIndex)
	r.GET("/change-password", s.AuthRequired(), serveIndex)

	orgs := r.Group("/orgs", s.AuthRequired())
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
