// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package licenseguardextension

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/extension"
)

const (
	typeStr = "license_guard"

	defaultCheckInterval  = 1 * time.Hour
	defaultGracePeriodDays = 7
)

func NewFactory() extension.Factory {
	return extension.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		createExtension,
		component.StabilityLevelBeta,
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		CheckInterval:   defaultCheckInterval,
		GracePeriodDays: defaultGracePeriodDays,
		FailMode:        FailModeClosed,
	}
}

func createExtension(_ context.Context, set extension.Settings, cfg component.Config) (extension.Extension, error) {
	return newLicenseGuardExtension(set, cfg.(*Config))
}
