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
        }
        return cb(null, lines.slice(4));
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
        lines.slice(0, -1).forEach(line => {
            const device = line
                .substring(0, line.search(': '))
                .trim()
                .replace(/ +/g, '_')
                .toLowerCase();
            deviceInfos[device] = line.substring(1 + line.search(': ')).trim();
        });
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
        lines.forEach(line => {
            const attr = line.substring(head.indexOf('ATTRIBUTE_NAME'), head.indexOf('FLAGS')).trim().toLowerCase();
            if (attr === '')
                return;

            let info = {
                attr,
                id: Number(line.substring(0, head.indexOf('ATTRIBUTE_NAME')).trim()),
                flags: line.substring(head.indexOf('FLAGS'), head.indexOf('VALUE')).trim(),
                value: line.substring(head.indexOf('VALUE'), head.indexOf('WORST')).trim(),
                worst: line.substring(head.indexOf('WORST'), head.indexOf('THRESH')).trim(),
                thresh: line.substring(head.indexOf('THRESH'), head.indexOf('FAIL')).trim(),
                fail: line.substring(head.indexOf('FAIL'), head.indexOf('RAW_VALUE')).trim(),
                raw: Number(line.substring(head.indexOf('RAW_VALUE')).trim().split(' ')[0])
            };

            // === raw value formatting exceptions === //

            if (attr === 'power_on_hours' && isNaN(info.raw)) {
                // raw value may be in format "59342h+32m+09.624s"
                info.raw = Number(line.substring(head.indexOf('RAW_VALUE')).trim().split('h')[0])
            }

            if (attr === 'head_flying_hours' && isNaN(info.raw)) {
                // raw value may be in format "59342h+32m+09.624s"
                info.raw = Number(line.substring(head.indexOf('RAW_VALUE')).trim().split('h')[0])
            }

            infos.push(info);
        });
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
        lines.forEach(line => {
            if (line.search(':') === -1)
                return;

            infos.push({
                attr: line.substring(0, line.search(':')).trim().toLowerCase().replace(/ +/g, '_'),
                id: -1,
                raw: Number(line.substring(line.search(':') + 1).trim().split(' ')[0])
            });
        });
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
        }
        return cb(null, lines);
    });
};

const scan = (cb) => {
    return exec('smartctl --scan-open', {
        maxBuffer: 1024 * 1024 * 24
    }, function (e, stdout, stderr) {
        const devices = [];
        const ref = stdout.split('\n').slice(0, -1);
        ref.forEach(n => {
            devices.push(n.split(' ')[0]);
        });
        return cb(devices);
    });
};

export {execSmart, info, smartAttrs, smartSasAttrs, health, scan};