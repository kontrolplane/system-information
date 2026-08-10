# system-information

a tiny server that reports the system information of the machine it runs on. it is the reference application for the [kontrolplane](https://github.com/kontrolplane/konfig) fleet, but for checking a virtual machine rather than a kubernetes pod. `konfig` deploys it onto a machine; you hit `/` to confirm the host converged as expected. standard library only, no third-party dependencies.

## endpoints

| method | path              | description                                                     |
| ------ | ----------------- | --------------------------------------------------------------- |
| get    | `/`               | system information as json (see below)                          |
| get    | `/healthz`        | liveness — `200 OK` while the process is up                     |
| get    | `/readyz`         | readiness — `200` when ready, `503` when disabled               |
| post   | `/readyz/{state}` | `enable` / `disable` readiness (to exercise the readiness probe) |
| get    | `/version`        | `{version, commit, go}`                                         |
| get    | `/metrics`        | prometheus text exposition                                      |

`GET /` reports: version, commit, go version, hostname, os, arch, cpu count, and kernel, distro, uptime, load averages, and memory/disk totals. off linux (e.g. a macos dev box) the linux-only fields read zero.

## run

```sh
go run ./cmd/system-information
curl -s localhost:9898/ | jq

docker build -t system-information .
docker run -p 9898:9898 system-information
```

flags: `-port` (default `9898`).

## deployed by konfig

the `application` profile in [`konfig-state`](https://github.com/kontrolplane/konfig-state) fetches a release tarball, unpacks it to `/opt/system-information/<version>/`, and runs it as a systemd service on `:8080`. releases are cut by tagging `vX.Y.Z`, which triggers `.github/workflows/release.yml` to publish `system-information_<tag>_linux_<arch>.tar.gz` assets — the layout the profile's `archive` resource expects.
