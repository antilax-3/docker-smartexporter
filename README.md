# AntilaX-3/smartexporter
[smartexporter](https://github.com/AntilaX-3/docker-smartexporter) is a simple server that periodically scrapes S.M.A.R.T stats and exports them via HTTP for Prometheus consumption, written in Node.js.
## Usage
```
docker create --name=smartexporter \
-v <path to config>:/config \
-p 9120:9120 \
--privileged=true \
antilax3/smart-exporter
```
## Parameters
The parameters are split into two halves, separated by a colon, the left hand side representing the host and the right the container side. For example with a volume -v external:internal - what this shows is the volume mapping from internal to external of the container. So -v /mnt/app/config:/config would map /config from inside the container to be accessible from /mnt/app/config on the host's filesystem.

- `-v /config` - local path for smartexporter config file
- `-p 9120` - http port for webserver

It is based on alpine linux, utilising the official node docker repository with alpine tag, for shell access whilst the container is running do `docker exec -it smartexporter /bin/bash`.
## Version
- **23/01/18:** Initial Release