// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package licensegateprocessor

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.uber.org/zap"

	"github.com/SigNoz/signoz-otel-collector/extension/licenseguardextension"
)

type licenseGateProcessor struct {
	config       *Config
	logger       *zap.Logger
	nextTraces   consumer.Traces
	nextLogs     consumer.Logs
	nextMetrics  consumer.Metrics

	checker      licenseguardextension.LicenseChecker
	checkerOnce  sync.Once
	host         component.Host
	metrics      *gateMetrics
}

func newLicenseGateProcessor(
	set processor.Settings,
	cfg *Config,
	nextTraces consumer.Traces,
	nextLogs consumer.Logs,
	nextMetrics consumer.Metrics,
) (*licenseGateProcessor, error) {
	m, err := newGateMetrics(set.TelemetrySettings.MeterProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize gate metrics: %w", err)
	}
	return &licenseGateProcessor{
		config:      cfg,
		logger:      set.Logger,
		nextTraces:  nextTraces,
		nextLogs:    nextLogs,
		nextMetrics: nextMetrics,
		metrics:     m,
	}, nil
}

func (p *licenseGateProcessor) Start(_ context.Context, host component.Host) error {
	p.host = host
	return nil
}

func (p *licenseGateProcessor) Shutdown(_ context.Context) error {
	return nil
}

func (p *licenseGateProcessor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// getChecker lazily resolves the license extension from the host.
func (p *licenseGateProcessor) getChecker() licenseguardextension.LicenseChecker {
	p.checkerOnce.Do(func() {
		if p.host == nil {
			return
		}
		for id, ext := range p.host.GetExtensions() {
			if id.Type().String() == p.config.ExtensionName {
				if checker, ok := ext.(licenseguardextension.LicenseChecker); ok {
					p.checker = checker
					p.logger.Info("License gate processor linked to extension",
						zap.String("extension", p.config.ExtensionName))
				}
				break
			}
		}
		if p.checker == nil {
			p.logger.Warn("License guard extension not found — running in pass-through mode",
				zap.String("expected_extension", p.config.ExtensionName))
		}
	})
	return p.checker
}

func (p *licenseGateProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	checker := p.getChecker()
	if checker == nil || checker.IsValid() {
		if checker != nil && checker.IsGracePeriod() {
			p.logger.Warn("License expired — grace period active, traces still accepted",
				zap.Int("span_count", td.SpanCount()))
		}
		return p.nextTraces.ConsumeTraces(ctx, td)
	}
	count := int64(td.SpanCount())
	p.logger.Error("License expired — traces dropped",
		zap.Int64("span_count", count),
		zap.String("tenant_id", checker.TenantID()))
	if p.metrics != nil {
		p.metrics.recordDropped(ctx, count, "traces")
	}
	return nil
}

func (p *licenseGateProcessor) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	checker := p.getChecker()
	if checker == nil || checker.IsValid() {
		if checker != nil && checker.IsGracePeriod() {
			p.logger.Warn("License expired — grace period active, logs still accepted",
				zap.Int("log_count", ld.LogRecordCount()))
		}
		return p.nextLogs.ConsumeLogs(ctx, ld)
	}
	count := int64(ld.LogRecordCount())
	p.logger.Error("License expired — logs dropped",
		zap.Int64("log_count", count),
		zap.String("tenant_id", checker.TenantID()))
	if p.metrics != nil {
		p.metrics.recordDropped(ctx, count, "logs")
	}
	return nil
}

func (p *licenseGateProcessor) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	checker := p.getChecker()
	if checker == nil || checker.IsValid() {
		if checker != nil && checker.IsGracePeriod() {
			p.logger.Warn("License expired — grace period active, metrics still accepted",
				zap.Int("metric_count", md.MetricCount()))
		}
		return p.nextMetrics.ConsumeMetrics(ctx, md)
	}
	count := int64(md.MetricCount())
	p.logger.Error("License expired — metrics dropped",
		zap.Int64("metric_count", count),
		zap.String("tenant_id", checker.TenantID()))
	if p.metrics != nil {
		p.metrics.recordDropped(ctx, count, "metrics")
	}
	return nil
}
