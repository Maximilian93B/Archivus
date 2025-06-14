package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/archivus/archivus/internal/domain/repositories"
	"github.com/archivus/archivus/internal/infrastructure/database/models"
	"github.com/google/uuid"
)

var (
	ErrTenantNotFound       = errors.New("tenant not found")
	ErrTenantExists         = errors.New("tenant already exists")
	ErrInvalidSubdomain     = errors.New("invalid subdomain")
	ErrSubdomainTaken       = errors.New("subdomain already taken")
	ErrTrialExpired         = errors.New("trial period expired")
	ErrSubscriptionInactive = errors.New("subscription inactive")
	ErrQuotaExceeded        = errors.New("quota exceeded")
	ErrInvalidBusinessInfo  = errors.New("invalid business information")
)

// TenantService manages multi-tenant functionality
type TenantService struct {
	tenantRepo   repositories.TenantRepository
	userRepo     repositories.UserRepository
	documentRepo repositories.DocumentRepository
	auditRepo    repositories.AuditLogRepository

	subscriptionService SubscriptionService
	config              TenantServiceConfig
}

// TenantServiceConfig holds configuration for tenant management
type TenantServiceConfig struct {
	DefaultTrialDays      int
	MaxSubdomainLength    int
	MinSubdomainLength    int
	ReservedSubdomains    []string
	DefaultStorageQuota   int64 // bytes
	DefaultAPIQuota       int
	RequireBusinessInfo   bool
	EnableCompliance      bool
	SupportedIndustries   []string
	SupportedCompanySizes []string
}

// NewTenantService creates a new tenant service
func NewTenantService(
	tenantRepo repositories.TenantRepository,
	userRepo repositories.UserRepository,
	documentRepo repositories.DocumentRepository,
	auditRepo repositories.AuditLogRepository,
	subscriptionService SubscriptionService,
	config TenantServiceConfig,
) *TenantService {
	return &TenantService{
		tenantRepo:          tenantRepo,
		userRepo:            userRepo,
		documentRepo:        documentRepo,
		auditRepo:           auditRepo,
		subscriptionService: subscriptionService,
		config:              config,
	}
}

// CreateTenantParams contains parameters for creating a new tenant
type CreateTenantParams struct {
	Name             string                  `json:"name"`
	Subdomain        string                  `json:"subdomain"`
	SubscriptionTier models.SubscriptionTier `json:"subscription_tier"`
	BusinessType     string                  `json:"business_type,omitempty"`
	Industry         string                  `json:"industry,omitempty"`
	CompanySize      string                  `json:"company_size,omitempty"`
	TaxID            string                  `json:"tax_id,omitempty"`
	Address          map[string]interface{}  `json:"address,omitempty"`
	Settings         map[string]interface{}  `json:"settings,omitempty"`

	// Admin user details
	AdminEmail     string `json:"admin_email"`
	AdminFirstName string `json:"admin_first_name"`
	AdminLastName  string `json:"admin_last_name"`
	AdminPassword  string `json:"admin_password"`
}

// TenantInfo contains detailed tenant information
type TenantInfo struct {
	*models.Tenant
	QuotaStatus        *repositories.QuotaStatus `json:"quota_status"`
	UserCount          int64                     `json:"user_count"`
	DocumentCount      int64                     `json:"document_count"`
	SubscriptionStatus string                    `json:"subscription_status"`
	DaysUntilExpiry    *int                      `json:"days_until_expiry,omitempty"`
}

// CreateTenant creates a new tenant with initial setup
func (s *TenantService) CreateTenant(ctx context.Context, params CreateTenantParams) (*models.Tenant, error) {
	// Validate subdomain
	if err := s.validateSubdomain(params.Subdomain); err != nil {
		return nil, err
	}

	// Check if subdomain is available
	existing, err := s.tenantRepo.GetBySubdomain(ctx, params.Subdomain)
	if err == nil && existing != nil {
		return nil, ErrSubdomainTaken
	}

	// Validate business information if required
	if s.config.RequireBusinessInfo {
		if err := s.validateBusinessInfo(params); err != nil {
			return nil, err
		}
	}

	// Set up trial period
	trialEnd := time.Now().AddDate(0, 0, s.config.DefaultTrialDays)

	// Create tenant
	tenant := &models.Tenant{
		ID:               uuid.New(),
		Name:             params.Name,
		Subdomain:        strings.ToLower(params.Subdomain),
		SubscriptionTier: params.SubscriptionTier,
		StorageQuota:     s.getStorageQuotaForTier(params.SubscriptionTier),
		APIQuota:         s.getAPIQuotaForTier(params.SubscriptionTier),
		Settings:         models.JSONB(params.Settings),
		IsActive:         true,
		TrialEndsAt:      &trialEnd,
		BusinessType:     params.BusinessType,
		Industry:         params.Industry,
		CompanySize:      params.CompanySize,
		TaxID:            params.TaxID,
		Address:          models.JSONB(params.Address),
	}

	// Set up default retention and compliance policies
	tenant.RetentionPolicy = s.getDefaultRetentionPolicy(params.Industry)
	if s.config.EnableCompliance {
		tenant.ComplianceRules = s.getDefaultComplianceRules(params.Industry)
	}

	// Create tenant
	if err := s.tenantRepo.Create(ctx, tenant); err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	// Create admin user
	if err := s.createAdminUser(ctx, tenant.ID, params); err != nil {
		// Rollback tenant creation
		s.tenantRepo.Delete(ctx, tenant.ID)
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	// Set up default folder structure
	if err := s.setupDefaultFolders(ctx, tenant.ID); err != nil {
		// Log but don't fail - folders can be created later
	}

	// Set up default categories
	if err := s.setupDefaultCategories(ctx, tenant.ID); err != nil {
		// Log but don't fail
	}

	// Initialize subscription
	if s.subscriptionService != nil {
		s.subscriptionService.InitializeSubscription(ctx, tenant.ID, params.SubscriptionTier)
	}

	return tenant, nil
}

// GetTenant retrieves tenant information
func (s *TenantService) GetTenant(ctx context.Context, tenantID uuid.UUID) (*TenantInfo, error) {
	tenant, err := s.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, ErrTenantNotFound
	}

	// Get quota status
	quotaStatus, err := s.tenantRepo.CheckQuotaLimits(ctx, tenantID)
	if err != nil {
		quotaStatus = nil // Don't fail if quota check fails
	}

	// Get user count
	users, userCount, err := s.userRepo.ListByTenant(ctx, tenantID, repositories.ListParams{PageSize: 1})
	if err != nil {
		userCount = 0
	}

	// Get document count
	documents, docCount, err := s.documentRepo.List(ctx, tenantID, repositories.DocumentFilters{
		ListParams: repositories.ListParams{PageSize: 1},
	})
	if err != nil {
		docCount = 0
	}

	// Calculate subscription status
	subscriptionStatus := s.getSubscriptionStatus(tenant)

	// Calculate days until expiry
	var daysUntilExpiry *int
	if tenant.TrialEndsAt != nil {
		days := int(time.Until(*tenant.TrialEndsAt).Hours() / 24)
		if days >= 0 {
			daysUntilExpiry = &days
		}
	}

	return &TenantInfo{
		Tenant:             tenant,
		QuotaStatus:        quotaStatus,
		UserCount:          userCount,
		DocumentCount:      docCount,
		SubscriptionStatus: subscriptionStatus,
		DaysUntilExpiry:    daysUntilExpiry,
	}, nil
}

// UpdateTenant updates tenant information
func (s *TenantService) UpdateTenant(ctx context.Context, tenantID uuid.UUID, updates map[string]interface{}, updatedBy uuid.UUID) (*models.Tenant, error) {
	tenant, err := s.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, ErrTenantNotFound
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		tenant.Name = name
	}

	if businessType, ok := updates["business_type"].(string); ok {
		tenant.BusinessType = businessType
	}

	if industry, ok := updates["industry"].(string); ok {
		tenant.Industry = industry
		// Update compliance rules based on new industry
		if s.config.EnableCompliance {
			tenant.ComplianceRules = s.getDefaultComplianceRules(industry)
		}
	}

	if companySize, ok := updates["company_size"].(string); ok {
		tenant.CompanySize = companySize
	}

	if taxID, ok := updates["tax_id"].(string); ok {
		tenant.TaxID = taxID
	}

	if address, ok := updates["address"].(map[string]interface{}); ok {
		tenant.Address = models.JSONB(address)
	}

	if settings, ok := updates["settings"].(map[string]interface{}); ok {
		// Merge with existing settings
		existingSettings := map[string]interface{}(tenant.Settings)
		for key, value := range settings {
			existingSettings[key] = value
		}
		tenant.Settings = models.JSONB(existingSettings)
	}

	tenant.UpdatedAt = time.Now()

	if err := s.tenantRepo.Update(ctx, tenant); err != nil {
		return nil, fmt.Errorf("failed to update tenant: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, updatedBy, tenantID, models.AuditUpdate, "Tenant updated")

	return tenant, nil
}

// UpgradeSubscription upgrades tenant subscription
func (s *TenantService) UpgradeSubscription(ctx context.Context, tenantID uuid.UUID, newTier models.SubscriptionTier, upgradedBy uuid.UUID) error {
	tenant, err := s.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return ErrTenantNotFound
	}

	// Validate upgrade path
	if !s.isValidUpgrade(tenant.SubscriptionTier, newTier) {
		return errors.New("invalid subscription upgrade")
	}

	// Update tenant subscription
	tenant.SubscriptionTier = newTier
	tenant.StorageQuota = s.getStorageQuotaForTier(newTier)
	tenant.APIQuota = s.getAPIQuotaForTier(newTier)

	// Remove trial if upgrading from starter
	if tenant.TrialEndsAt != nil && newTier != models.SubscriptionStarter {
		tenant.TrialEndsAt = nil
	}

	if err := s.tenantRepo.Update(ctx, tenant); err != nil {
		return fmt.Errorf("failed to upgrade subscription: %w", err)
	}

	// Update subscription service
	if s.subscriptionService != nil {
		if err := s.subscriptionService.UpgradeSubscription(ctx, tenantID, newTier); err != nil {
			// Log but don't fail
		}
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, upgradedBy, tenantID, models.AuditUpdate,
		fmt.Sprintf("Subscription upgraded to %s", string(newTier)))

	return nil
}

// SuspendTenant suspends a tenant (for non-payment, etc.)
func (s *TenantService) SuspendTenant(ctx context.Context, tenantID uuid.UUID, reason string, suspendedBy uuid.UUID) error {
	tenant, err := s.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return ErrTenantNotFound
	}

	tenant.IsActive = false
	tenant.UpdatedAt = time.Now()

	if err := s.tenantRepo.Update(ctx, tenant); err != nil {
		return fmt.Errorf("failed to suspend tenant: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, suspendedBy, tenantID, models.AuditUpdate,
		fmt.Sprintf("Tenant suspended: %s", reason))

	return nil
}

// ReactivateTenant reactivates a suspended tenant
func (s *TenantService) ReactivateTenant(ctx context.Context, tenantID uuid.UUID, reactivatedBy uuid.UUID) error {
	tenant, err := s.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return ErrTenantNotFound
	}

	tenant.IsActive = true
	tenant.UpdatedAt = time.Now()

	if err := s.tenantRepo.Update(ctx, tenant); err != nil {
		return fmt.Errorf("failed to reactivate tenant: %w", err)
	}

	// Create audit log
	s.createAuditLog(ctx, tenantID, reactivatedBy, tenantID, models.AuditUpdate, "Tenant reactivated")

	return nil
}

// GetTenantUsage gets detailed usage statistics
func (s *TenantService) GetTenantUsage(ctx context.Context, tenantID uuid.UUID) (*TenantUsage, error) {
	tenant, err := s.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, ErrTenantNotFound
	}

	quotaStatus, err := s.tenantRepo.CheckQuotaLimits(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get quota status: %w", err)
	}

	// Get document statistics
	docFilters := repositories.DocumentFilters{}
	_, totalDocs, err := s.documentRepo.List(ctx, tenantID, docFilters)
	if err != nil {
		totalDocs = 0
	}

	// Get user statistics
	_, totalUsers, err := s.userRepo.ListByTenant(ctx, tenantID, repositories.ListParams{})
	if err != nil {
		totalUsers = 0
	}

	return &TenantUsage{
		TenantID:       tenantID,
		QuotaStatus:    quotaStatus,
		TotalUsers:     totalUsers,
		TotalDocuments: totalDocs,
		LastUpdated:    time.Now(),
	}, nil
}

// CheckTenantHealth performs health checks on tenant
func (s *TenantService) CheckTenantHealth(ctx context.Context, tenantID uuid.UUID) (*TenantHealth, error) {
	tenant, err := s.tenantRepo.GetByID(ctx, tenantID)
	if err != nil {
		return nil, ErrTenantNotFound
	}

	health := &TenantHealth{
		TenantID:  tenantID,
		IsHealthy: true,
		Issues:    []string{},
		Warnings:  []string{},
	}

	// Check if tenant is active
	if !tenant.IsActive {
		health.IsHealthy = false
		health.Issues = append(health.Issues, "Tenant is suspended")
	}

	// Check trial expiry
	if tenant.TrialEndsAt != nil && time.Now().After(*tenant.TrialEndsAt) {
		health.IsHealthy = false
		health.Issues = append(health.Issues, "Trial period expired")
	}

	// Check quotas
	quotaStatus, err := s.tenantRepo.CheckQuotaLimits(ctx, tenantID)
	if err == nil {
		if quotaStatus.StoragePercent > 90 {
			health.Warnings = append(health.Warnings, "Storage quota nearly exceeded")
		}
		if quotaStatus.APIPercent > 90 {
			health.Warnings = append(health.Warnings, "API quota nearly exceeded")
		}
		if !quotaStatus.CanUpload {
			health.IsHealthy = false
			health.Issues = append(health.Issues, "Storage quota exceeded")
		}
	}

	return health, nil
}

// Helper methods

func (s *TenantService) validateSubdomain(subdomain string) error {
	if len(subdomain) < s.config.MinSubdomainLength || len(subdomain) > s.config.MaxSubdomainLength {
		return ErrInvalidSubdomain
	}

	// Check format (alphanumeric and hyphens only)
	matched, _ := regexp.MatchString(`^[a-z0-9-]+$`, subdomain)
	if !matched {
		return ErrInvalidSubdomain
	}

	// Check reserved subdomains
	for _, reserved := range s.config.ReservedSubdomains {
		if subdomain == reserved {
			return ErrInvalidSubdomain
		}
	}

	return nil
}

func (s *TenantService) validateBusinessInfo(params CreateTenantParams) error {
	if params.BusinessType == "" {
		return ErrInvalidBusinessInfo
	}

	if params.Industry != "" {
		found := false
		for _, industry := range s.config.SupportedIndustries {
			if params.Industry == industry {
				found = true
				break
			}
		}
		if !found && len(s.config.SupportedIndustries) > 0 {
			return ErrInvalidBusinessInfo
		}
	}

	if params.CompanySize != "" {
		found := false
		for _, size := range s.config.SupportedCompanySizes {
			if params.CompanySize == size {
				found = true
				break
			}
		}
		if !found && len(s.config.SupportedCompanySizes) > 0 {
			return ErrInvalidBusinessInfo
		}
	}

	return nil
}

func (s *TenantService) getStorageQuotaForTier(tier models.SubscriptionTier) int64 {
	switch tier {
	case models.SubscriptionStarter:
		return 5 * 1024 * 1024 * 1024 // 5GB
	case models.SubscriptionProfessional:
		return 50 * 1024 * 1024 * 1024 // 50GB
	case models.SubscriptionEnterprise:
		return 500 * 1024 * 1024 * 1024 // 500GB
	default:
		return s.config.DefaultStorageQuota
	}
}

func (s *TenantService) getAPIQuotaForTier(tier models.SubscriptionTier) int {
	switch tier {
	case models.SubscriptionStarter:
		return 1000
	case models.SubscriptionProfessional:
		return 10000
	case models.SubscriptionEnterprise:
		return 100000
	default:
		return s.config.DefaultAPIQuota
	}
}

func (s *TenantService) getDefaultRetentionPolicy(industry string) models.JSONB {
	// Industry-specific retention policies
	policies := map[string]interface{}{
		"default_retention_years": 7,
		"document_specific": map[string]interface{}{
			"tax_documents":   10,
			"legal_documents": 7,
			"hr_documents":    7,
		},
	}

	// Adjust based on industry
	switch industry {
	case "finance", "banking":
		policies["default_retention_years"] = 10
	case "healthcare":
		policies["default_retention_years"] = 6
	case "legal":
		policies["default_retention_years"] = 15
	}

	return models.JSONB(policies)
}

func (s *TenantService) getDefaultComplianceRules(industry string) models.JSONB {
	rules := map[string]interface{}{
		"encryption_required": true,
		"audit_required":      true,
		"access_logging":      true,
	}

	// Industry-specific compliance
	switch industry {
	case "healthcare":
		rules["hipaa_compliant"] = true
		rules["data_anonymization"] = true
	case "finance", "banking":
		rules["sox_compliant"] = true
		rules["pci_dss"] = true
	case "legal":
		rules["attorney_client_privilege"] = true
	}

	return models.JSONB(rules)
}

func (s *TenantService) getSubscriptionStatus(tenant *models.Tenant) string {
	if !tenant.IsActive {
		return "suspended"
	}

	if tenant.TrialEndsAt != nil {
		if time.Now().After(*tenant.TrialEndsAt) {
			return "trial_expired"
		}
		return "trial"
	}

	return "active"
}

func (s *TenantService) isValidUpgrade(current, new models.SubscriptionTier) bool {
	tiers := map[models.SubscriptionTier]int{
		models.SubscriptionStarter:      1,
		models.SubscriptionProfessional: 2,
		models.SubscriptionEnterprise:   3,
	}

	return tiers[new] > tiers[current]
}

func (s *TenantService) createAdminUser(ctx context.Context, tenantID uuid.UUID, params CreateTenantParams) error {
	// This would hash the password and create the admin user
	// Implementation depends on your user service
	return nil
}

func (s *TenantService) setupDefaultFolders(ctx context.Context, tenantID uuid.UUID) error {
	// Create default folder structure for SMB
	defaultFolders := []string{
		"Invoices",
		"Receipts",
		"Contracts",
		"Tax Documents",
		"HR Documents",
		"Marketing Materials",
		"Reports",
	}

	// Implementation would create these folders
	return nil
}

func (s *TenantService) setupDefaultCategories(ctx context.Context, tenantID uuid.UUID) error {
	// Create default categories for SMB
	return nil
}

func (s *TenantService) createAuditLog(ctx context.Context, tenantID, userID, resourceID uuid.UUID, action models.AuditAction, details string) {
	log := &models.AuditLog{
		TenantID:     tenantID,
		UserID:       userID,
		ResourceID:   resourceID,
		Action:       action,
		ResourceType: "tenant",
		Details:      models.JSONB{"message": details},
	}

	go func() {
		s.auditRepo.Create(context.Background(), log)
	}()
}

// Supporting types

type TenantUsage struct {
	TenantID       uuid.UUID                 `json:"tenant_id"`
	QuotaStatus    *repositories.QuotaStatus `json:"quota_status"`
	TotalUsers     int64                     `json:"total_users"`
	TotalDocuments int64                     `json:"total_documents"`
	LastUpdated    time.Time                 `json:"last_updated"`
}

type TenantHealth struct {
	TenantID  uuid.UUID `json:"tenant_id"`
	IsHealthy bool      `json:"is_healthy"`
	Issues    []string  `json:"issues"`
	Warnings  []string  `json:"warnings"`
}

// External service interface
type SubscriptionService interface {
	InitializeSubscription(ctx context.Context, tenantID uuid.UUID, tier models.SubscriptionTier) error
	UpgradeSubscription(ctx context.Context, tenantID uuid.UUID, newTier models.SubscriptionTier) error
	CancelSubscription(ctx context.Context, tenantID uuid.UUID) error
	GetSubscriptionStatus(ctx context.Context, tenantID uuid.UUID) (string, error)
}
