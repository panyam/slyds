/**
 * slyds — Lightweight HTML presentation engine
 * Convention: <div class="slide"> elements, .active class, #slideNum/#totalSlides spans
 */
(function () {
    'use strict';

    var totalSlides = document.querySelectorAll('.slide').length;
    var notesWindow = null;

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

    function showSlide(n) {
        var slides = document.querySelectorAll('.slide');

        if (n > totalSlides) currentSlide = 1;
        if (n < 1) currentSlide = totalSlides;

        slides.forEach(function (slide) { slide.classList.remove('active'); });
        slides[currentSlide - 1].classList.add('active');

        var slideNumEl = document.getElementById('slideNum');
        if (slideNumEl) slideNumEl.textContent = currentSlide;

        // Update URL hash for bookmarking/refresh
        history.replaceState(null, null, '#' + currentSlide);

        // Update navigation buttons
        var prevBtn = document.getElementById('prevBtn');
        var nextBtn = document.getElementById('nextBtn');
        if (prevBtn) prevBtn.disabled = currentSlide === 1;
        if (nextBtn) nextBtn.disabled = currentSlide === totalSlides;

        // Update speaker notes window if it exists
        updateNotesWindow();
    }

    function changeSlide(n) {
        currentSlide += n;
        showSlide(currentSlide);
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
            var title = getSlideTitleForSlide(currentSlide);
            var notes = getNotesForSlide(currentSlide);

            var titleElement = notesWindow.document.getElementById('notesTitle');
            var slideNumberElement = notesWindow.document.getElementById('slideNumber');
            var contentElement = notesWindow.document.getElementById('notesContent');

            if (titleElement && slideNumberElement && contentElement) {
                titleElement.textContent = title;
                slideNumberElement.textContent = currentSlide;
                contentElement.innerHTML = notes;
                notesWindow.document.title = 'Speaker Notes - ' + title;
            }
        }
    }

    function closeNotesWindow() {
        if (notesWindow && !notesWindow.closed) {
            notesWindow.close();
        }
    }

    // Keyboard navigation
    document.addEventListener('keydown', function (e) {
        if (e.key === 'ArrowLeft') changeSlide(-1);
        if (e.key === 'ArrowRight') changeSlide(1);
        if (e.key === 'Escape') closeNotesWindow();
        if (e.key === 'n' || e.key === 'N') openNotesWindow();
    });

    // Handle browser back/forward buttons
    window.addEventListener('hashchange', function () {
        var slideNum = getSlideFromHash();
        if (slideNum !== currentSlide) {
            currentSlide = slideNum;
            showSlide(currentSlide);
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

    // Expose to onclick handlers in HTML
    window.changeSlide = changeSlide;
    window.openNotesWindow = openNotesWindow;

    // Initialize
    showSlide(currentSlide);
})();
