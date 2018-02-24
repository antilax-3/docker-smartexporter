[logo]: https://ci.nerv.com.au/userContent/antilax-3.png "AntilaX-3"
[![alt text][logo]](https://github.com/AntilaX-3/)

# AntilaX-3/smart-exporter
[![](https://images.microbadger.com/badges/version/antilax3/smart-exporter.svg)](https://microbadger.com/images/antilax3/smart-exporter "Get your own version badge on microbadger.com") [![](https://images.microbadger.com/badges/image/antilax3/smart-exporter.svg)](https://microbadger.com/images/antilax3/smart-exporter "Get your own image badge on microbadger.com") [![Docker Pulls](https://img.shields.io/docker/pulls/antilax3/smart-exporter.svg)](https://hub.docker.com/r/antilax3/smart-exporter/) [![Docker Stars](https://img.shields.io/docker/stars/antilax3/smart-exporter.svg)](https://hub.docker.com/r/antilax3/smart-exporter/)

[smart-exporter](https://github.com/AntilaX-3/docker-smartexporter) is a simple server that periodically scrapes S.M.A.R.T stats and exports them via HTTP for Prometheus consumption, written in Node.js.
The attributes it supplies to Prometheus are configurable, as well as the labels it supplies. 
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
- `-p 9120` - HTTP port for webserver
- `-e PUID` - for UserID, see below for explanation
- `-e PGID` - for GroupID, see below for explanation
- `-e TZ` - for setting timezone information, eg Australia/Melbourne

It is based on alpine linux with s6 overlay, for shell access whilst the container is running do `docker exec -it smartexporter /bin/bash`.

## User / Group Identifiers
Sometimes when using data volumes (-v flags) permissions issues can arise between the host OS and the container. We avoid this issue by allowing you to specify the user `PUID` and group `PGID`. Ensure the data volume directory on the host is owned by the same user you specify and it will "just work".

In this instance `PUID=1001` and `PGID=1001`. To find yours use `id user` as below:
`$ id <dockeruser>`
    `uid=1001(dockeruser) gid=1001(dockergroup) groups=1001(dockergroup)`
    
## Volumes

The container uses a single volume mounted at '/config'. This volume stores the configuration file 'smartexporter.json'.

    config
    |-- smartexporter.json

## Configuration

The smartexporter.json is copied to the /config volume when first run. It has two parameters, one optional and one mandatory.

The optional parameter is:
 - scrapeInterval (default 10 seconds)
 
The mandatory parameter *reportedAttributes* is an array of objects. The objects define the SMART attributes that will be parsed and reported. The [default file](https://github.com/AntilaX-3/docker-smartexporter/blob/master/root/app/src/config/default.json) has examples. 
 
 **Only one of either attributeID or attributeName is required.**
 
    attributeID: Number | The attribute ID
    attributeName: String | The attribute name
    name: String (Required) | The name reported to Prometheus, prepended with 'smartexporter_'
    help: String (Required) | Help text provided to Prometheus
    labelNames: Array of Strings | Mapped to data from the information section of smartctl. Can be used for labels, ie "Device" for /dev/sdx or "Serial Number" for the serial number of the HDD. 
     
[Known S.M.A.R.T. attributes (Wikipedia)](https://en.wikipedia.org/w/index.php?title=S.M.A.R.T.#Known_ATA_S.M.A.R.T._attributes)
## Version
- **24/02/18:** Updated to use alpine 3.7 image and build with jenkins
- **24/01/18:** Corrected documentation
- **24/01/18:** Refactoring & Cleaning
- **23/01/18:** Initial Release