// Copyright 2024 Wondermove Inc.
// SPDX-License-Identifier: Apache-2.0

package licensegateprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

// mockChecker implements LicenseChecker for testing
type mockChecker struct {
	valid      bool
	grace      bool
	tenantID   string
	contractID string
}

func (m *mockChecker) IsValid() bool                  { return m.valid }
func (m *mockChecker) IsGracePeriod() bool             { return m.grace }
func (m *mockChecker) ExpiresInDays() float64           { return 365 }
func (m *mockChecker) GracePeriodRemainingDays() float64 { return 0 }
func (m *mockChecker) TenantID() string                 { return m.tenantID }
func (m *mockChecker) ContractID() string               { return m.contractID }

// mockTracesConsumer tracks if ConsumeTraces was called
type mockTracesConsumer struct {
	consumed bool
	count    int
}

func (m *mockTracesConsumer) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	m.consumed = true
	m.count = td.SpanCount()
	return nil
}
func (m *mockTracesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// mockLogsConsumer tracks if ConsumeLogs was called
type mockLogsConsumer struct {
	consumed bool
}

func (m *mockLogsConsumer) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	m.consumed = true
	return nil
}
func (m *mockLogsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

// mockMetricsConsumer tracks if ConsumeMetrics was called
type mockMetricsConsumer struct {
	consumed bool
}

func (m *mockMetricsConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	m.consumed = true
	return nil
}
func (m *mockMetricsConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func TestProcessor_ValidLicense_TracesPass(t *testing.T) {
	next := &mockTracesConsumer{}
	p := &licenseGateProcessor{
		config:     &Config{ExtensionName: "license_guard"},
		logger:     zap.NewNop(),
		nextTraces: next,
		checker:    &mockChecker{valid: true, tenantID: "t1"},
	}

	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	err := p.ConsumeTraces(context.Background(), td)
	assert.NoError(t, err)
	assert.True(t, next.consumed, "traces should pass through when license is valid")
}

func TestProcessor_ExpiredLicense_TracesDropped(t *testing.T) {
	next := &mockTracesConsumer{}
	p := &licenseGateProcessor{
		config:     &Config{ExtensionName: "license_guard"},
		logger:     zap.NewNop(),
		nextTraces: next,
		checker:    &mockChecker{valid: false, tenantID: "t1"},
	}

	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	err := p.ConsumeTraces(context.Background(), td)
	assert.NoError(t, err)
	assert.False(t, next.consumed, "traces should be dropped when license expired")
}

func TestProcessor_GracePeriod_TracesPassWithWarning(t *testing.T) {
	next := &mockTracesConsumer{}
	p := &licenseGateProcessor{
		config:     &Config{ExtensionName: "license_guard"},
		logger:     zap.NewNop(),
		nextTraces: next,
		checker:    &mockChecker{valid: true, grace: true, tenantID: "t1"},
	}

	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	err := p.ConsumeTraces(context.Background(), td)
	assert.NoError(t, err)
	assert.True(t, next.consumed, "traces should pass during grace period")
}

func TestProcessor_NilChecker_PassThrough(t *testing.T) {
	next := &mockTracesConsumer{}
	p := &licenseGateProcessor{
		config:     &Config{ExtensionName: "license_guard"},
		logger:     zap.NewNop(),
		nextTraces: next,
		checker:    nil, // extension not found
	}

	td := ptrace.NewTraces()
	td.ResourceSpans().AppendEmpty().ScopeSpans().AppendEmpty().Spans().AppendEmpty()

	err := p.ConsumeTraces(context.Background(), td)
	assert.NoError(t, err)
	assert.True(t, next.consumed, "traces should pass when checker is nil")
}

func TestProcessor_ValidLicense_LogsPass(t *testing.T) {
	next := &mockLogsConsumer{}
	p := &licenseGateProcessor{
		config:   &Config{ExtensionName: "license_guard"},
		logger:   zap.NewNop(),
		nextLogs: next,
		checker:  &mockChecker{valid: true},
	}

	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err := p.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)
	assert.True(t, next.consumed)
}

func TestProcessor_ExpiredLicense_LogsDropped(t *testing.T) {
	next := &mockLogsConsumer{}
	p := &licenseGateProcessor{
		config:   &Config{ExtensionName: "license_guard"},
		logger:   zap.NewNop(),
		nextLogs: next,
		checker:  &mockChecker{valid: false},
	}

	ld := plog.NewLogs()
	ld.ResourceLogs().AppendEmpty().ScopeLogs().AppendEmpty().LogRecords().AppendEmpty()

	err := p.ConsumeLogs(context.Background(), ld)
	assert.NoError(t, err)
	assert.False(t, next.consumed)
}

func TestProcessor_ValidLicense_MetricsPass(t *testing.T) {
	next := &mockMetricsConsumer{}
	p := &licenseGateProcessor{
		config:      &Config{ExtensionName: "license_guard"},
		logger:      zap.NewNop(),
		nextMetrics: next,
		checker:     &mockChecker{valid: true},
	}

	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()

	err := p.ConsumeMetrics(context.Background(), md)
	assert.NoError(t, err)
	assert.True(t, next.consumed)
}

func TestProcessor_ExpiredLicense_MetricsDropped(t *testing.T) {
	next := &mockMetricsConsumer{}
	p := &licenseGateProcessor{
		config:      &Config{ExtensionName: "license_guard"},
		logger:      zap.NewNop(),
		nextMetrics: next,
		checker:     &mockChecker{valid: false},
	}

	md := pmetric.NewMetrics()
	md.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty().Metrics().AppendEmpty()

	err := p.ConsumeMetrics(context.Background(), md)
	require.NoError(t, err)
	assert.False(t, next.consumed)
}
