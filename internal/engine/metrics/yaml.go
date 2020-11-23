package metrics

// YAML copied from
// https://github.com/tilt-dev/tilt-local-metrics

const collector = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tilt-local-metrics-collector
  labels:
    app.kubernetes.io/name: otel-collector
    app.kubernetes.io/part-of: tilt-local-metrics
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: otel-collector
      app.kubernetes.io/part-of: tilt-local-metrics
  template:
    metadata:
      labels:
        app.kubernetes.io/name: otel-collector
        app.kubernetes.io/part-of: tilt-local-metrics
    spec:
      volumes:
        - name: config-vol
          configMap:
            name: tilt-otel-collector-config
            items:
              - key: otel-collector-config
                path: otel-collector-config.yaml

      containers:
        - name: otel-collector
          image: otel/opentelemetry-collector:0.14.0
          command:
          - "/otelcol"
          - "--config=/conf/otel-collector-config.yaml"

          ports:
            - name: pprof
              containerPort: 1777
              protocol: TCP
            - name: zpages
              containerPort: 55679
              protocol: TCP
            - name: receiver
              containerPort: 55678
              protocol: TCP
            - name: prom-exporter
              containerPort: 55681
              protocol: TCP
            - name: healthcheck
              containerPort: 13133
              protocol: TCP
          volumeMounts:
            - name: config-vol
              mountPath: /conf

          readinessProbe:
            httpGet:
              port: 13133
            initialDelaySeconds: 5
            periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: tilt-local-metrics-collector
  labels:
    app.kubernetes.io/name: otel-collector
    app.kubernetes.io/part-of: tilt-local-metrics
spec:
  ports:
  - name: receiver
    port: 55678
    protocol: TCP
    targetPort: 55678
  - name: exporter
    port: 55681
    protocol: TCP
    targetPort: 55681
  selector:
    app.kubernetes.io/name: otel-collector
    app.kubernetes.io/part-of: tilt-local-metrics
`

const collectorConfig = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: tilt-otel-collector-config
  labels:
    app.kubernetes.io/name: otel-collector
    app.kubernetes.io/part-of: tilt-local-metrics
data:
  otel-collector-config: |
    extensions:
      health_check:
      pprof:
        endpoint: 0.0.0.0:1777
      zpages:
        endpoint: 0.0.0.0:55679

    receivers:
      opencensus:
        endpoint: "0.0.0.0:55678"

    processors:
      memory_limiter:
        check_interval: 5s
        limit_mib: 4000
        spike_limit_mib: 500
      batch:

    exporters:
      logging:
        loglevel: info
      prometheus:
        endpoint: "0.0.0.0:55681"

    service:
      extensions: [health_check, pprof, zpages]
      pipelines:
        metrics:
          receivers: [opencensus]
          exporters: [logging, prometheus]
`

const prometheus = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tilt-local-metrics-prometheus
  labels:
    app.kubernetes.io/name: prometheus
    app.kubernetes.io/part-of: tilt-local-metrics
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: prometheus
      app.kubernetes.io/part-of: tilt-local-metrics
  template:
    metadata:
      labels:
        app.kubernetes.io/name: prometheus
        app.kubernetes.io/part-of: tilt-local-metrics
    spec:
      volumes:
        - name: config-vol
          configMap:
            name: tilt-prometheus-config
            items:
              - key: prometheus-config
                path: prometheus.yml

      containers:
        - name: prometheus
          image: prom/prometheus:v2.22.2

          ports:
            - name: http
              containerPort: 9090
              protocol: TCP

          volumeMounts:
            - name: config-vol
              mountPath: /etc/prometheus

          readinessProbe:
            httpGet:
              port: 9090
            initialDelaySeconds: 5
            periodSeconds: 10
---
apiVersion: v1
kind: Service
metadata:
  name: tilt-local-metrics-prometheus
  labels:
    app.kubernetes.io/name: prometheus
    app.kubernetes.io/part-of: tilt-local-metrics
spec:
  ports:
  - name: http
    port: 9090
    protocol: TCP
    targetPort: 9090
  selector:
    app.kubernetes.io/name: prometheus
    app.kubernetes.io/part-of: tilt-local-metrics
`

const prometheusConfig = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: tilt-prometheus-config
  labels:
    app.kubernetes.io/name: prometheus
    app.kubernetes.io/part-of: tilt-local-metrics
data:
  prometheus-config: |
    global:
      scrape_interval: 15s

    scrape_configs:
    - job_name: 'otel-collector'
      scrape_interval: 5s
      static_configs:
        - targets: ['tilt-local-metrics-collector:55681']
`

const grafana = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tilt-local-metrics-grafana
  labels:
    app.kubernetes.io/name: grafana
    app.kubernetes.io/part-of: tilt-local-metrics
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: grafana
      app.kubernetes.io/part-of: tilt-local-metrics
  template:
    metadata:
      labels:
        app.kubernetes.io/name: grafana
        app.kubernetes.io/part-of: tilt-local-metrics
    spec:
      volumes:
        - name: ini-vol
          configMap:
            name: tilt-grafana-config
            items:
            - key: grafana.ini
              path: grafana.ini
        - name: provision-dashboards-vol
          configMap:
            name: tilt-grafana-config
            items:
            - key: dashboards.yaml
              path: dashboards.yaml
        - name: provision-datasources-vol
          configMap:
            name: tilt-grafana-config
            items:
            - key: datasource-prometheus.yaml
              path: datasource-prometheus.yaml
        - name: dashboard-vol
          configMap:
            name: tilt-grafana-dashboards

      containers:
        - name: grafana
          image: grafana/grafana:7.3.3

          ports:
            - name: http
              containerPort: 3000
              protocol: TCP

          volumeMounts:
            - name: ini-vol
              mountPath: /etc/grafana/grafana.ini
              subPath: grafana.ini
            - name: provision-dashboards-vol
              mountPath: /etc/grafana/provisioning/dashboards/dashboards.yaml
              subPath: dashboards.yaml
            - name: provision-datasources-vol
              mountPath: /etc/grafana/provisioning/datasources/datasource-prometheus.yaml
              subPath: datasource-prometheus.yaml
            - name: dashboard-vol
              mountPath: /var/lib/grafana/dashboards

          readinessProbe:
            httpGet:
              port: 3000
            initialDelaySeconds: 5
            periodSeconds: 10
`

const grafanaConfig = `
kind: ConfigMap
apiVersion: v1
data:
  dashboards.yaml: |
    apiVersion: 1

    providers:
    - name: 'default-dashboards'
      orgId: 1
      type: file
      disableDeletion: true
      updateIntervalSeconds: 1000
      allowUiUpdates: true
      options:
        path: /var/lib/grafana/dashboards
        foldersFromFilesStructure: true
  datasource-prometheus.yaml: |
    apiVersion: 1

    datasources:
    - name: Prometheus
      type: prometheus
      url: http://tilt-local-metrics-prometheus:9090/
      default: true
  grafana.ini: |+
    [auth.anonymous]
    enabled = true
`

const grafanaDashboardConfig = `
kind: ConfigMap
apiVersion: v1
data:
  tiltfile-execution.json: |
    {
      "annotations": {
        "list": [
          {
            "builtIn": 1,
            "datasource": "-- Grafana --",
            "enable": true,
            "hide": true,
            "iconColor": "rgba(0, 211, 255, 1)",
            "name": "Annotations & Alerts",
            "type": "dashboard"
          }
        ]
      },
      "editable": true,
      "gnetId": null,
      "graphTooltip": 0,
      "id": 1,
      "links": [],
      "panels": [
        {
          "aliasColors": {
            "Rolling 15 minute average ": "yellow"
          },
          "bars": false,
          "dashLength": 10,
          "dashes": false,
          "datasource": "Prometheus",
          "description": "",
          "fieldConfig": {
            "defaults": {
              "custom": {}
            },
            "overrides": []
          },
          "fill": 1,
          "fillGradient": 0,
          "gridPos": {
            "h": 9,
            "w": 12,
            "x": 0,
            "y": 0
          },
          "hiddenSeries": false,
          "id": 2,
          "legend": {
            "alignAsTable": false,
            "avg": false,
            "current": false,
            "hideEmpty": false,
            "hideZero": true,
            "max": false,
            "min": false,
            "rightSide": false,
            "show": true,
            "total": false,
            "values": false
          },
          "lines": true,
          "linewidth": 1,
          "nullPointMode": "connected",
          "options": {
            "alertThreshold": false
          },
          "percentage": false,
          "pluginVersion": "7.3.3",
          "pointradius": 2,
          "points": true,
          "renderer": "flot",
          "seriesOverrides": [],
          "spaceLength": 10,
          "stack": false,
          "steppedLine": false,
          "targets": [
            {
              "expr": "rate(tiltfile_exec_duration_dist_sum[60s]) / rate(tiltfile_exec_duration_dist_count[60s])",
              "instant": false,
              "interval": "",
              "intervalFactor": 1,
              "legendFormat": "Rolling 5 min avg (Error: {{error}})",
              "refId": "B"
            }
          ],
          "thresholds": [],
          "timeFrom": null,
          "timeRegions": [],
          "timeShift": null,
          "title": "Tiltfile Execution (ms)",
          "tooltip": {
            "shared": true,
            "sort": 0,
            "value_type": "individual"
          },
          "type": "graph",
          "xaxis": {
            "buckets": null,
            "mode": "time",
            "name": null,
            "show": true,
            "values": []
          },
          "yaxes": [
            {
              "format": "ms",
              "label": "",
              "logBase": 1,
              "max": null,
              "min": "0",
              "show": true
            },
            {
              "format": "short",
              "label": null,
              "logBase": 1,
              "max": null,
              "min": null,
              "show": true
            }
          ],
          "yaxis": {
            "align": true,
            "alignLevel": null
          }
        }
      ],
      "schemaVersion": 26,
      "style": "dark",
      "tags": [],
      "templating": {
        "list": []
      },
      "time": {
        "from": "now-15m",
        "to": "now"
      },
      "timepicker": {},
      "timezone": "",
      "title": "Tilt Local Metrics",
      "uid": "nIq4P-TMz",
      "version": 2
    }
`
