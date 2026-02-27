'use strict';

const fs = require('fs');
const path = require('path');
const inlineAssets = require('../builder');

module.exports = function build(args) {
    var dir = args[0] || '.';
    var root = path.resolve(process.cwd(), dir);
    var indexPath = path.join(root, 'index.html');

    if (!fs.existsSync(indexPath)) {
        console.error('No index.html found in ' + root);
        process.exit(1);
    }

    var html = fs.readFileSync(indexPath, 'utf8');

    inlineAssets(html, root).then(function (result) {
        var distDir = path.join(root, 'dist');
        fs.mkdirSync(distDir, { recursive: true });

        var outPath = path.join(distDir, 'index.html');
        fs.writeFileSync(outPath, result.html);

        console.log('\nBuild complete: ' + path.relative(process.cwd(), outPath));

        if (result.warnings.length) {
            console.log('\nWarnings:');
            result.warnings.forEach(function (w) {
                console.log('  - ' + w);
            });
        }

        console.log('');
    }).catch(function (err) {
        console.error('Build failed: ' + err.message);
        process.exit(1);
    });
};
