// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package licenseguardextension

import (
	"time"
)

// FailMode controls behavior when license validation fails (e.g., wrong public key).
const (
	// FailModeClosed blocks all data when license validation fails. Default.
	FailModeClosed = "closed"
	// FailModeOpen allows all data through even when license validation fails.
	// Use for initial deployment testing to avoid accidental data loss.
	FailModeOpen = "open"
)

// Config holds the configuration for the license guard extension.
type Config struct {
	// LicenseKey is the JWT token for license validation (RS256 signed).
	// If empty (and LicenseKeyEnv is also empty), the extension runs in pass-through mode.
	LicenseKey string `mapstructure:"license_key"`

	// LicenseKeyEnv is the environment variable name containing the license key JWT.
	// Used to avoid exposing the license key in ConfigMap.
	// If both LicenseKey and LicenseKeyEnv are set, LicenseKey takes precedence.
	LicenseKeyEnv string `mapstructure:"license_key_env"`

	// PublicKeyPEM is the RSA public key in PEM format for JWT signature verification.
	PublicKeyPEM string `mapstructure:"public_key_pem"`

	// CheckInterval is the interval between license re-validation checks.
	// Default: 1h
	CheckInterval time.Duration `mapstructure:"check_interval"`

	// GracePeriodDays is the number of days after expiration during which
	// data ingestion is still allowed.
	// Default: 7
	GracePeriodDays int `mapstructure:"grace_period_days"`

	// FailMode controls behavior when license validation fails (signature error, wrong key, etc.).
	// "closed" (default): block all data when validation fails.
	// "open": allow all data through even when validation fails (log warnings).
	FailMode string `mapstructure:"fail_mode"`
}
