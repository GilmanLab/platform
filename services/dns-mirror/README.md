# dns-mirror

`dns-mirror` mirrors a single Route 53 hosted zone to a deterministic zonefile
on disk, serves the latest rendered snapshot over HTTP, and can fetch that
snapshot to another host.

It is designed for the AWS subnet router in the `glab` lab account:

- reads the Route 53 private hosted zone using the instance IAM role
- writes the zonefile atomically to a host-mounted path
- keeps serving the last good snapshot when later syncs fail

## Configuration

Required environment variables:

- `AWS_REGION`
- `DNS_MIRROR_HOSTED_ZONE_ID`
- `DNS_MIRROR_OUTPUT_PATH`

Optional environment variables:

- `DNS_MIRROR_SYNC_INTERVAL` default `1m`
- `DNS_MIRROR_LISTEN_ADDR` default `:8080`
- `DNS_MIRROR_LOG_LEVEL` default `info`

## Endpoints

- `GET /zonefile`
- `GET /healthz`
- `GET /readyz`

## Local usage

```sh
just check
go run ./cmd/dns-mirror --once
go run ./cmd/dns-mirror fetch --source-url http://100.80.89.100:8080/zonefile --output-path /tmp/glab.lol.zone
```
