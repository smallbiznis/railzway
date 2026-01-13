package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bwmarrin/snowflake"
	"github.com/gin-gonic/gin"
	authdomain "github.com/smallbiznis/railzway/internal/auth/domain"
	"github.com/smallbiznis/railzway/internal/config"
	orgdomain "github.com/smallbiznis/railzway/internal/organization/domain"
	"github.com/smallbiznis/railzway/internal/signup"
	signupdomain "github.com/smallbiznis/railzway/internal/signup/domain"
)

type fakeSignupService struct {
	called bool
}

func (f *fakeSignupService) Signup(ctx context.Context, req signupdomain.Request) (*signupdomain.Result, error) {
	f.called = true
	_ = ctx
	_ = req
	return &signupdomain.Result{}, nil
}

type fakeProvisioner struct {
	calls              int
	lastOrganizationID string
}

func (f *fakeProvisioner) Provision(ctx context.Context, organizationID string) error {
	f.calls++
	f.lastOrganizationID = organizationID
	_ = ctx
	return nil
}

type fakeAuthService struct {
	createUserCalls int
	loginCalls      int
}

func (f *fakeAuthService) CreateUser(ctx context.Context, req authdomain.CreateUserRequest) (*authdomain.User, error) {
	f.createUserCalls++
	_ = ctx
	return &authdomain.User{
		ID:    snowflake.ID(200),
		Email: req.Email,
	}, nil
}

func (f *fakeAuthService) Login(ctx context.Context, req authdomain.LoginRequest) (*authdomain.LoginResult, error) {
	f.loginCalls++
	_ = ctx
	_ = req
	return &authdomain.LoginResult{
		Session: &authdomain.SessionView{
			Metadata: map[string]any{
				"user_id": "200",
			},
		},
		RawToken:  "session-token",
		SessionID: snowflake.ID(300),
	}, nil
}

func (f *fakeAuthService) Logout(ctx context.Context, rawToken string) error {
	_ = ctx
	_ = rawToken
	return nil
}

func (f *fakeAuthService) Authenticate(ctx context.Context, rawToken string) (*authdomain.Session, error) {
	_ = ctx
	_ = rawToken
	return nil, nil
}

func (f *fakeAuthService) UpdateSessionOrgContext(ctx context.Context, sessionID snowflake.ID, activeOrgID *int64, orgIDs []int64) error {
	_ = ctx
	_ = sessionID
	_ = activeOrgID
	_ = orgIDs
	return nil
}

func (f *fakeAuthService) ChangePassword(ctx context.Context, userID string, newPassword string) error {
	_ = ctx
	_ = userID
	_ = newPassword
	return nil
}

func (f *fakeAuthService) CurrentUser(ctx context.Context) (*authdomain.User, error) {
	_ = ctx
	return nil, nil
}

type fakeOrgService struct {
	org            *orgdomain.OrganizationResponse
	createOrgCalls int
	lastOrgName    string
	lastUserID     snowflake.ID
}

// AcceptInvite implements domain.Service.
func (f *fakeOrgService) AcceptInvite(ctx context.Context, userID snowflake.ID, inviteID string) error {
	panic("unimplemented")
}

func newFakeOrgService() *fakeOrgService {
	return &fakeOrgService{
		org: &orgdomain.OrganizationResponse{
			ID:           snowflake.ID(100).String(),
			Name:         "default",
			CountryCode:  "ID",
			TimezoneName: "Asia/Jakarta",
		},
	}
}

func (f *fakeOrgService) ListOrganizationsByUser(ctx context.Context, userID snowflake.ID) ([]orgdomain.OrganizationListResponseItem, error) {
	_ = ctx
	_ = userID
	return nil, nil
}

func (f *fakeOrgService) Create(ctx context.Context, userID snowflake.ID, req orgdomain.CreateOrganizationRequest) (*orgdomain.OrganizationResponse, error) {
	f.createOrgCalls++
	f.lastUserID = userID
	f.lastOrgName = req.Name
	f.org.Name = req.Name
	_ = ctx
	return f.org, nil
}

func (f *fakeOrgService) GetByID(ctx context.Context, id string) (*orgdomain.OrganizationResponse, error) {
	_ = ctx
	_ = id
	return f.org, nil
}

func (f *fakeOrgService) InviteMembers(ctx context.Context, userID snowflake.ID, orgID string, invites []orgdomain.InviteRequest) error {
	_ = ctx
	_ = userID
	_ = orgID
	_ = invites
	return nil
}

func (f *fakeOrgService) SetBillingPreferences(ctx context.Context, userID snowflake.ID, orgID string, req orgdomain.BillingPreferencesRequest) error {
	_ = ctx
	_ = userID
	_ = orgID
	_ = req
	return nil
}

func TestSignupHandlerOSSModeReturns404(t *testing.T) {
	gin.SetMode(gin.TestMode)

	signupSvc := &fakeSignupService{}
	srv := &Server{
		cfg:       config.Config{Mode: config.ModeOSS},
		signupsvc: signupSvc,
	}

	router := gin.New()
	router.Use(ErrorHandlingMiddleware())
	router.POST("/auth/signup", srv.Signup)

	req := httptest.NewRequest(http.MethodPost, "/auth/signup", bytes.NewBufferString(`{"org_name":"Acme","username":"alice","password":"secret"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", resp.Code)
	}
	if signupSvc.called {
		t.Fatal("expected signup service not to be called in OSS mode")
	}
}

func TestSignupHandlerCloudModeDelegatesProvisioner(t *testing.T) {
	gin.SetMode(gin.TestMode)

	authSvc := &fakeAuthService{}
	orgSvc := newFakeOrgService()
	provisioner := &fakeProvisioner{}
	signupSvc := signup.NewService(authSvc, orgSvc, provisioner)

	srv := &Server{
		cfg:       config.Config{Mode: config.ModeCloud},
		signupsvc: signupSvc,
	}

	router := gin.New()
	router.Use(ErrorHandlingMiddleware())
	router.POST("/auth/signup", srv.Signup)

	req := httptest.NewRequest(http.MethodPost, "/auth/signup", bytes.NewBufferString(`{"org_name":"Acme","username":"alice","password":"secret","email":"a@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK && resp.Code != http.StatusCreated {
		t.Fatalf("expected status 200 or 201, got %d", resp.Code)
	}
	if provisioner.calls != 1 {
		t.Fatalf("expected provisioner to be called once, got %d", provisioner.calls)
	}
	if provisioner.lastOrganizationID != orgSvc.org.ID {
		t.Fatalf("expected provisioner organization ID %s, got %s", orgSvc.org.ID, provisioner.lastOrganizationID)
	}
}
