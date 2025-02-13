# Prometheus Mimic

## Configuration

### VictoriaMetrics

```yaml
services:
  vmagent:
    image: victoriametrics/vmagent:latest
    restart: unless-stopped
    volumes:
      - './vmagent-load.dev.yml:/etc/vmagent/prometheus.yml'
    command:
      - '--promscrape.config=/etc/vmagent/prometheus.yml'
      - '--remoteWrite.url=http://prometheus-mimic-gateway:8080/api/v1/write'
      - '--remoteWrite.flushInterval=30s'
      - '--httpListenAddr=:8429'
```

### Prometheus

```yaml
remote_write:
  - url: http://prometheus-mimic-gateway:8080/api/v1/write
```
