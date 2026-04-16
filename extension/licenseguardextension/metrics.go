// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package licenseguardextension

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "otel_license"

type licenseMetrics struct {
	validGauge              metric.Int64Gauge
	expiresInDaysGauge      metric.Float64Gauge
	gracePeriodDaysGauge    metric.Float64Gauge
	attrs                   []attribute.KeyValue
}

func newLicenseMetrics(mp metric.MeterProvider) (*licenseMetrics, error) {
	meter := mp.Meter(meterName)

	validGauge, err := meter.Int64Gauge("otel_license_valid",
		metric.WithDescription("Whether the license is currently valid (1=valid, 0=invalid)"),
	)
	if err != nil {
		return nil, err
	}

	expiresGauge, err := meter.Float64Gauge("otel_license_expires_in_days",
		metric.WithDescription("Days until license expiration (negative = expired)"),
	)
	if err != nil {
		return nil, err
	}

	graceGauge, err := meter.Float64Gauge("otel_grace_period_remaining_days",
		metric.WithDescription("Days remaining in grace period (0 if not in grace period)"),
	)
	if err != nil {
		return nil, err
	}

	return &licenseMetrics{
		validGauge:           validGauge,
		expiresInDaysGauge:   expiresGauge,
		gracePeriodDaysGauge: graceGauge,
	}, nil
}

func (m *licenseMetrics) setAttributes(tenantID, contractID string) {
	m.attrs = []attribute.KeyValue{
		attribute.String("tenant_id", tenantID),
		attribute.String("contract_id", contractID),
	}
}

func (m *licenseMetrics) record(ctx context.Context, valid bool, expiresAt time.Time, gracePeriod bool, gracePeriodEnd time.Time) {
	opts := metric.WithAttributes(m.attrs...)

	if valid {
		m.validGauge.Record(ctx, 1, opts)
	} else {
		m.validGauge.Record(ctx, 0, opts)
	}

	daysUntilExpiry := time.Until(expiresAt).Hours() / 24
	m.expiresInDaysGauge.Record(ctx, daysUntilExpiry, opts)

	if gracePeriod {
		graceDays := time.Until(gracePeriodEnd).Hours() / 24
		m.gracePeriodDaysGauge.Record(ctx, graceDays, opts)
	} else {
		m.gracePeriodDaysGauge.Record(ctx, 0, opts)
	}
}
