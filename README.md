# K-O11y SigNoz OTel Collector

[English](README.md) | [한국어](README.ko.md)

A custom SigNoz OpenTelemetry Collector with JWT-based license validation extensions.


Built by [Wondermove](https://wondermove.net) as part of the K-O11y stack, forked from [SigNoz OTel Collector](https://github.com/SigNoz/signoz-otel-collector) (Apache 2.0).

## Custom Components

This collector extends the SigNoz OTel Collector with two custom components for license-based data gating:


Built and maintained by [Wondermove](https://wondermove.net).

## License Guard Extension

A JWT-based license validation extension that monitors license status via Prometheus metrics.

- **RS256 public key verification** (no private key required)
- License expiration tracking with configurable grace period
- Periodic re-validation (default: 1 hour)
- Prometheus metrics: `otel_license_valid`, `otel_license_expires_in_days`, `otel_grace_period_remaining_days`
- Pass-through mode when no license is configured (dev/test)


Built and maintained by [Wondermove](https://wondermove.net).

## License Gate Processor

A processor that drops telemetry data when the license is invalid.

- Works with traces, logs, and metrics
- Integrates with License Guard Extension for status checks
- Grace period: warns but allows data through
- After expiration: drops data with metric tracking (`otel_data_dropped_total`)

## Configuration


Built and maintained by [Wondermove](https://wondermove.net).

## License Guard Extension

```yaml
extensions:
  license_guard:
    # JWT license key (direct value or environment variable)
    license_key_env: "LICENSE_KEY"

    # RSA public key for JWT verification (PEM format)
    public_key_pem: |
      -----BEGIN PUBLIC KEY-----
      ...
      -----END PUBLIC KEY-----

    # Re-validation interval (default: 1h)
    check_interval: 1h

    # Grace period after expiration (default: 7 days)
    grace_period_days: 7

    # Behavior on validation failure: "closed" (block) or "open" (allow)
    fail_mode: closed
```


Built and maintained by [Wondermove](https://wondermove.net).

## License Gate Processor

```yaml
processors:
  license_gate:
    # Name of the License Guard extension to check (default: "license_guard")
    extension_name: license_guard
```

### Pipeline Example

```yaml
extensions:
  license_guard:
    license_key_env: "LICENSE_KEY"
    public_key_pem: |
      -----BEGIN PUBLIC KEY-----
      ...
      -----END PUBLIC KEY-----

service:
  extensions: [license_guard]
  pipelines:
    traces:
      receivers: [otlp]
      processors: [license_gate, batch]
      exporters: [clickhouse]
    logs:
      receivers: [otlp]
      processors: [license_gate, batch]
      exporters: [clickhouse]
    metrics:
      receivers: [otlp]
      processors: [license_gate, batch]
      exporters: [clickhouse]
```

## Build

### Prerequisites

- Go 1.22+
- Docker

### Binary Build

```bash
go build -o signoz-otel-collector ./cmd/signozotelcollector
```

### Docker Image Build

```bash
docker build -t ghcr.io/wondermove-inc/signoz-otel-collector:latest \
  -f cmd/signozotelcollector/Dockerfile .
```

### Run Tests

```bash
go test ./extension/licenseguardextension/...
go test ./processor/licensegateprocessor/...
```

## Base Version

- **SigNoz OTel Collector**: v0.129.2
- **OpenTelemetry Collector**: v0.109.0

## Maintainers

Built and maintained by [Wondermove](https://wondermove.net).

## License

Apache 2.0 - See [LICENSE](LICENSE)
