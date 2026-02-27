'use strict';

const http = require('http');
const fs = require('fs');
const path = require('path');

const MIME = {
    '.html': 'text/html',
    '.css':  'text/css',
    '.js':   'application/javascript',
    '.json': 'application/json',
    '.png':  'image/png',
    '.jpg':  'image/jpeg',
    '.jpeg': 'image/jpeg',
    '.gif':  'image/gif',
    '.svg':  'image/svg+xml',
    '.ico':  'image/x-icon',
    '.woff': 'font/woff',
    '.woff2':'font/woff2',
};

module.exports = function startServer(root, port) {
    var server = http.createServer(function (req, res) {
        var url = req.url.split('?')[0];
        if (url === '/') url = '/index.html';

        var filePath = path.join(root, url);

        // Prevent directory traversal
        if (!filePath.startsWith(root)) {
            res.writeHead(403);
            res.end('Forbidden');
            return;
        }

        fs.readFile(filePath, function (err, data) {
            if (err) {
                res.writeHead(404);
                res.end('Not found');
                return;
            }
            var ext = path.extname(filePath).toLowerCase();
            var mime = MIME[ext] || 'application/octet-stream';
            res.writeHead(200, { 'Content-Type': mime });
            res.end(data);
        });
    });

    server.listen(port, function () {
        console.log('\nServing at http://localhost:' + port);
        console.log('Press Ctrl+C to stop.\n');
    });
};
