// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package licensegateprocessor

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "otel_license"

type gateMetrics struct {
	droppedCounter metric.Int64Counter
}

func newGateMetrics(mp metric.MeterProvider) (*gateMetrics, error) {
	meter := mp.Meter(meterName)

	dropped, err := meter.Int64Counter("otel_data_dropped_total",
		metric.WithDescription("Total number of data points dropped due to license expiration"),
	)
	if err != nil {
		return nil, err
	}

	return &gateMetrics{
		droppedCounter: dropped,
	}, nil
}

func (m *gateMetrics) recordDropped(ctx context.Context, count int64, signal string) {
	m.droppedCounter.Add(ctx, count,
		metric.WithAttributes(
			attribute.String("reason", "license_expired"),
			attribute.String("signal", signal),
		),
	)
}
