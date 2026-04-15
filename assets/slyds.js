/**
 * slyds — Lightweight HTML presentation engine
 * Convention: <div class="slide"> elements, .active class, #slideNum/#totalSlides spans
 */
(function () {
    'use strict';

    var totalSlides = document.querySelectorAll('.slide').length;
    var notesWindow = null;
    var isSandboxed = (window.parent !== window); // MCP App iframe detection

    // Timer state — lives in main window so closing/reopening notes preserves it
    var timerStart = null;
    var timerPaused = false;
    var timerElapsed = 0;
    var timerInterval = null;

    // Reading time — computed once on load
    var slideReadingTimes = [];
    var WPM = 200;

    // ── Presentation context ──
    // Persistent context for hook consumers. The `state` bag survives slide
    // transitions — use it to cache chart instances, track first-visit flags, etc.
    window.slydsContext = {
        totalSlides: totalSlides,
        currentSlide: 1,
        direction: 'init',
        state: {}
    };

    // Write total into counter (if element exists)
    var totalEl = document.getElementById('totalSlides');
    if (totalEl) totalEl.textContent = totalSlides;

    // Get initial slide from URL hash (e.g., #5 or #slide-5)
    function getSlideFromHash() {
        var hash = window.location.hash;
        if (hash) {
            var match = hash.match(/^#(?:slide-)?(\d+)$/);
            if (match) {
                var slideNum = parseInt(match[1], 10);
                if (slideNum >= 1 && slideNum <= totalSlides) {
                    return slideNum;
                }
            }
        }
        return 1;
    }

    var currentSlide = getSlideFromHash();

    // ── Reading time computation ──
    // Computes word count and estimated reading time for each slide's
    // visible content (excludes speaker notes). Called once on load.
    function computeReadingTimes() {
        var slides = document.querySelectorAll('.slide');
        slideReadingTimes = [];
        slides.forEach(function (slide) {
            var clone = slide.cloneNode(true);
            var notes = clone.querySelector('.speaker-notes');
            if (notes) notes.remove();
            var text = clone.textContent || '';
            var words = text.trim().split(/\s+/).filter(function (w) { return w.length > 0; });
            slideReadingTimes.push(Math.ceil(words.length / WPM * 60));
        });
    }

    // Returns reading time in seconds for slide n (1-indexed)
    function getReadingTime(n) {
        return slideReadingTimes[n - 1] || 0;
    }

    // Returns total remaining reading time in seconds from current slide onward
    function getRemainingReadingTime() {
        var total = 0;
        for (var i = currentSlide; i < totalSlides; i++) {
            total += (slideReadingTimes[i] || 0);
        }
        return total;
    }

    // ── Timer functions ──

    function getElapsedMs() {
        if (timerStart !== null && !timerPaused) {
            return timerElapsed + (Date.now() - timerStart);
        }
        return timerElapsed;
    }

    function formatTime(ms) {
        var totalSec = Math.floor(ms / 1000);
        var h = Math.floor(totalSec / 3600);
        var m = Math.floor((totalSec % 3600) / 60);
        var s = totalSec % 60;
        var pad = function (n) { return n < 10 ? '0' + n : '' + n; };
        if (h > 0) {
            return h + ':' + pad(m) + ':' + pad(s);
        }
        return m + ':' + pad(s);
    }

    function formatReadingTime(seconds) {
        if (seconds < 60) return '< 1 min';
        var mins = Math.round(seconds / 60);
        return '~' + mins + ' min';
    }

    function timerButtonLabel() {
        if (timerStart === null && timerElapsed === 0) return 'Start';
        if (timerPaused) return 'Resume';
        return 'Pause';
    }

    function updateTimerDisplay() {
        if (notesWindow && !notesWindow.closed) {
            var timerEl = notesWindow.document.getElementById('notesTimer');
            if (timerEl) timerEl.textContent = formatTime(getElapsedMs());
        }
    }

    function startTimer() {
        timerStart = Date.now();
        timerPaused = false;
        timerInterval = setInterval(updateTimerDisplay, 1000);
        updateTimerDisplay();
        updateNotesTimerUI();
    }

    function pauseTimer() {
        timerElapsed += (Date.now() - timerStart);
        timerStart = null;
        timerPaused = true;
        clearInterval(timerInterval);
        timerInterval = null;
        updateTimerDisplay();
        updateNotesTimerUI();
    }

    function toggleTimer() {
        if (timerStart === null && !timerPaused && timerElapsed === 0) {
            startTimer();
        } else if (timerPaused || timerStart === null) {
            startTimer();
        } else {
            pauseTimer();
        }
    }

    function updateNotesTimerUI() {
        if (!notesWindow || notesWindow.closed) return;
        var btn = notesWindow.document.getElementById('notesTimerToggle');
        if (btn) btn.textContent = timerButtonLabel();
        var readEl = notesWindow.document.getElementById('notesReadingTime');
        if (readEl) readEl.textContent = formatReadingTime(getReadingTime(currentSlide)) + ' read';
        var remEl = notesWindow.document.getElementById('notesRemaining');
        if (remEl) remEl.textContent = formatReadingTime(getRemainingReadingTime()) + ' remaining';
    }

    // Get speaker notes from the DOM
    function getNotesForSlide(n) {
        var slide = document.querySelectorAll('.slide')[n - 1];
        var notesEl = slide ? slide.querySelector('.speaker-notes') : null;
        return notesEl ? notesEl.innerHTML : '<p>No notes for this slide.</p>';
    }

    // Get slide title from the DOM
    function getSlideTitleForSlide(n) {
        var slide = document.querySelectorAll('.slide')[n - 1];
        if (!slide) return 'Slide ' + n;
        var h1 = slide.querySelector('h1');
        return h1 ? h1.textContent : 'Slide ' + n;
    }

    // Build the detail payload for slideEnter/slideLeave events.
    function buildEventDetail(slideEl, slideIndex, direction) {
        var h1 = slideEl.querySelector('h1');
        var title = h1 ? h1.textContent : 'Slide ' + (slideIndex + 1);
        var layout = slideEl.getAttribute('data-layout') || null;

        var dataset = {};
        if (slideEl.dataset) {
            for (var key in slideEl.dataset) {
                if (slideEl.dataset.hasOwnProperty(key)) {
                    dataset[key] = slideEl.dataset[key];
                }
            }
        }

        return {
            index: slideIndex,
            slideNum: slideIndex + 1,
            title: title,
            layout: layout,
            total: totalSlides,
            direction: direction,
            data: dataset
        };
    }

    function showSlide(n, from) {
        var slides = document.querySelectorAll('.slide');
        // from is the slide we're leaving; undefined on initial load and hash nav
        var previousSlide = (from !== undefined) ? from : currentSlide;

        if (n > totalSlides) currentSlide = 1;
        if (n < 1) currentSlide = totalSlides;

        // Compute direction: use raw n (before clamping) so wrap-around is correct
        var direction;
        if (previousSlide === currentSlide) {
            direction = 'init';
        } else if (n > previousSlide) {
            direction = 'forward';
        } else {
            direction = 'backward';
        }

        // Dispatch slideLeave on outgoing slide (still .active, has dimensions)
        var outgoing = slides[previousSlide - 1];
        if (outgoing && outgoing.classList.contains('active')) {
            outgoing.dispatchEvent(new CustomEvent('slideLeave', {
                bubbles: true,
                detail: buildEventDetail(outgoing, previousSlide - 1, direction)
            }));
        }

        slides.forEach(function (slide) { slide.classList.remove('active'); });
        slides[currentSlide - 1].classList.add('active');

        // Dispatch slideEnter on incoming slide (now .active, has dimensions)
        slides[currentSlide - 1].dispatchEvent(new CustomEvent('slideEnter', {
            bubbles: true,
            detail: buildEventDetail(slides[currentSlide - 1], currentSlide - 1, direction)
        }));

        // Update presentation context
        window.slydsContext.currentSlide = currentSlide;
        window.slydsContext.direction = direction;

        var slideNumEl = document.getElementById('slideNum');
        if (slideNumEl) slideNumEl.textContent = currentSlide;

        // Update URL hash for bookmarking/refresh
        history.replaceState(null, null, '#' + currentSlide);

        // Update navigation buttons
        var prevBtn = document.getElementById('prevBtn');
        var nextBtn = document.getElementById('nextBtn');
        if (prevBtn) prevBtn.disabled = currentSlide === 1;
        if (nextBtn) nextBtn.disabled = currentSlide === totalSlides;

        // Update speaker notes (popup window or inline panel)
        updateNotesWindow();
        updateNotesPanel();
    }

    function changeSlide(n) {
        var from = currentSlide;
        currentSlide += n;
        showSlide(currentSlide, from);
    }

    // ── Inline notes panel (sandboxed/iframe context) ──
    // In MCP App iframes, window.open() is blocked. Instead we show a
    // collapsible panel between the slide area and the nav bar.

    function createNotesPanel() {
        if (document.getElementById('slyds-notes-panel')) return;
        var panel = document.createElement('div');
        panel.id = 'slyds-notes-panel';
        panel.className = 'slyds-notes-panel';
        panel.style.display = 'none';

        var header = document.createElement('div');
        header.className = 'slyds-notes-header';

        var titleEl = document.createElement('span');
        titleEl.className = 'slyds-notes-title';
        header.appendChild(titleEl);

        var metaEl = document.createElement('span');
        metaEl.className = 'slyds-notes-meta';
        header.appendChild(metaEl);

        var closeBtn = document.createElement('button');
        closeBtn.className = 'slyds-notes-close';
        closeBtn.textContent = '\u00d7';
        closeBtn.setAttribute('title', 'Close notes');
        closeBtn.onclick = function() { toggleNotesPanel(); };
        header.appendChild(closeBtn);

        var content = document.createElement('div');
        content.className = 'slyds-notes-content';

        panel.appendChild(header);
        panel.appendChild(content);

        var nav = document.querySelector('.navigation');
        if (nav && nav.parentNode) {
            nav.parentNode.insertBefore(panel, nav);
        } else {
            document.querySelector('.slideshow-container').appendChild(panel);
        }
    }

    function toggleNotesPanel() {
        var panel = document.getElementById('slyds-notes-panel');
        if (!panel) return;
        var visible = panel.style.display !== 'none';
        panel.style.display = visible ? 'none' : 'flex';
        if (!visible) updateNotesPanel();
    }

    function updateNotesPanel() {
        var panel = document.getElementById('slyds-notes-panel');
        if (!panel || panel.style.display === 'none') return;
        updateNotesContent(
            panel.querySelector('.slyds-notes-title'),
            null, // no separate slide number element — included in meta
            panel.querySelector('.slyds-notes-content'),
            panel.querySelector('.slyds-notes-meta')
        );
    }

    // Shared content update for both popup window and inline panel.
    // Updates title, slide number, notes content, and reading time.
    // Notes HTML comes from the user's own .speaker-notes DOM elements — not external input.
    function updateNotesContent(titleEl, slideNumEl, contentEl, metaEl) {
        var title = getSlideTitleForSlide(currentSlide);
        var notes = getNotesForSlide(currentSlide);
        var readTime = formatReadingTime(getReadingTime(currentSlide));
        if (titleEl) titleEl.textContent = title;
        if (slideNumEl) slideNumEl.textContent = currentSlide;
        if (metaEl) metaEl.textContent = 'Slide ' + currentSlide + '/' + totalSlides + ' \u00b7 ' + readTime + ' read';
        // Safe: notes HTML is from the user's own slides, not external input.
        if (contentEl) contentEl.innerHTML = notes; // eslint-disable-line no-unsanitized/property
    }

    // openSpeakerNotes dispatches to inline panel or popup based on context.
    // Pass asWindow=true to force the popup (ignored if sandboxed).
    function openSpeakerNotes(asWindow) {
        if (!asWindow && isSandboxed) {
            toggleNotesPanel();
            return;
        }
        openNotesWindow();
    }

    function openNotesWindow() {
        // Close existing notes window if open
        if (notesWindow && !notesWindow.closed) {
            notesWindow.close();
        }

        // Open new notes window
        notesWindow = window.open('', 'speakerNotes', 'width=900,height=700,scrollbars=yes,resizable=yes');

        if (notesWindow) {
            var title = getSlideTitleForSlide(currentSlide);
            var notes = getNotesForSlide(currentSlide);
            var readTime = formatReadingTime(getReadingTime(currentSlide));
            var remaining = formatReadingTime(getRemainingReadingTime());

            // Build notes window HTML — uses document.write because we're
            // populating a blank popup window (no XSS risk: content is from
            // the user's own slide DOM, not external input)
            var html = [
                '<html>',
                '<head>',
                '<title>Speaker Notes - ' + title + '</title>',
                '<style>',
                'body { font-family: "Segoe UI", Arial, sans-serif; padding: 30px; background: #2c3e50; color: white; line-height: 1.6; margin: 0; }',
                '.header { background: #34495e; padding: 20px; margin: -30px -30px 30px -30px; border-bottom: 3px solid #3498db; }',
                'h1 { color: #3498db; margin: 0; font-size: 1.8em; }',
                '.slide-info { color: #bdc3c7; margin-top: 5px; font-size: 1.1em; }',
                '.timer-bar { display: flex; align-items: center; gap: 16px; margin-top: 12px; padding-top: 12px; border-top: 1px solid rgba(255,255,255,0.1); }',
                '.timer-bar .elapsed { font-family: "SF Mono", "JetBrains Mono", monospace; font-size: 1.4em; color: #2ecc71; font-weight: bold; min-width: 70px; }',
                '.timer-bar .meta { color: #95a5a6; font-size: 0.9em; }',
                '.timer-bar .timer-btn { background: #3498db; color: white; border: none; padding: 5px 14px; border-radius: 4px; cursor: pointer; font-size: 0.85em; font-weight: 600; }',
                '.timer-bar .timer-btn:hover { background: #2980b9; }',
                'h2 { color: #3498db; margin-top: 30px; margin-bottom: 15px; font-size: 1.4em; }',
                'h3 { color: #e74c3c; margin-top: 25px; margin-bottom: 10px; font-size: 1.2em; }',
                'p, li { font-size: 1em; line-height: 1.6; margin-bottom: 12px; }',
                'ul, ol { margin-left: 20px; margin-bottom: 15px; }',
                'li { margin-bottom: 8px; }',
                'strong { color: #f39c12; }',
                '.content { max-width: 800px; }',
                'code { background: #1a252f; padding: 2px 6px; border-radius: 3px; font-family: monospace; }',
                '</style>',
                '</head>',
                '<body>',
                '<div class="header">',
                '<h1 id="notesTitle">' + title + '</h1>',
                '<div class="slide-info">Slide <span id="slideNumber">' + currentSlide + '</span> of ' + totalSlides + '</div>',
                '<div class="timer-bar">',
                '<span class="elapsed" id="notesTimer">' + formatTime(getElapsedMs()) + '</span>',
                '<span class="meta" id="notesReadingTime">' + readTime + ' read</span>',
                '<span class="meta" id="notesRemaining">' + remaining + ' remaining</span>',
                '<button class="timer-btn" id="notesTimerToggle" onclick="window.opener.toggleTimer()">' + timerButtonLabel() + '</button>',
                '</div>',
                '</div>',
                '<div class="content" id="notesContent">' + notes + '</div>',
                '</body>',
                '</html>'
            ].join('\n');

            notesWindow.document.open();
            notesWindow.document.write(html);
            notesWindow.document.close();
        }
    }

    function updateNotesWindow() {
        if (notesWindow && !notesWindow.closed) {
            updateNotesContent(
                notesWindow.document.getElementById('notesTitle'),
                notesWindow.document.getElementById('slideNumber'),
                notesWindow.document.getElementById('notesContent'),
                null // popup has its own reading time UI
            );
            notesWindow.document.title = 'Speaker Notes - ' + getSlideTitleForSlide(currentSlide);
            updateTimerDisplay();
            updateNotesTimerUI();
        }
    }

    function closeNotesWindow() {
        if (notesWindow && !notesWindow.closed) {
            notesWindow.close();
        }
    }

    function closeSpeakerNotes() {
        closeNotesWindow();
        var panel = document.getElementById('slyds-notes-panel');
        if (panel) panel.style.display = 'none';
    }

    // Keyboard navigation
    document.addEventListener('keydown', function (e) {
        if (e.key === 'ArrowLeft') changeSlide(-1);
        if (e.key === 'ArrowRight') changeSlide(1);
        if (e.key === 'Escape') closeSpeakerNotes();
        if (e.key === 'n' || e.key === 'N') openSpeakerNotes();
        if (e.key === 't' || e.key === 'T') toggleTimer();
    });

    // Handle browser back/forward buttons
    window.addEventListener('hashchange', function () {
        var slideNum = getSlideFromHash();
        if (slideNum !== currentSlide) {
            var from = currentSlide;
            currentSlide = slideNum;
            showSlide(currentSlide, from);
        }
    });

    // Set position-aware CSS custom properties on each slide.
    // Themes can use these for conditional styling:
    //   --slide-index:    0-based position (0, 1, 2, ...)
    //   --slide-progress: percentage through the deck ("0%", "50%", "100%")
    //   --total-slides:   total number of slides
    (function setSlidePositionProperties() {
        var slides = document.querySelectorAll('.slide');
        slides.forEach(function (slide, i) {
            slide.style.setProperty('--slide-index', i);
            var progress = totalSlides > 1 ? (i / (totalSlides - 1) * 100) : 0;
            slide.style.setProperty('--slide-progress', progress + '%');
            slide.style.setProperty('--total-slides', totalSlides);
        });
    })();

    // ── Theme switcher ──

    // Discover available themes from loaded [data-theme="..."] CSS rules
    function getAvailableThemes() {
        var themes = [];
        var seen = {};
        try {
            var sheets = document.styleSheets;
            for (var i = 0; i < sheets.length; i++) {
                var rules;
                try { rules = sheets[i].cssRules || sheets[i].rules; } catch (e) { continue; }
                if (!rules) continue;
                for (var j = 0; j < rules.length; j++) {
                    var sel = rules[j].selectorText || '';
                    var match = sel.match(/\[data-theme="(\w[\w-]*)"\]/);
                    if (match && !seen[match[1]]) {
                        seen[match[1]] = true;
                        themes.push(match[1]);
                    }
                }
            }
        } catch (e) { /* cross-origin stylesheet, ignore */ }
        return themes.length > 0 ? themes : ['default'];
    }

    function getCurrentTheme() {
        return document.documentElement.getAttribute('data-theme') || 'default';
    }

    function setTheme(name) {
        document.documentElement.setAttribute('data-theme', name);
        try { localStorage.setItem('slyds-theme', name); } catch (e) { /* ignore */ }
    }

    function cycleTheme() {
        var themes = getAvailableThemes();
        var current = getCurrentTheme();
        var idx = themes.indexOf(current);
        var next = themes[(idx + 1) % themes.length];
        setTheme(next);
    }

    // Restore theme from localStorage if saved
    (function restoreTheme() {
        try {
            var saved = localStorage.getItem('slyds-theme');
            if (saved) {
                document.documentElement.setAttribute('data-theme', saved);
            }
        } catch (e) { /* ignore */ }
    })();

    // Expose to onclick handlers in HTML and MCP App bridge
    window.changeSlide = changeSlide;
    window.showSlide = showSlide;
    window.openSpeakerNotes = openSpeakerNotes;
    window.openNotesWindow = openNotesWindow; // legacy — use openSpeakerNotes
    window.toggleNotesPanel = toggleNotesPanel;
    window.toggleTimer = toggleTimer;
    window.cycleTheme = cycleTheme;
    window.slydsGetCurrentSlide = function() {
        return currentSlide + 1; // 1-based
    };
    window.slydsTotalSlides = function() {
        return document.querySelectorAll('.slide').length;
    };

    // Initialize
    computeReadingTimes();
    if (isSandboxed) createNotesPanel();
    showSlide(currentSlide);
})();
