/*
    s m a r t c t l
 */

const exec = require('child_process').exec;

/*
    R A W  C M D
 */

const execSmart = (args, cb) => {
    return exec("smartctl " + args, {
        maxBuffer: 1024 * 1024 * 24
    }, function (e, stdout, stderr) {
        const lines = stdout.split('\n').slice(0, -1);
        if (e != null) {
            return cb(lines.slice(3), []);
        } else {
            return cb(null, lines.slice(4));
        }
    });
};

/*
    D E V I C E  I N F O S
 */

const info = (devicePath, cb) => {
    return execSmart("-i " + devicePath, function (e, lines) {
        if (e != null) {
            return cb(e, lines);
        }
        const deviceInfos = {};
        const ref = lines.slice(0, -1);
        for (let i = 0, len = ref.length; i < len; i++) {
            const line = ref[i];
            deviceInfos[line.substring(0, line.search(': ')).trim().replace(/ +/g, '_').toLowerCase()] = line.substring(1 + line.search(': ')).trim();
        }
        return cb(null, deviceInfos);
    });
};

/*
    S M A R T  A T T R S
 */

const smartAttrs = (devicePath, cb) => {
    return execSmart("-A -f brief " + devicePath, function (e, lines) {
        if (e != null) {
            return cb(e, lines);
        }
        lines = lines.slice(2, -1);
        const head = lines.shift();
        const infos = [];
        for (let i = 0, len = lines.length; i < len; i++) {
            const line = lines[i];
            const attr = line.substring(head.indexOf('ATTRIBUTE_NAME'), head.indexOf('FLAGS')).trim().toLowerCase();
            if (attr === '') {
                continue;
            }
            infos.push({
                attr,
                id: Number(line.substring(0, head.indexOf('ATTRIBUTE_NAME')).trim()),
                flags: line.substring(head.indexOf('FLAGS'), head.indexOf('VALUE')).trim(),
                value: line.substring(head.indexOf('VALUE'), head.indexOf('WORST')).trim(),
                worst: line.substring(head.indexOf('WORST'), head.indexOf('THRESH')).trim(),
                thresh: line.substring(head.indexOf('THRESH'), head.indexOf('FAIL')).trim(),
                fail: line.substring(head.indexOf('FAIL'), head.indexOf('RAW_VALUE')).trim(),
                raw: Number(line.substring(head.indexOf('RAW_VALUE')).trim().split(' ')[0])
            });
        }
        return cb(null, infos);
    });
};

/*
    S M A R T  S A S  A T T R S
 */

const smartSasAttrs = (devicePath, cb) => {
    return execSmart("-A -f brief " + devicePath, function (e, lines) {
        if (e != null) {
            return cb(e, lines);
        }

        const infos = [];
        for (let i = 0, len = lines.length; i < len; i++) {
            const line = lines[i];
            if (line.search(':') === -1)
                continue;

            infos.push({
                attr: line.substring(0, line.search(':')).trim().toLowerCase().replace(/ +/g, '_'),
                id: -1,
                raw: Number(line.substring(line.search(':') + 1).trim().split(' ')[0])
            });
        }
        return cb(null, infos);
    });
};

/*
    S M A R T  H E A L T H
 */

const health = (devicePath, cb) => {
    return execSmart("-H " + devicePath, function (e, lines) {
        if (e != null) {
            return cb(e, lines);
        }
        lines = lines.slice(0, -1);
        if (0 === lines[0].search('SMART overall-health self-assessment test result: ')) {
            const status = lines[0].split(' ').pop().toLowerCase();
            return cb(null, status);
        } else {
            return cb(null, lines);
        }
    });
};

const scan = (cb) => {
    return exec('smartctl --scan-open', {
        maxBuffer: 1024 * 1024 * 24
    }, function (e, stdout, stderr) {
        const devices = [];
        const ref = stdout.split('\n').slice(0, -1);
        for (let i = 0, len = ref.length; i < len; i++) {
            const n = ref[i];
            devices.push(n.split(' ')[0]);
        }
        return cb(devices);
    });
};

export {execSmart, info, smartAttrs, smartSasAttrs, health, scan};