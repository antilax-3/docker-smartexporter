import express from 'express';
import Prometheus from 'prom-client';
import smartctl from 'smartctl';
import config from 'config';
import fs from 'fs';
import defConfig from './config/default';

const app = express();
const port = config.has('port') && config.get('port') || 9120;
const scrapeInterval = (config.has('scrapeInterval') && config.get('scrapeInterval') || 15) * 1000;

let metricsTimer;
const defaultMetrics = Prometheus.collectDefaultMetrics({ timeout: scrapeInterval });

let reportedAttributes;

if (!config.has('reportedAttributes')) {
  // Copy over default configuration file
  console.log('Unable to find configuration file, using defaults');
  reportedAttributes = defConfig.reportedAttributes;
  // Copy default to /config
  fs.writeFile('/config/smartexporter.json', JSON.stringify(defConfig), 'utf8', (err) => {
    if (err) {
      return console.log('Error writing to /config/smartexporter.json', err);
    }
    console.log('Copied default config to /config/smartexporter.json');
  });
} else {
  reportedAttributes = config.get('reportedAttributes');
}

// Generate the prometheus metrics
const prometheusMetrics = reportedAttributes.map((attribute) => {
  const { name, help, labelNames } = attribute;
  const reporter = new Prometheus.Gauge({ name: `smartexporter_${name}`, help, labelNames });
  return { ...attribute, reporter };
});

app.get('/', (req, res, next) => {
  setTimeout(() => {
    res.send('Hi!');
    next();
  }, Math.round(Math.random() * 200));
});

app.get('/metrics', (req, res) => {
  res.set('Content-Type', Prometheus.register.contentType);
  res.end(Prometheus.register.metrics());
});

app.use((err, req, res, next) => {
  res.statusCode = 500;

  // Dev only:
  //res.json({ error: err.message });
  next();
});

const getMetrics = () => {
  // Gather smartctl metrics

  // We only care about /dev/sdx devices, this regex will be later used to test
  // against the devices returned by the SMART scan
  const devRegex = RegExp('\/dev\/s.+');

  // deviceInfo is an array of our devices including their information
  //const deviceInfo = [];

  const gatherAsync = async () => {
    // Perform --scan-open
    const devices = await new Promise((res) => {
      smartctl.scan((dev) => {
        // Filter out strings that don't match the regex above
        res(dev.filter(el => devRegex.test(el)));
      });
    });

    // devices is an array of strings
    const getDeviceInfo = await Promise.all(devices.map(async (device) => {
      return await new Promise((resolve, reject) => {
        smartctl.info(device, (err, info) => {
          if (err) return reject(err);
          return resolve({ info: { device, ...info } });
        });
      });
    }));

    const getDeviceAttributes = await Promise.all(getDeviceInfo.map(async (device) => {
      return await new Promise((resolve, reject) => {
        smartctl.smartAttrs(device.info.device, (err, info) => {
          if (err) return reject(err);
          return resolve({ ...device, attributes: info });
        });
      });
    }));

    // Get attributes info and append
    for (let device of getDeviceAttributes) {
      // console.log('Device:', device.device);
      // console.log('Info:', device.info);
      // console.log('Attributes:', device.attributes);

      prometheusMetrics.forEach((metric) => {
        if (!metric || !metric.attributeName) return;

        // Find the attribute name in the array of attributes
        const attribute = device.attributes.find((attribute) => attribute.attr === metric.attributeName);

        if (attribute === undefined) return;

        // console.log(`Metric ${metric.name} has a raw value of ${attribute.raw}`);

        // Update the metric
        const labels = {};

        // Map label names to device info
        metric.labelNames.map((labelName) => {
          labels[labelName] = device.info[labelName];
        });

        metric.reporter.set(labels, attribute.raw);
      });
    }
  };

  gatherAsync()
    .catch((err) => {
      console.log(err);
    });
};


const server = app.listen((port), () => {
  console.log(`smartexporter listening on port ${port}`);

  // Start a timer to fetch metrics
  metricsTimer = setInterval(getMetrics, scrapeInterval);

  // Get initial metrics
  getMetrics();
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