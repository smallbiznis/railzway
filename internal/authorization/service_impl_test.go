package authorization

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAuthorizeAllowsAdmin(t *testing.T) {
	db := setupAuthzTestDB(t)
	insertMember(t, db, 1, 10, "ADMIN")

	enforcer, err := NewEnforcer(db)
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	svc := &ServiceImpl{
		db:       db,
		log:      zap.NewNop(),
		enforcer: enforcer,
	}

	if err := svc.Authorize(context.Background(), "user:10", "1", ObjectSubscription, ActionSubscriptionActivate); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func TestAuthorizeDeniesMemberCapability(t *testing.T) {
	db := setupAuthzTestDB(t)
	insertMember(t, db, 1, 11, "MEMBER")

	enforcer, err := NewEnforcer(db)
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	svc := &ServiceImpl{
		db:       db,
		log:      zap.NewNop(),
		enforcer: enforcer,
	}

	err = svc.Authorize(context.Background(), "user:11", "1", ObjectSubscription, ActionSubscriptionCancel)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAuthorizeDeniesCrossOrg(t *testing.T) {
	db := setupAuthzTestDB(t)
	insertMember(t, db, 1, 12, "ADMIN")

	enforcer, err := NewEnforcer(db)
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	svc := &ServiceImpl{
		db:       db,
		log:      zap.NewNop(),
		enforcer: enforcer,
	}

	err = svc.Authorize(context.Background(), "user:12", "2", ObjectInvoice, ActionInvoiceFinalize)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAuthorizeSystem(t *testing.T) {
	db := setupAuthzTestDB(t)

	enforcer, err := NewEnforcer(db)
	if err != nil {
		t.Fatalf("new enforcer: %v", err)
	}
	svc := &ServiceImpl{
		db:       db,
		log:      zap.NewNop(),
		enforcer: enforcer,
	}

	if err := svc.Authorize(context.Background(), "system", "3", ObjectBillingCycle, ActionBillingCycleClose); err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
}

func setupAuthzTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(
		`CREATE TABLE IF NOT EXISTS organization_members (
			id INTEGER PRIMARY KEY,
			org_id BIGINT NOT NULL,
			user_id BIGINT NOT NULL,
			role TEXT NOT NULL
		)`,
	).Error; err != nil {
		t.Fatalf("create organization_members: %v", err)
	}
	if err := db.Exec(
		`CREATE TABLE IF NOT EXISTS casbin_rule (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ptype VARCHAR(100) NOT NULL,
			v0 VARCHAR(100),
			v1 VARCHAR(100),
			v2 VARCHAR(100),
			v3 VARCHAR(100),
			v4 VARCHAR(100),
			v5 VARCHAR(100)
		)`,
	).Error; err != nil {
		t.Fatalf("create casbin_rule: %v", err)
	}
	return db
}

func insertMember(t *testing.T, db *gorm.DB, orgID, userID int64, role string) {
	t.Helper()
	if err := db.Exec(
		`INSERT INTO organization_members (id, org_id, user_id, role)
		 VALUES (?, ?, ?, ?)`,
		userID,
		orgID,
		userID,
		role,
	).Error; err != nil {
		t.Fatalf("insert member: %v", err)
	}
}
