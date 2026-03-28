/**
 * slyds-export — Client-side slide export/download
 *
 * Extracts slides from the DOM, wraps each in standalone HTML with styles,
 * bundles into a ZIP, and triggers a browser download. Works from file://,
 * static hosts, and slyds serve — no server required.
 *
 * Uses a minimal store-only ZIP writer (no external dependencies).
 */
(function () {
    'use strict';

    // ── CRC-32 ──────────────────────────────────────────────────────────
    var crcTable = (function () {
        var table = new Uint32Array(256);
        for (var n = 0; n < 256; n++) {
            var c = n;
            for (var k = 0; k < 8; k++) {
                c = (c & 1) ? (0xEDB88320 ^ (c >>> 1)) : (c >>> 1);
            }
            table[n] = c;
        }
        return table;
    })();

    function crc32(bytes) {
        var crc = 0xFFFFFFFF;
        for (var i = 0; i < bytes.length; i++) {
            crc = crcTable[(crc ^ bytes[i]) & 0xFF] ^ (crc >>> 8);
        }
        return (crc ^ 0xFFFFFFFF) >>> 0;
    }

    // ── Minimal ZIP writer (store only, no compression) ─────────────────
    function MiniZip() {
        this.files = [];
    }

    MiniZip.prototype.addFile = function (name, content) {
        var encoder = new TextEncoder();
        var nameBytes = encoder.encode(name);
        var contentBytes = (content instanceof Uint8Array) ? content : encoder.encode(content);
        this.files.push({
            name: nameBytes,
            content: contentBytes,
            crc: crc32(contentBytes)
        });
    };

    MiniZip.prototype.generate = function () {
        var localHeaders = [];
        var centralHeaders = [];
        var offset = 0;

        for (var i = 0; i < this.files.length; i++) {
            var f = this.files[i];

            // Local file header (30 bytes + name + content)
            var local = new Uint8Array(30 + f.name.length + f.content.length);
            var lv = new DataView(local.buffer);
            lv.setUint32(0, 0x04034b50, true);   // local file header signature
            lv.setUint16(4, 20, true);            // version needed (2.0)
            lv.setUint16(6, 0x0800, true);        // general purpose bit flag (bit 11 = UTF-8)
            lv.setUint16(8, 0, true);             // compression method (store)
            lv.setUint16(10, 0, true);            // last mod time
            lv.setUint16(12, 0, true);            // last mod date
            lv.setUint32(14, f.crc, true);        // crc-32
            lv.setUint32(18, f.content.length, true); // compressed size
            lv.setUint32(22, f.content.length, true); // uncompressed size
            lv.setUint16(26, f.name.length, true);    // filename length
            lv.setUint16(28, 0, true);            // extra field length
            local.set(f.name, 30);
            local.set(f.content, 30 + f.name.length);
            localHeaders.push(local);

            // Central directory header (46 bytes + name)
            var central = new Uint8Array(46 + f.name.length);
            var cv = new DataView(central.buffer);
            cv.setUint32(0, 0x02014b50, true);   // central directory signature
            cv.setUint16(4, 20, true);            // version made by
            cv.setUint16(6, 20, true);            // version needed
            cv.setUint16(8, 0x0800, true);        // general purpose bit flag (UTF-8)
            cv.setUint16(10, 0, true);            // compression method (store)
            cv.setUint16(12, 0, true);            // last mod time
            cv.setUint16(14, 0, true);            // last mod date
            cv.setUint32(16, f.crc, true);        // crc-32
            cv.setUint32(20, f.content.length, true); // compressed size
            cv.setUint32(24, f.content.length, true); // uncompressed size
            cv.setUint16(28, f.name.length, true);    // filename length
            cv.setUint16(30, 0, true);            // extra field length
            cv.setUint16(32, 0, true);            // file comment length
            cv.setUint16(34, 0, true);            // disk number start
            cv.setUint16(36, 0, true);            // internal file attributes
            cv.setUint32(38, 0, true);            // external file attributes
            cv.setUint32(42, offset, true);       // relative offset of local header
            central.set(f.name, 46);
            centralHeaders.push(central);

            offset += local.length;
        }

        // End of central directory record (22 bytes)
        var centralSize = centralHeaders.reduce(function (s, c) { return s + c.length; }, 0);
        var eocd = new Uint8Array(22);
        var ev = new DataView(eocd.buffer);
        ev.setUint32(0, 0x06054b50, true);    // end of central directory signature
        ev.setUint16(4, 0, true);             // disk number
        ev.setUint16(6, 0, true);             // disk with central directory
        ev.setUint16(8, this.files.length, true);  // entries on this disk
        ev.setUint16(10, this.files.length, true); // total entries
        ev.setUint32(12, centralSize, true);  // size of central directory
        ev.setUint32(16, offset, true);       // offset of central directory
        ev.setUint16(20, 0, true);            // comment length

        // Concatenate all parts
        var totalSize = offset + centralSize + 22;
        var result = new Uint8Array(totalSize);
        var pos = 0;
        for (var j = 0; j < localHeaders.length; j++) {
            result.set(localHeaders[j], pos);
            pos += localHeaders[j].length;
        }
        for (var k = 0; k < centralHeaders.length; k++) {
            result.set(centralHeaders[k], pos);
            pos += centralHeaders[k].length;
        }
        result.set(eocd, pos);

        return result;
    };

    // ── Slugify (mirrors Go's scaffold.Slugify) ─────────────────────────
    function slugify(text) {
        return text
            .toLowerCase()
            .replace(/[^a-z0-9]+/g, '-')
            .replace(/^-+|-+$/g, '')
            .replace(/-{2,}/g, '-');
    }

    // ── Export logic ────────────────────────────────────────────────────
    function exportPresentation() {
        var zip = new MiniZip();

        // Collect all <style> blocks for standalone slide wrapping
        var styles = [];
        var styleEls = document.querySelectorAll('style');
        for (var s = 0; s < styleEls.length; s++) {
            styles.push(styleEls[s].outerHTML);
        }
        var styleBlock = styles.join('\n');

        // Add the full deck as index.html
        var fullHTML = '<!DOCTYPE html>\n' + document.documentElement.outerHTML;
        zip.addFile('index.html', fullHTML);

        // Extract individual slides
        var slides = document.querySelectorAll('.slide');
        for (var i = 0; i < slides.length; i++) {
            var slide = slides[i];
            var num = String(i + 1);
            if (num.length < 2) num = '0' + num;

            // Get slide name from <h1> or fall back to slide-NN
            var h1 = slide.querySelector('h1');
            var name = h1 ? slugify(h1.textContent) : 'slide';
            var filename = 'slides/' + num + '-' + name + '.html';

            // Build standalone HTML for this slide
            var slideHTML = '<!DOCTYPE html>\n' +
                '<html lang="en">\n' +
                '<head>\n' +
                '  <meta charset="UTF-8">\n' +
                '  <meta name="viewport" content="width=device-width, initial-scale=1.0">\n' +
                '  <title>' + (h1 ? h1.textContent : 'Slide ' + (i + 1)) + '</title>\n' +
                styleBlock + '\n' +
                '</head>\n' +
                '<body>\n' +
                '  <div class="slideshow-container">\n' +
                '    <div class="slide active">\n' +
                slide.innerHTML + '\n' +
                '    </div>\n' +
                '  </div>\n' +
                '</body>\n' +
                '</html>';

            zip.addFile(filename, slideHTML);
        }

        // Generate and download
        var zipData = zip.generate();
        var blob = new Blob([zipData], { type: 'application/zip' });
        var url = URL.createObjectURL(blob);

        var title = slugify(document.title || 'presentation');
        var a = document.createElement('a');
        a.href = url;
        a.download = title + '.zip';
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    }

    // Expose to onclick handler in HTML
    window.exportPresentation = exportPresentation;
})();
