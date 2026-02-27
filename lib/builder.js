'use strict';

const fs = require('fs');
const path = require('path');
const https = require('https');

/**
 * Fetch a URL over HTTPS and return its body as a string.
 */
function fetchUrl(url) {
    return new Promise(function (resolve, reject) {
        https.get(url, function (res) {
            if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
                return fetchUrl(res.headers.location).then(resolve, reject);
            }
            if (res.statusCode !== 200) {
                return reject(new Error('HTTP ' + res.statusCode + ' for ' + url));
            }
            var chunks = [];
            res.on('data', function (chunk) { chunks.push(chunk); });
            res.on('end', function () { resolve(Buffer.concat(chunks).toString('utf8')); });
            res.on('error', reject);
        }).on('error', reject);
    });
}

/**
 * Resolve an href/src to its content — local file or CDN fetch.
 */
function resolveAsset(ref, baseDir) {
    if (ref.startsWith('https://')) return fetchUrl(ref);
    if (ref.startsWith('http://')) return fetchUrl(ref);
    var filePath = path.resolve(baseDir, ref);
    if (!fs.existsSync(filePath)) return Promise.resolve(null);
    return Promise.resolve(fs.readFileSync(filePath, 'utf8'));
}

/**
 * Inline all <link rel="stylesheet" href="..."> and <script src="...">
 * references in an HTML string. Handles both local files and CDN URLs.
 * Returns a promise of { html, warnings }.
 */
module.exports = function inlineAssets(html, baseDir) {
    var warnings = [];
    var replacements = [];

    // Collect CSS: <link rel="stylesheet" href="X"> (either attr order)
    var cssRe = /<link\s+[^>]*(?:rel=["']stylesheet["'][^>]*href=["']([^"']+)["']|href=["']([^"']+)["'][^>]*rel=["']stylesheet["'])[^>]*\/?>/gi;
    var cssMatch;
    while ((cssMatch = cssRe.exec(html)) !== null) {
        var href = cssMatch[1] || cssMatch[2];
        replacements.push({ original: cssMatch[0], ref: href, type: 'css' });
    }

    // Collect JS: <script src="X"></script>
    var jsRe = /<script\s+[^>]*src=["']([^"']+)["'][^>]*><\/script>/gi;
    var jsMatch;
    while ((jsMatch = jsRe.exec(html)) !== null) {
        replacements.push({ original: jsMatch[0], ref: jsMatch[1], type: 'js' });
    }

    // Resolve all assets in parallel (local reads or CDN fetches)
    var promises = replacements.map(function (r) {
        return resolveAsset(r.ref, baseDir).then(function (content) {
            r.content = content;
        }).catch(function (err) {
            warnings.push('Failed to fetch ' + r.ref + ': ' + err.message);
            r.content = null;
        });
    });

    return Promise.all(promises).then(function () {
        replacements.forEach(function (r) {
            if (r.content === null) {
                warnings.push((r.type === 'css' ? 'CSS' : 'JS') + ' not inlined: ' + r.ref);
                return;
            }
            var tag = r.type === 'css'
                ? '<style>\n' + r.content + '\n</style>'
                : '<script>\n' + r.content + '\n</script>';
            html = html.replace(r.original, tag);
        });

        // Warn about local images
        var imgRe = /<img\s+[^>]*src=["']([^"']+)["']/gi;
        var imgMatch;
        while ((imgMatch = imgRe.exec(html)) !== null) {
            var imgSrc = imgMatch[1];
            if (!imgSrc.startsWith('http://') && !imgSrc.startsWith('https://') && !imgSrc.startsWith('data:')) {
                warnings.push('Image not inlined (use base64 manually if needed): ' + imgSrc);
            }
        }

        return { html: html, warnings: warnings };
    });
};
