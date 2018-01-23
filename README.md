# AntilaX-3/smartexporter
[smartexporter](https://github.com/AntilaX-3/docker-smartexporter) is a simple server that periodically scrapes S.M.A.R.T stats and exports them via HTTP for Prometheus consumption, written in Node.js.
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
- `-p 9120` - http port for webserver

It is based on alpine linux, utilising the official node docker repository with alpine tag, for shell access whilst the container is running do `docker exec -it smartexporter /bin/bash`.

## Volumes

The container uses a single volume mounted at '/config'. This volume stores the configuration file 'smartexporter.json'.

    smartexporter
    |-- smartexporter.json

## Configuration

The smartexporter.json is copied to the /config volume when first run. It has two optional parameters and one required. 

The optional parameters are:
 - port (default 9120)
 - scrapeInterval (default 10 seconds)
 
 The required parameter *reportedAttributes* is an array of objects. The objects define the SMART attributes that will be parsed and reported. The [default file](https://github.com/AntilaX-3/docker-smartexporter/blob/master/root/app/src/config/default.json) has examples. **Only one of either attributeID or attributeName is required.** 
 
    attributeID: Number | The attribute ID
    attributeName: String | The attribute name
    name: String (Required) | The name reported to Prometheus, prepended with 'smartexporter_'
    help: String (Required) | Help text provided to Prometheus
    labelNames: Array of Strings | Mapped to data from the information section of smartctl. Can be used for labels, ie "Device" for /dev/sdx or "Serial Number" for the serial number of the HDD. 
     
[Known S.M.A.R.T. attributes (Wikipedia)](https://en.wikipedia.org/w/index.php?title=S.M.A.R.T.#Known_ATA_S.M.A.R.T._attributes)
## Version
- **24/01/18:** 1.0.1 | Refactoring & Cleaning
- **23/01/18:** 1.0.0 | Initial Release