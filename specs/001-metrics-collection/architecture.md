# Metrics Architecture Diagram

```mermaid
flowchart LR
  %% Clients
  subgraph Clients
    U1[Reporting UI / scripts]
    U2[Analysts / automation]
  end

  %% OSCAR API
  subgraph OSCAR["OSCAR API (Gin)"]
    R1[/GET /system/metrics/{serviceName}/]
    R2[/GET /system/metrics/]
    R3[/GET /system/metrics/breakdown/]
    H1[MetricValueHandler]
    H2[MetricsSummaryHandler]
    H3[MetricsBreakdownHandler]
    CSV[renderBreakdownCSV]
  end

  U1 --> R1 --> H1
  U1 --> R2 --> H2
  U2 --> R3 --> H3
  H3 -->|format=csv| CSV

  %% Aggregation layer
  subgraph Aggregation["pkg/metrics.Aggregator"]
    A1[Aggregator.Value]
    A2[Aggregator.Summary]
    A3[Aggregator.Breakdown]
    A4[Source status / completeness flags]
  end

  H1 --> A1
  H2 --> A2
  H3 --> A3
  A1 --> A4
  A2 --> A4
  A3 --> A4

  %% Sources
  subgraph Sources["Data Sources (pkg/metrics/sources.go)"]
    S1[ServiceInventorySource]
    S2[UsageMetricsSource]
    S3[RequestLogSource]
    S6[ExposedRequestLogSource]
    S4[CountryAttributionSource]
    S5[UserRosterSource]
  end

  A1 --> S2
  A2 --> S1
  A2 --> S2
  A2 --> S3
  A2 --> S6
  A2 --> S4
  A3 --> S3
  A3 --> S4
  A3 --> S5

  %% External systems
  subgraph External["External Systems"]
    K8s[Kubernetes API]
    Prom[Prometheus HTTP API]
    Loki[Loki HTTP API]
    GeoIPDB["GeoIP DB (GeoLite2)"]
    PodLogs["Pod logs fallback"]
    IngressLogs["Ingress controller logs"]
    OIDC[OIDC / request metadata]
    Roster[User roster source]
  end

  S1 --> K8s
  S2 --> Prom
  S3 --> Loki
  S6 --> Loki
  S3 -.fallback .-> PodLogs
  S6 -.fallback .-> IngressLogs
  S4 --> OIDC
  S5 --> Roster

  %% Log enrichment
  subgraph LogPipeline["Alloy Log Pipeline"]
    LP1[loki.source.kubernetes]
    LP2["loki.process: regex + geoip"]
    LP3[loki.write]
    LP4[loki.source.kubernetes ingress]
  end

  PodLogs --> LP1 --> LP2 --> LP3 --> Loki
  IngressLogs --> LP4 --> LP3
  GeoIPDB --> LP2

  %% Config
  subgraph Config["Config & Env Vars"]
    C1[PROMETHEUS_URL]
    C2[PROMETHEUS_CPU_QUERY]
    C3[PROMETHEUS_GPU_QUERY]
    C4[LOKI_URL]
    C5[LOKI_QUERY]
    C6[LOKI_EXPOSED_QUERY]
    C7[LOKI_EXPOSED_NAMESPACE]
    C8[LOKI_EXPOSED_APP]
  end

  C1 --> S2
  C2 --> S2
  C3 --> S2
  C4 --> S3
  C5 --> S3
  C4 --> S6
  C6 --> S6
  C7 --> S6
  C8 --> S6

  %% Outputs
  subgraph Outputs["Responses"]
    O1[MetricValueResponse]
    O2[MetricsSummaryResponse]
    O3[MetricsBreakdownResponse]
    O4[CSV export]
  end

  A1 --> O1
  A2 --> O2
  A3 --> O3
  CSV --> O4
```
