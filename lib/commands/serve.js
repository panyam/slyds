'use strict';

const path = require('path');
const fs = require('fs');
const startServer = require('../server');

module.exports = function serve(args) {
    var port = 3000;
    var dir = '.';

    for (var i = 0; i < args.length; i++) {
        if (args[i] === '--port' || args[i] === '-p') {
            port = parseInt(args[++i], 10);
            if (isNaN(port)) {
                console.error('Invalid port number.');
                process.exit(1);
            }
        } else {
            dir = args[i];
        }
    }

    var root = path.resolve(process.cwd(), dir);
    if (!fs.existsSync(path.join(root, 'index.html'))) {
        console.error('No index.html found in ' + root);
        process.exit(1);
    }

    startServer(root, port);
};
