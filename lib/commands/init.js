'use strict';

const fs = require('fs');
const path = require('path');
const ejs = require('ejs');

var pkg = require('../../package.json');
var CDN_BASE = 'https://unpkg.com/slyds@' + pkg.version + '/assets';

function slugify(text) {
    return text
        .toLowerCase()
        .replace(/[^a-z0-9]+/g, '-')
        .replace(/^-|-$/g, '');
}

module.exports = function init(args) {
    var slideCount = 3;
    var local = false;
    var titleParts = [];

    for (var i = 0; i < args.length; i++) {
        if (args[i] === '-n' || args[i] === '--slides') {
            slideCount = parseInt(args[++i], 10);
            if (isNaN(slideCount) || slideCount < 2) {
                console.error('Slide count must be at least 2 (title + closing).');
                process.exit(1);
            }
        } else if (args[i] === '--local') {
            local = true;
        } else {
            titleParts.push(args[i]);
        }
    }

    var title = titleParts.join(' ').trim();
    if (!title) {
        console.error('Usage: slyds init "Talk Title" [-n slides] [--local]');
        process.exit(1);
    }

    var slug = slugify(title);
    var dir = path.resolve(process.cwd(), slug);
    var root = path.resolve(__dirname, '..', '..');

    if (fs.existsSync(dir)) {
        console.error('Directory "' + slug + '" already exists.');
        process.exit(1);
    }

    fs.mkdirSync(dir, { recursive: true });

    // Determine asset URLs
    var cssUrl, jsUrl;
    if (local) {
        cssUrl = 'slyds.css';
        jsUrl = 'slyds.js';
        fs.copyFileSync(
            path.join(root, 'assets', 'slyds.js'),
            path.join(dir, 'slyds.js')
        );
        fs.copyFileSync(
            path.join(root, 'assets', 'slyds.css'),
            path.join(dir, 'slyds.css')
        );
    } else {
        cssUrl = CDN_BASE + '/slyds.css';
        jsUrl = CDN_BASE + '/slyds.js';
    }

    // Render templates
    var contentSlides = Math.max(0, slideCount - 2);

    var data = {
        title: title,
        contentSlides: contentSlides,
        totalSlides: slideCount,
        cssUrl: cssUrl,
        jsUrl: jsUrl,
    };

    var htmlTmpl = fs.readFileSync(
        path.join(root, 'templates', 'index.html.ejs'), 'utf8'
    );
    fs.writeFileSync(path.join(dir, 'index.html'), ejs.render(htmlTmpl, data));

    var cssTmpl = fs.readFileSync(
        path.join(root, 'templates', 'theme.css.ejs'), 'utf8'
    );
    fs.writeFileSync(path.join(dir, 'theme.css'), ejs.render(cssTmpl, data));

    var fileCount = local ? 4 : 2;
    console.log('\nCreated "' + slug + '/" with ' + fileCount + ' files (' + slideCount + ' slides):\n');
    console.log('  index.html       Your presentation');
    console.log('  theme.css        Color/style overrides');
    if (local) {
        console.log('  slyds.css        Base styles (local copy)');
        console.log('  slyds.js         Slide engine (local copy)');
    } else {
        console.log('  slyds.css        via CDN (' + CDN_BASE + ')');
        console.log('  slyds.js         via CDN');
    }
    console.log('\nNext steps:\n');
    console.log('  1. Open ' + slug + '/index.html in your browser');
    console.log('  2. Edit index.html to add slides');
    console.log('  3. Edit theme.css to customize colors');
    console.log('  4. slyds serve ' + slug + '    (local dev server)');
    console.log('  5. slyds build ' + slug + '    (single-file export)\n');
};
