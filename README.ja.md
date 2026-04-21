# K-O11y SigNoz OTel Collector

[English](README.md) | [한국어](README.ko.md) | [日本語](README.ja.md) | [中文](README.zh-CN.md)

JWT ベースのライセンス検証拡張を含む、カスタム SigNoz OpenTelemetry Collector です。

[Wondermove](https://wondermove.net) が K-O11y スタックの一部として開発しており、[SigNoz OTel Collector](https://github.com/SigNoz/signoz-otel-collector) (Apache 2.0) をベースとしています。

## カスタムコンポーネント

このコレクターは、ライセンスベースのデータゲーティングのために SigNoz OTel Collector に 2 つのカスタムコンポーネントを追加しています。

### License Guard Extension

Prometheus メトリクスを通じてライセンス状態を監視する、JWT ベースのライセンス検証拡張です。

- **RS256 公開鍵検証**（秘密鍵は不要）
- ライセンスの有効期限追跡と設定可能なグレースピリオド
- 定期的な再検証（デフォルト: 1 時間）
- Prometheus メトリクス: `otel_license_valid`、`otel_license_expires_in_days`、`otel_grace_period_remaining_days`
- ライセンス未設定時のパススルーモード（開発・テスト用）

### License Gate Processor

ライセンスが無効な場合にテレメトリデータをドロップするプロセッサーです。

- トレース、ログ、メトリクスに対応
- License Guard Extension と連携してライセンス状態を確認
- グレースピリオド中: 警告のみ記録し、データは通過させます
- 有効期限切れ後: データをドロップし、メトリクスに記録（`otel_data_dropped_total`）

## 設定

### License Guard Extension

```yaml
extensions:
  license_guard:
    # JWT ライセンスキー（直接値または環境変数）
    license_key_env: "LICENSE_KEY"

    # JWT 検証用 RSA 公開鍵（PEM 形式）
    public_key_pem: |
      -----BEGIN PUBLIC KEY-----
      ...
      -----END PUBLIC KEY-----

    # 再検証の間隔（デフォルト: 1h）
    check_interval: 1h

    # 有効期限後のグレースピリオド（デフォルト: 7 日）
    grace_period_days: 7

    # 検証失敗時の動作: "closed"（ブロック）または "open"（許可）
    fail_mode: closed
```

### License Gate Processor

```yaml
processors:
  license_gate:
    # 参照する License Guard 拡張の名前（デフォルト: "license_guard"）
    extension_name: license_guard
```

### パイプライン設定例

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

## ビルド

### 前提条件

- Go 1.22+
- Docker

### バイナリビルド

```bash
go build -o signoz-otel-collector ./cmd/signozotelcollector
```

### Docker イメージビルド

```bash
docker build -t ghcr.io/wondermove-inc/signoz-otel-collector:latest \
  -f cmd/signozotelcollector/Dockerfile .
```

### テスト実行

```bash
go test ./extension/licenseguardextension/...
go test ./processor/licensegateprocessor/...
```

## ベースバージョン

- **SigNoz OTel Collector**: v0.129.2
- **OpenTelemetry Collector**: v0.109.0

## メンテナー

[Wondermove](https://wondermove.net) が開発・管理しています。

## ライセンス

Apache 2.0 - [LICENSE](LICENSE) を参照してください。
