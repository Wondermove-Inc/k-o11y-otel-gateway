// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package licenseguardextension

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
	"go.uber.org/zap"
)

// LicenseClaims represents the JWT claims structure for K-O11y license keys.
type LicenseClaims struct {
	TenantID   string `json:"tenant_id"`
	ClusterID  string `json:"cluster_id"`
	ContractID string `json:"contract_id"`
	jwt.RegisteredClaims
}

// LicenseChecker is the interface that the processor uses to check license status.
type LicenseChecker interface {
	IsValid() bool
	IsGracePeriod() bool
	ExpiresInDays() float64
	GracePeriodRemainingDays() float64
	TenantID() string
	ContractID() string
}

type licenseGuardExtension struct {
	config    *Config
	logger    *zap.Logger
	publicKey *rsa.PublicKey

	mu              sync.RWMutex
	valid           bool
	gracePeriod     bool
	expiresAt       time.Time
	gracePeriodEnd  time.Time
	tenantID        string
	contractID      string
	passThrough     bool // true when no license key is configured

	cancel  context.CancelFunc
	metrics *licenseMetrics
}

var _ extension.Extension = (*licenseGuardExtension)(nil)
var _ LicenseChecker = (*licenseGuardExtension)(nil)

func newLicenseGuardExtension(set extension.Settings, cfg *Config) (*licenseGuardExtension, error) {
	ext := &licenseGuardExtension{
		config: cfg,
		logger: set.Logger,
	}

	// Resolve license key: direct value > environment variable
	licenseKey := cfg.LicenseKey
	if licenseKey == "" && cfg.LicenseKeyEnv != "" {
		licenseKey = os.Getenv(cfg.LicenseKeyEnv)
		if licenseKey != "" {
			ext.logger.Info("License key loaded from environment variable",
				zap.String("env_var", cfg.LicenseKeyEnv))
		}
	}
	cfg.LicenseKey = licenseKey

	// Pass-through mode when no license key is configured
	if cfg.LicenseKey == "" {
		ext.passThrough = true
		ext.valid = true
		return ext, nil
	}

	// Parse public key
	if cfg.PublicKeyPEM == "" {
		return nil, fmt.Errorf("public_key_pem is required when license_key is set")
	}

	pubKey, err := parsePublicKey(cfg.PublicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	ext.publicKey = pubKey

	// Initialize metrics
	m, err := newLicenseMetrics(set.TelemetrySettings.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize license metrics: %w", err)
	}
	ext.metrics = m

	return ext, nil
}

func (e *licenseGuardExtension) Start(_ context.Context, _ component.Host) error {
	if e.passThrough {
		e.logger.Info("License guard running in pass-through mode (no license key configured)")
		return nil
	}

	// Initial validation
	if err := e.checkLicense(); err != nil {
		e.logger.Error("Initial license validation failed", zap.Error(err))
		// Don't return error — start the extension but mark as invalid
	}

	// Start periodic re-validation
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	go e.validationLoop(ctx)

	return nil
}

func (e *licenseGuardExtension) Shutdown(_ context.Context) error {
	if e.cancel != nil {
		e.cancel()
	}
	return nil
}

// validationLoop periodically re-validates the license.
func (e *licenseGuardExtension) validationLoop(ctx context.Context) {
	ticker := time.NewTicker(e.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := e.checkLicense(); err != nil {
				e.logger.Error("License re-validation failed", zap.Error(err))
			}
		}
	}
}

// checkLicense validates the JWT and updates the extension state.
func (e *licenseGuardExtension) checkLicense() error {
	claims, err := e.verifyJWT(e.config.LicenseKey)
	if err != nil {
		e.mu.Lock()
		if e.config.FailMode == FailModeOpen {
			e.valid = true
			e.gracePeriod = false
			e.mu.Unlock()
			e.logger.Error("JWT verification failed but fail_mode=open — data ingestion continues",
				zap.Error(err))
			return nil
		}
		e.valid = false
		e.gracePeriod = false
		e.mu.Unlock()
		return fmt.Errorf("JWT verification failed: %w", err)
	}

	now := time.Now()
	gracePeriodEnd := claims.ExpiresAt.Time.Add(time.Duration(e.config.GracePeriodDays) * 24 * time.Hour)
	daysUntilExpiry := time.Until(claims.ExpiresAt.Time).Hours() / 24

	e.mu.Lock()
	e.tenantID = claims.TenantID
	e.contractID = claims.ContractID
	e.expiresAt = claims.ExpiresAt.Time
	e.gracePeriodEnd = gracePeriodEnd

	if now.Before(claims.ExpiresAt.Time) {
		// License is valid (not expired)
		e.valid = true
		e.gracePeriod = false

		if daysUntilExpiry <= 30 {
			e.logger.Warn("License expiring soon",
				zap.Float64("days_remaining", daysUntilExpiry),
				zap.Time("expires_at", claims.ExpiresAt.Time),
				zap.String("tenant_id", claims.TenantID),
			)
		}
		if daysUntilExpiry <= 7 {
			e.logger.Warn("License expiring in less than 7 days — renew immediately",
				zap.Float64("days_remaining", daysUntilExpiry),
				zap.String("tenant_id", claims.TenantID),
				zap.String("contract_id", claims.ContractID),
			)
		}
	} else if now.Before(gracePeriodEnd) {
		// Expired but within grace period
		e.valid = true
		e.gracePeriod = true
		graceDaysLeft := time.Until(gracePeriodEnd).Hours() / 24
		e.logger.Error("License expired — grace period active, data ingestion will stop soon",
			zap.Float64("grace_days_remaining", graceDaysLeft),
			zap.Time("grace_period_ends", gracePeriodEnd),
			zap.String("tenant_id", claims.TenantID),
		)
	} else {
		// Expired and grace period ended
		e.valid = false
		e.gracePeriod = false
		e.logger.Error("License expired and grace period ended — data ingestion stopped",
			zap.Time("expired_at", claims.ExpiresAt.Time),
			zap.Time("grace_ended_at", gracePeriodEnd),
			zap.String("tenant_id", claims.TenantID),
		)
	}
	e.mu.Unlock()

	if e.valid {
		e.logger.Info("License validated",
			zap.Bool("grace_period", e.gracePeriod),
			zap.Float64("expires_in_days", daysUntilExpiry),
			zap.String("tenant_id", claims.TenantID),
		)
	}

	// Record metrics
	if e.metrics != nil {
		e.metrics.setAttributes(claims.TenantID, claims.ContractID)
		e.metrics.record(context.Background(), e.valid, e.expiresAt, e.gracePeriod, e.gracePeriodEnd)
	}

	return nil
}

// verifyJWT parses and verifies the JWT RS256 signature.
// It returns claims even if the token is expired, because grace period logic
// needs the expiration time. Only signature failures are hard errors.
// exp validation is skipped here and handled by checkLicense().
func (e *licenseGuardExtension) verifyJWT(tokenString string) (*LicenseClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &LicenseClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return e.publicKey, nil
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithoutClaimsValidation(), // skip exp — grace period handles it in checkLicense()
	)
	if err != nil {
		return nil, fmt.Errorf("signature verification failed: %w", err)
	}

	claims, ok := token.Claims.(*LicenseClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token or claims type")
	}

	if claims.TenantID == "" {
		return nil, fmt.Errorf("tenant_id is missing from JWT claims")
	}

	if claims.ExpiresAt == nil {
		return nil, fmt.Errorf("exp claim is missing from JWT")
	}

	return claims, nil
}

// --- LicenseChecker interface ---

func (e *licenseGuardExtension) IsValid() bool {
	if e.passThrough {
		return true
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.valid
}

func (e *licenseGuardExtension) IsGracePeriod() bool {
	if e.passThrough {
		return false
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.gracePeriod
}

func (e *licenseGuardExtension) ExpiresInDays() float64 {
	if e.passThrough {
		return math.MaxFloat64
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return time.Until(e.expiresAt).Hours() / 24
}

func (e *licenseGuardExtension) GracePeriodRemainingDays() float64 {
	if e.passThrough {
		return 0
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	if !e.gracePeriod {
		return 0
	}
	return time.Until(e.gracePeriodEnd).Hours() / 24
}

func (e *licenseGuardExtension) TenantID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.tenantID
}

func (e *licenseGuardExtension) ContractID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.contractID
}

// parsePublicKey parses a PEM-encoded RSA public key.
func parsePublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not RSA public key")
	}

	return rsaPub, nil
}
