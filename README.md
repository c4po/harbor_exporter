# Prometheus exporter for Harbor 

Export harbor service health to Prometheus.

To run it:

```bash
make
./harbor_exporter [flags]
```


## Exported Metrics

| Metric | Meaning | Labels |
| ------ | ------- | ------ |
| harbor_scans_completed | | |
| harbor_scans_total | | |
| harbor_scans_requester | | |


### Flags

```bash
./harbor_exporter --help
```



### Environment variables

```
HARBOR_URI
HARBOR_USERNAME
HARBOR_PASSWORD
```

## Using Docker

You can deploy this exporter using the Docker image.

For example:

```bash
docker pull c4po/harbor-exporter

docker run -d -p 9107:9107 -e HARBOR_USERNAME=admin -e HARBOR_PASSWORD=password c4po/harbor-exporter --harbor.server=https://harbor.dev
```

if you deploy Harbor to Kubernetes using the helm chart [goharbor/harbor-helm](https://github.com/goharbor/harbor-helm), you can use this file [kubernetes/harbor-exporter.yaml](kubernetes/harbor-exporter.yaml) to deploy the `harbor-exporter` with `secretKeyRef`

