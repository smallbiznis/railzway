package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/gosimple/slug"
	"github.com/smallbiznis/railzway/internal/organization/domain"
	"github.com/smallbiznis/railzway/internal/organization/event"
	"github.com/smallbiznis/railzway/internal/providers/email"
	referencedomain "github.com/smallbiznis/railzway/internal/reference/domain"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type service struct {
	db        *gorm.DB
	repo      domain.Repository
	ref       referencedomain.Repository
	genID     *snowflake.Node
	publisher event.EventPublisher
	email     email.Provider
}

func NewService(db *gorm.DB, repo domain.Repository, ref referencedomain.Repository, genID *snowflake.Node, publisher event.EventPublisher, email email.Provider) domain.Service {
	return &service{
		db:        db,
		repo:      repo,
		ref:       ref,
		genID:     genID,
		publisher: publisher,
		email:     email,
	}
}

func (s *service) Create(ctx context.Context, userID snowflake.ID, req domain.CreateOrganizationRequest) (*domain.OrganizationResponse, error) {
	if userID == 0 {
		return nil, domain.ErrInvalidUser
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, domain.ErrInvalidName
	}

	countryCode := strings.TrimSpace(req.CountryCode)
	if countryCode == "" {
		return nil, domain.ErrInvalidCountry
	}

	timezoneName := strings.TrimSpace(req.TimezoneName)
	if timezoneName == "" {
		return nil, domain.ErrInvalidTimezone
	}

	countryOK, err := s.countryExists(ctx, countryCode)
	if err != nil {
		return nil, err
	}
	if !countryOK {
		return nil, domain.ErrInvalidCountry
	}

	timezoneOK, err := s.timezoneAllowed(ctx, countryCode, timezoneName)
	if err != nil {
		return nil, err
	}
	if !timezoneOK {
		return nil, domain.ErrInvalidTimezone
	}

	now := time.Now().UTC()
	orgID := s.genID.Generate()
	org := domain.Organization{
		ID:           orgID,
		Name:         name,
		Slug:         slug.Make(name),
		CountryCode:  countryCode,
		TimezoneName: timezoneName,
		CreatedAt:    now,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)
		if err := repo.CreateOrganization(ctx, org); err != nil {
			return err
		}

		member := domain.OrganizationMember{
			ID:        s.genID.Generate(),
			OrgID:     orgID,
			UserID:    userID,
			Role:      domain.RoleOwner,
			CreatedAt: now,
		}

		if err := repo.AddMember(ctx, member); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	s.emitOrganizationCreated(ctx, org, userID)

	return &domain.OrganizationResponse{
		ID:           orgID.String(),
		Name:         name,
		Slug:         org.Slug,
		CountryCode:  countryCode,
		TimezoneName: timezoneName,
	}, nil
}

func (s *service) ListOrganizationsByUser(ctx context.Context, userID snowflake.ID) ([]domain.OrganizationListResponseItem, error) {
	if userID == 0 {
		return nil, domain.ErrInvalidUser
	}

	items, err := s.repo.ListOrganizationsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	resp := make([]domain.OrganizationListResponseItem, 0, len(items))
	for _, item := range items {
		resp = append(resp, domain.OrganizationListResponseItem{
			ID:        item.ID.String(),
			Name:      item.Name,
			Role:      item.Role,
			CreatedAt: item.CreatedAt,
		})
	}

	return resp, nil
}

func (s *service) GetByID(ctx context.Context, id string) (*domain.OrganizationResponse, error) {
	raw := strings.TrimSpace(id)
	if raw == "" {
		return nil, domain.ErrInvalidOrganization
	}
	orgID, err := snowflake.ParseString(raw)
	if err != nil {
		return nil, domain.ErrInvalidOrganization
	}

	var org domain.Organization
	if err := s.db.WithContext(ctx).First(&org, "id = ?", orgID).Error; err != nil {
		return nil, err
	}

	return &domain.OrganizationResponse{
		ID:           org.ID.String(),
		Name:         org.Name,
		Slug:         org.Slug,
		CountryCode:  org.CountryCode,
		TimezoneName: org.TimezoneName,
	}, nil
}

func (s *service) InviteMembers(ctx context.Context, userID snowflake.ID, orgID string, invites []domain.InviteRequest) error {
	if userID == 0 {
		return domain.ErrInvalidUser
	}
	if len(invites) == 0 {
		return nil
	}

	rawOrgID := strings.TrimSpace(orgID)
	if rawOrgID == "" {
		return domain.ErrInvalidOrganization
	}
	parsedOrgID, err := snowflake.ParseString(rawOrgID)
	if err != nil {
		return domain.ErrInvalidOrganization
	}

	org, err := s.getOrganization(ctx, parsedOrgID)
	if err != nil {
		return err
	}

	isMember, err := s.repo.IsMember(ctx, parsedOrgID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return domain.ErrForbidden
	}

	now := time.Now().UTC()
	rows := make([]domain.OrganizationInvite, 0, len(invites))
	for _, invite := range invites {
		email, err := normalizeEmail(invite.Email)
		if err != nil {
			return domain.ErrInvalidEmail
		}
		role := normalizeRole(invite.Role)
		if !isValidRole(role) {
			return domain.ErrInvalidRole
		}
		rows = append(rows, domain.OrganizationInvite{
			ID:        s.genID.Generate(),
			OrgID:     org.ID,
			Email:     email,
			Role:      role,
			Status:    "pending",
			InvitedBy: userID,
			CreatedAt: now,
		})
	}

	err = s.repo.CreateInvites(ctx, rows)
	if err != nil {
		return err
	}

	// Send emails
	for _, invite := range rows {
		go func(inv domain.OrganizationInvite) {
			msg := email.EmailMessage{
				To: []string{inv.Email},
			}
			if err := s.email.SendTemplate(context.Background(), msg, "invite_member", map[string]interface{}{
				"org_name":    org.Name,
				"invite_link": "http://localhost:8080/invite/" + inv.ID.String(),
				"role":        inv.Role,
			}); err != nil {
				zap.L().Error("failed to send invite email", zap.Error(err), zap.String("email", inv.Email))
			}
		}(invite)
	}

	return nil
}

func (s *service) GetInvite(ctx context.Context, inviteID string) (*domain.PublicInviteInfo, error) {
	rawInviteID := strings.TrimSpace(inviteID)
	if rawInviteID == "" {
		return nil, domain.ErrInvalidOrganization
	}
	parsedInviteID, err := snowflake.ParseString(rawInviteID)
	if err != nil {
		return nil, err
	}

	invite, err := s.repo.GetInvite(ctx, parsedInviteID)
	if err != nil {
		return nil, err
	}
	if invite == nil {
		return nil, domain.ErrInvalidOrganization
	}

	org, err := s.getOrganization(ctx, invite.OrgID)
	if err != nil {
		return nil, err
	}

	return &domain.PublicInviteInfo{
		ID:        invite.ID.String(),
		OrgID:     invite.OrgID.String(),
		OrgName:   org.Name,
		Email:     invite.Email,
		Role:      invite.Role,
		Status:    invite.Status,
		InvitedBy: invite.InvitedBy.String(),
	}, nil
}

func (s *service) AcceptInvite(ctx context.Context, userID snowflake.ID, inviteID string) error {
	if userID == 0 {
		return domain.ErrInvalidUser
	}

	rawInviteID := strings.TrimSpace(inviteID)
	if rawInviteID == "" {
		return domain.ErrInvalidOrganization // Should probably carry a specific invite error, but sticking to existing pattern or creating new one.
	}
	parsedInviteID, err := snowflake.ParseString(rawInviteID)
	if err != nil {
		return err
	}

	invite, err := s.repo.GetInvite(ctx, parsedInviteID)
	if err != nil {
		return err
	}
	if invite == nil {
		return domain.ErrInvalidOrganization
	}

	if invite.Status != "pending" {
		return errors.New("invite already accepted or expired")
	}

	// Check if user is already a member
	isMember, err := s.repo.IsMember(ctx, invite.OrgID, userID)
	if err != nil {
		return err
	}
	if isMember {
		// User is already a member, just mark invite as accepted
		invite.Status = "accepted"
		return s.repo.UpdateInvite(ctx, *invite)
	}

	// Add member
	now := time.Now().UTC()
	member := domain.OrganizationMember{
		ID:        s.genID.Generate(),
		OrgID:     invite.OrgID,
		UserID:    userID,
		Role:      invite.Role,
		CreatedAt: now,
	}

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		repo := s.repo.WithTx(tx)
		if err := repo.AddMember(ctx, member); err != nil {
			return err
		}

		invite.Status = "accepted"
		// update invite status
		if err := repo.UpdateInvite(ctx, *invite); err != nil {
			return err
		}
		return nil
	})

	return err
}

func (s *service) SetBillingPreferences(ctx context.Context, userID snowflake.ID, orgID string, req domain.BillingPreferencesRequest) error {
	if userID == 0 {
		return domain.ErrInvalidUser
	}

	rawOrgID := strings.TrimSpace(orgID)
	if rawOrgID == "" {
		return domain.ErrInvalidOrganization
	}
	parsedOrgID, err := snowflake.ParseString(rawOrgID)
	if err != nil {
		return domain.ErrInvalidOrganization
	}

	org, err := s.getOrganization(ctx, parsedOrgID)
	if err != nil {
		return err
	}

	isMember, err := s.repo.IsMember(ctx, parsedOrgID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return domain.ErrForbidden
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))
	if currency == "" {
		return domain.ErrInvalidCurrency
	}
	currencyOK, err := s.currencyExists(ctx, currency)
	if err != nil {
		return err
	}
	if !currencyOK {
		return domain.ErrInvalidCurrency
	}

	timezone := strings.TrimSpace(req.Timezone)
	if timezone == "" {
		return domain.ErrInvalidTimezone
	}
	timezoneOK, err := s.timezoneAllowed(ctx, org.CountryCode, timezone)
	if err != nil {
		return err
	}
	if !timezoneOK {
		return domain.ErrInvalidTimezone
	}

	now := time.Now().UTC()
	return s.repo.UpsertBillingPreferences(ctx, domain.OrganizationBillingPreferences{
		OrgID:     org.ID,
		Currency:  currency,
		Timezone:  timezone,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *service) countryExists(ctx context.Context, code string) (bool, error) {
	countries, err := s.ref.ListCountries(ctx)
	if err != nil {
		return false, err
	}
	for _, country := range countries {
		if country.Code == code {
			return true, nil
		}
	}
	return false, nil
}

func (s *service) timezoneAllowed(ctx context.Context, countryCode, timezoneName string) (bool, error) {
	timezones, err := s.ref.ListTimezonesByCountry(ctx, countryCode)
	if err != nil {
		return false, err
	}
	for _, tz := range timezones {
		if tz.Name == timezoneName {
			return true, nil
		}
	}
	return false, nil
}

func (s *service) currencyExists(ctx context.Context, code string) (bool, error) {
	currencies, err := s.ref.ListCurrencies(ctx)
	if err != nil {
		return false, err
	}
	for _, currency := range currencies {
		if currency.Code == code {
			return true, nil
		}
	}
	return false, nil
}

func (s *service) emitOrganizationCreated(ctx context.Context, org domain.Organization, ownerUserID snowflake.ID) {
	if s.publisher == nil {
		return
	}

	payload := map[string]string{
		"organization_id": org.ID.String(),
		"owner_user_id":   ownerUserID.String(),
		"country_code":    org.CountryCode,
		"timezone_name":   org.TimezoneName,
		"created_at":      org.CreatedAt.Format(time.RFC3339),
	}

	data, err := json.Marshal(payload)
	if err != nil {
		zap.L().Warn("failed to marshal organization.created payload", zap.Error(err))
		return
	}

	if err := s.publisher.Publish(ctx, event.OrganizationCreatedTopic, data); err != nil {
		zap.L().Warn("failed to publish organization.created", zap.Error(err))
	}
}

func (s *service) getOrganization(ctx context.Context, orgID snowflake.ID) (*domain.Organization, error) {
	var org domain.Organization
	if err := s.db.WithContext(ctx).First(&org, "id = ?", orgID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, domain.ErrInvalidOrganization
		}
		return nil, err
	}
	return &org, nil
}

func normalizeEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(addr.Address)), nil
}

func normalizeRole(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func isValidRole(role string) bool {
	switch role {
	case domain.RoleOwner, domain.RoleAdmin, domain.RoleMember, domain.RoleFinOps, domain.RoleDeveloper:
		return true
	default:
		return false
	}
}
