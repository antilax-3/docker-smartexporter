import express from 'express';
import fs from 'fs';
import Prometheus from 'prom-client';
import smartctl from 'smartctl';

import defConfig from './config/default';

// Attempt to read configuration file
let config;
let metricsTimer;

const copyDefaultConfigFile = (err, copyFile = false) => {
  console.log(err);
  config = defConfig;
  if (copyFile) {
    fs.writeFile('/config/smartexporter.json', `${JSON.stringify(defConfig, null, 2)}\n`, 'utf8', (err) => {
      if (err) {
        return console.log('Error writing to /config/smartexporter.json', err);
      }
      console.log('Copied default config to /config/smartexporter.json');
    });
  }
};

if (fs.existsSync('/config/smartexporter.json')) {
  // Read config file
  const fileContents = fs.readFileSync('/config/smartexporter.json', 'utf8');

  if (fileContents) {
    try {
      config = JSON.parse(fileContents);

      if (!config || !config.attributes) {
        copyDefaultConfigFile('Missing required in from the configuration file... Using defaults for now.');
      }
    } catch (err) {
      copyDefaultConfigFile(`Unable to parse configuration file, please check JSON validity. Using defaults for now. Error: ${err}`);
    }
  }
} else {
  copyDefaultConfigFile('Unable to find configuration file, using defaults.', true);
}

const port = config.port || 9120;
const scrapeInterval = (config.scrapeInterval || 15) * 1000;
const defaultMetrics = Prometheus.collectDefaultMetrics({ timeout: scrapeInterval });

// Generate the prometheus metrics
const prometheusMetrics = config.attributes.map((attribute) => {
  const { attributeName, attributeID, name, help, labelNames } = attribute;
  // Sanitize inputs
  attribute.attributeName = attributeName && attributeName.trim().replace(/ +/g, '_').toLowerCase();
  attribute.attributeID = attributeID && Math.max(Math.min(attributeID, 255), 0);
  attribute.name = name.trim().replace(/ +/g, '_');
  attribute.labelNames = labelNames.map((labelName) => labelName.replace(/ +/g, '_').toLowerCase());

  const reporter = new Prometheus.Gauge({
    name: `smartexporter_${name.toLowerCase()}`,
    help,
    labelNames: attribute.labelNames,
  });

  return { ...attribute, reporter };
});

const getMetrics = () => {
  // Gather smartctl metrics

  // Only care about /dev/sdx devices, this regex will be later used to test
  // against the devices returned by the SMART scan
  const devRegex = RegExp('\/dev\/s.+');

  // Define asynchronous function
  const gatherAsync = async () => {
    // Perform --scan-open
    const devices = await new Promise((res) => {
      smartctl.scan((dev) => {
        // Filter out strings that don't match the regex above
        res(dev.filter(el => devRegex.test(el)));
      });
    });

    // Get devices information
    const getDeviceInfo = await Promise.all(devices.map(async (device) => {
      return await new Promise((resolve, reject) => {
        smartctl.info(device, (err, info) => {
          if (err) return reject(err);
          return resolve({ info: { device, ...info } });
        });
      });
    }));

    // Get devices attributes
    const getDeviceAttributes = await Promise.all(getDeviceInfo.map(async (device) => {
      return await new Promise((resolve, reject) => {
        smartctl.smartAttrs(device.info.device, (err, info) => {
          if (err) return reject(err);
          return resolve({ ...device, attributes: info });
        });
      });
    }));

    // For each device concatenate info and append
    for (let device of getDeviceAttributes) {
      prometheusMetrics.forEach((metric) => {
        // Bail if this metric isn't required
        if (!metric || (!metric.attributeName && !metric.attributeID)) return;

        // Find the attribute name in the array of attributes
        const attribute = device.attributes.find((attribute) => attribute.id === metric.attributeID || attribute.attr === metric.attributeName);
        if (attribute === undefined) return;

        // Map label names to device info
        const labels = {};
        metric.labelNames.map((labelName) => {
          labels[labelName] = device.info[labelName];
        });

        // Update the metric
        metric.reporter.set(labels, attribute.raw);
      });
    }
  };

  // Invoke the async function
  gatherAsync()
    .catch((err) => {
      console.log(err);
    });
};

// HTTP Server
const app = express();
app.get('/', (req, res, next) => {
  setTimeout(() => {
    res.send('Point Prometheus here for your HDD statistics');
    next();
  }, Math.round(Math.random() * 200));
});

app.get('/metrics', (req, res) => {
  res.set('Content-Type', Prometheus.register.contentType);
  res.end(Prometheus.register.metrics());
});

app.use((err, req, res, next) => {
  res.statusCode = 500;
  next();
});

const server = app.listen((port), () => {
  // Start a timer to fetch metrics
  metricsTimer = setInterval(getMetrics, scrapeInterval);

  // Get initial metrics
  getMetrics();

  console.log(`Running smartexporter. Listening on port ${port}.`);
});

// Shutdown gracefully
process.on('SIGTERM', () => {
  clearInterval(defaultMetrics);
  clearInterval(metricsTimer);

  server.close((err) => {
    if (err) {
      console.log(err);
      process.exit(1);
    }

    process.exit(0);
  });
});