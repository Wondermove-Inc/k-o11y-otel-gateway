# K-O11y SigNoz OTel Collector

[English](README.md)

JWT 기반 라이선스 검증 확장을 포함한 커스텀 SigNoz OpenTelemetry Collector입니다.


[Wondermove](https://wondermove.net)가 K-O11y 스택의 일부로 개발했으며, [SigNoz OTel Collector](https://github.com/SigNoz/signoz-otel-collector) (Apache 2.0)를 기반으로 합니다.

## 커스텀 컴포넌트

SigNoz OTel Collector에 라이선스 기반 데이터 게이팅을 위한 2개의 커스텀 컴포넌트를 추가했습니다:

### License Guard Extension

JWT 기반 라이선스 검증 확장으로 Prometheus 메트릭을 통해 라이선스 상태를 모니터링합니다.

- **RS256 공개키 검증** (비공개키 불필요)
- 라이선스 만료 추적 + 설정 가능한 유예 기간
- 정기 재검증 (기본: 1시간)
- Prometheus 메트릭: `otel_license_valid`, `otel_license_expires_in_days`, `otel_grace_period_remaining_days`
- 라이선스 미설정 시 통과 모드 (개발/테스트용)

### License Gate Processor

라이선스가 무효할 때 텔레메트리 데이터를 드롭하는 프로세서입니다.

- Traces, Logs, Metrics 모두 처리
- License Guard Extension과 연동하여 상태 확인
- 유예 기간: 경고만 기록하고 데이터 통과
- 만료 후: 데이터 드롭 + 메트릭 기록 (`otel_data_dropped_total`)

## 설정

### License Guard Extension

```yaml
extensions:
  license_guard:
    # JWT 라이선스 키 (직접 값 또는 환경변수)
    license_key_env: "LICENSE_KEY"

    # JWT 검증용 RSA 공개키 (PEM 형식)
    public_key_pem: |
      -----BEGIN PUBLIC KEY-----
      ...
      -----END PUBLIC KEY-----

    # 재검증 주기 (기본: 1h)
    check_interval: 1h

    # 만료 후 유예 기간 (기본: 7일)
    grace_period_days: 7

    # 검증 실패 시 동작: "closed" (차단) 또는 "open" (허용)
    fail_mode: closed
```

### License Gate Processor

```yaml
processors:
  license_gate:
    # 참조할 License Guard 확장 이름 (기본: "license_guard")
    extension_name: license_guard
```

### 파이프라인 예시

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

## 빌드

### 필수 조건

- Go 1.22+
- Docker

### 바이너리 빌드

```bash
go build -o signoz-otel-collector ./cmd/signozotelcollector
```

### Docker 이미지 빌드

```bash
docker build -t ghcr.io/wondermove-inc/signoz-otel-collector:latest \
  -f cmd/signozotelcollector/Dockerfile .
```

### 테스트 실행

```bash
go test ./extension/licenseguardextension/...
go test ./processor/licensegateprocessor/...
```

## 기반 버전

- **SigNoz OTel Collector**: v0.129.2
- **OpenTelemetry Collector**: v0.109.0

## 관리

[Wondermove](https://wondermove.net)가 개발 및 관리합니다.

## 라이선스

Apache 2.0 - [LICENSE](LICENSE) 참조
