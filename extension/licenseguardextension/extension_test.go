// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package licenseguardextension

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/extension"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.uber.org/zap"
)

// test helpers
var (
	testPrivateKey *rsa.PrivateKey
	testPublicPEM  string
)

func init() {
	var err error
	testPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	pubBytes, err := x509.MarshalPKIXPublicKey(&testPrivateKey.PublicKey)
	if err != nil {
		panic(err)
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}
	testPublicPEM = string(pem.EncodeToMemory(block))
}

func makeJWT(t *testing.T, claims LicenseClaims) string {
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(testPrivateKey)
	require.NoError(t, err)
	return signed
}

func testExtSettings(t *testing.T) extension.Settings {
	mp := sdkmetric.NewMeterProvider()
	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })
	set := extension.Settings{}
	set.Logger = zap.NewNop()
	set.TelemetrySettings.MeterProvider = mp
	return set
}

// --- Tests ---

func TestPassThroughMode(t *testing.T) {
	cfg := &Config{LicenseKey: ""} // no license key
	set := testExtSettings(t)

	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)

	assert.True(t, ext.passThrough)
	assert.True(t, ext.IsValid())
	assert.False(t, ext.IsGracePeriod())
	assert.Equal(t, "", ext.TenantID())
}

func TestValidLicense(t *testing.T) {
	tokenStr := makeJWT(t, LicenseClaims{
		TenantID:   "tenant-001",
		ContractID: "contract-001",
		ClusterID:  "cluster-001",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(365 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ko11y",
		},
	})

	cfg := &Config{
		LicenseKey:      tokenStr,
		PublicKeyPEM:    testPublicPEM,
		CheckInterval:   1 * time.Hour,
		GracePeriodDays: 7,
	}

	set := testExtSettings(t)

	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)
	require.False(t, ext.passThrough)

	err = ext.checkLicense()
	require.NoError(t, err)

	assert.True(t, ext.IsValid())
	assert.False(t, ext.IsGracePeriod())
	assert.Greater(t, ext.ExpiresInDays(), 364.0)
	assert.Equal(t, "tenant-001", ext.TenantID())
	assert.Equal(t, "contract-001", ext.ContractID())
}

func TestExpiredLicenseWithinGracePeriod(t *testing.T) {
	// Expired 2 days ago, grace period is 7 days → should still be valid
	tokenStr := makeJWT(t, LicenseClaims{
		TenantID:   "tenant-002",
		ContractID: "contract-002",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-2 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-367 * 24 * time.Hour)),
			Issuer:    "ko11y",
		},
	})

	cfg := &Config{
		LicenseKey:      tokenStr,
		PublicKeyPEM:    testPublicPEM,
		CheckInterval:   1 * time.Hour,
		GracePeriodDays: 7,
	}

	set := testExtSettings(t)

	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)

	err = ext.checkLicense()
	require.NoError(t, err)

	assert.True(t, ext.IsValid(), "should be valid during grace period")
	assert.True(t, ext.IsGracePeriod(), "should be in grace period")
	assert.Greater(t, ext.GracePeriodRemainingDays(), 4.0)
	assert.Less(t, ext.GracePeriodRemainingDays(), 6.0)
}

func TestExpiredLicenseGracePeriodEnded(t *testing.T) {
	// Expired 10 days ago, grace period is 7 days → should be invalid
	tokenStr := makeJWT(t, LicenseClaims{
		TenantID:   "tenant-003",
		ContractID: "contract-003",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-10 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-375 * 24 * time.Hour)),
			Issuer:    "ko11y",
		},
	})

	cfg := &Config{
		LicenseKey:      tokenStr,
		PublicKeyPEM:    testPublicPEM,
		CheckInterval:   1 * time.Hour,
		GracePeriodDays: 7,
	}

	set := testExtSettings(t)

	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)

	err = ext.checkLicense()
	require.NoError(t, err)

	assert.False(t, ext.IsValid(), "should be invalid after grace period")
	assert.False(t, ext.IsGracePeriod())
}

func TestInvalidSignature(t *testing.T) {
	// Sign with a different key
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, LicenseClaims{
		TenantID: "tenant-004",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "ko11y",
		},
	})
	badToken, _ := token.SignedString(otherKey)

	cfg := &Config{
		LicenseKey:      badToken,
		PublicKeyPEM:    testPublicPEM,
		CheckInterval:   1 * time.Hour,
		GracePeriodDays: 7,
	}

	set := testExtSettings(t)

	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)

	err = ext.checkLicense()
	assert.Error(t, err, "should fail with wrong signature")
	assert.Contains(t, err.Error(), "signature verification failed")
	assert.False(t, ext.IsValid())
}

func TestMissingTenantID(t *testing.T) {
	tokenStr := makeJWT(t, LicenseClaims{
		TenantID: "", // missing
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "ko11y",
		},
	})

	cfg := &Config{
		LicenseKey:      tokenStr,
		PublicKeyPEM:    testPublicPEM,
		CheckInterval:   1 * time.Hour,
		GracePeriodDays: 7,
	}

	set := testExtSettings(t)

	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)

	err = ext.checkLicense()
	assert.Error(t, err, "should fail with missing tenant_id")
	assert.Contains(t, err.Error(), "tenant_id")
}

func TestMissingPublicKey(t *testing.T) {
	cfg := &Config{
		LicenseKey:   "some-token",
		PublicKeyPEM: "",
	}

	set := testExtSettings(t)

	_, err := newLicenseGuardExtension(set, cfg)
	assert.Error(t, err, "should fail without public key")
	assert.Contains(t, err.Error(), "public_key_pem is required")
}

func TestMalformedPublicKey(t *testing.T) {
	cfg := &Config{
		LicenseKey:   "some-token",
		PublicKeyPEM: "not-a-pem",
	}

	set := testExtSettings(t)

	_, err := newLicenseGuardExtension(set, cfg)
	assert.Error(t, err, "should fail with malformed public key")
	assert.Contains(t, err.Error(), "failed to parse public key")
}

func TestWarningAt30Days(t *testing.T) {
	// Expires in 25 days → should log D-30 warning
	tokenStr := makeJWT(t, LicenseClaims{
		TenantID:   "tenant-warn",
		ContractID: "contract-warn",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(25 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ko11y",
		},
	})

	cfg := &Config{
		LicenseKey:      tokenStr,
		PublicKeyPEM:    testPublicPEM,
		CheckInterval:   1 * time.Hour,
		GracePeriodDays: 7,
	}

	set := testExtSettings(t)

	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)

	err = ext.checkLicense()
	require.NoError(t, err)

	assert.True(t, ext.IsValid())
	assert.Less(t, ext.ExpiresInDays(), 30.0)
	assert.Greater(t, ext.ExpiresInDays(), 24.0)
}

func TestAlgorithmConfusion(t *testing.T) {
	// Try HS256 (HMAC) instead of RS256
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, LicenseClaims{
		TenantID: "tenant-hack",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	})
	badToken, _ := token.SignedString([]byte("secret"))

	cfg := &Config{
		LicenseKey:      badToken,
		PublicKeyPEM:    testPublicPEM,
		CheckInterval:   1 * time.Hour,
		GracePeriodDays: 7,
	}

	set := testExtSettings(t)

	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)

	err = ext.checkLicense()
	assert.Error(t, err, "should reject HS256 algorithm")
	assert.False(t, ext.IsValid())
}

func TestFailModeOpen(t *testing.T) {
	// Use wrong public key to trigger signature failure
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, LicenseClaims{
		TenantID: "tenant-failopen",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "ko11y",
		},
	})
	badToken, _ := token.SignedString(otherKey)

	cfg := &Config{
		LicenseKey:      badToken,
		PublicKeyPEM:    testPublicPEM,
		CheckInterval:   1 * time.Hour,
		GracePeriodDays: 7,
		FailMode:        FailModeOpen,
	}

	set := testExtSettings(t)
	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)

	err = ext.checkLicense()
	assert.NoError(t, err, "fail_mode=open should not return error")
	assert.True(t, ext.IsValid(), "fail_mode=open should keep valid=true on signature failure")
}

func TestFailModeClosed(t *testing.T) {
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, LicenseClaims{
		TenantID: "tenant-failclosed",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			Issuer:    "ko11y",
		},
	})
	badToken, _ := token.SignedString(otherKey)

	cfg := &Config{
		LicenseKey:      badToken,
		PublicKeyPEM:    testPublicPEM,
		CheckInterval:   1 * time.Hour,
		GracePeriodDays: 7,
		FailMode:        FailModeClosed,
	}

	set := testExtSettings(t)
	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)

	err = ext.checkLicense()
	assert.Error(t, err, "fail_mode=closed should return error on signature failure")
	assert.False(t, ext.IsValid(), "fail_mode=closed should set valid=false")
}

func TestLicenseKeyEnv(t *testing.T) {
	tokenStr := makeJWT(t, LicenseClaims{
		TenantID: "tenant-env",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(365 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "ko11y",
		},
	})

	os.Setenv("TEST_LICENSE_KEY_FOR_GUARD", tokenStr)
	defer os.Unsetenv("TEST_LICENSE_KEY_FOR_GUARD")

	cfg := &Config{
		LicenseKey:      "", // empty — should fall back to env
		LicenseKeyEnv:   "TEST_LICENSE_KEY_FOR_GUARD",
		PublicKeyPEM:    testPublicPEM,
		CheckInterval:   1 * time.Hour,
		GracePeriodDays: 7,
		FailMode:        FailModeClosed,
	}

	set := testExtSettings(t)
	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)
	require.False(t, ext.passThrough, "should not be pass-through when env var has key")

	err = ext.checkLicense()
	require.NoError(t, err)
	assert.True(t, ext.IsValid())
	assert.Equal(t, "tenant-env", ext.TenantID())
}

func TestLicenseKeyEnvEmpty_PassThrough(t *testing.T) {
	os.Unsetenv("NONEXISTENT_LICENSE_KEY")

	cfg := &Config{
		LicenseKey:    "",
		LicenseKeyEnv: "NONEXISTENT_LICENSE_KEY",
	}

	set := testExtSettings(t)
	ext, err := newLicenseGuardExtension(set, cfg)
	require.NoError(t, err)
	assert.True(t, ext.passThrough, "should be pass-through when env var is empty")
}
