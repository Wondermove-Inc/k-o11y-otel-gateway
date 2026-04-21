# K-O11y SigNoz OTel Collector

[English](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) | [中文](README.zh-CN.md)

基于 JWT 许可证验证扩展的自定义 SigNoz OpenTelemetry Collector。

由 [Wondermove](https://wondermove.net) 作为 K-O11y 可观测性栈的一部分开发，基于 [SigNoz OTel Collector](https://github.com/SigNoz/signoz-otel-collector) (Apache 2.0) 构建。

## 自定义组件

本 Collector 在 SigNoz OTel Collector 基础上新增了两个用于许可证数据管控的自定义组件。

### License Guard Extension

基于 JWT 的许可证验证扩展，通过 Prometheus 指标监控许可证状态。

- **RS256 公钥验证**（无需私钥）
- 许可证到期追踪，支持配置宽限期
- 定期重新验证（默认：1 小时）
- Prometheus 指标：`otel_license_valid`、`otel_license_expires_in_days`、`otel_grace_period_remaining_days`
- 未配置许可证时进入透传模式（适用于开发/测试环境）

### License Gate Processor

许可证无效时丢弃遥测数据的处理器。

- 支持 Traces、Logs、Metrics
- 与 License Guard Extension 联动进行状态检查
- 宽限期内：仅记录警告，数据正常通过
- 到期后：丢弃数据并记录指标（`otel_data_dropped_total`）

## 配置

### License Guard Extension

```yaml
extensions:
  license_guard:
    # JWT 许可证密钥（直接值或环境变量）
    license_key_env: "LICENSE_KEY"

    # 用于 JWT 验证的 RSA 公钥（PEM 格式）
    public_key_pem: |
      -----BEGIN PUBLIC KEY-----
      ...
      -----END PUBLIC KEY-----

    # 重新验证间隔（默认：1h）
    check_interval: 1h

    # 到期后的宽限期（默认：7 天）
    grace_period_days: 7

    # 验证失败时的行为："closed"（阻断）或 "open"（放行）
    fail_mode: closed
```

### License Gate Processor

```yaml
processors:
  license_gate:
    # 引用的 License Guard 扩展名称（默认："license_guard"）
    extension_name: license_guard
```

### Pipeline 配置示例

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

## 构建

### 前置条件

- Go 1.22+
- Docker

### 二进制构建

```bash
go build -o signoz-otel-collector ./cmd/signozotelcollector
```

### Docker 镜像构建

```bash
docker build -t ghcr.io/wondermove-inc/signoz-otel-collector:latest \
  -f cmd/signozotelcollector/Dockerfile .
```

### 运行测试

```bash
go test ./extension/licenseguardextension/...
go test ./processor/licensegateprocessor/...
```

## 基础版本

- **SigNoz OTel Collector**: v0.129.2
- **OpenTelemetry Collector**: v0.109.0

## 维护者

由 [Wondermove](https://wondermove.net) 开发并维护。

## 许可证

Apache 2.0 - 参见 [LICENSE](LICENSE)
