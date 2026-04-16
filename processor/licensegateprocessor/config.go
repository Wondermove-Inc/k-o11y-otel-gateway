// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package licensegateprocessor

// Config holds the configuration for the license gate processor.
type Config struct {
	// ExtensionName is the name of the license_guard extension to reference.
	// Default: "license_guard"
	ExtensionName string `mapstructure:"extension_name"`
}
