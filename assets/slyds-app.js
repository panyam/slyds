/* slyds MCP App handlers — bidirectional tool support + live edit.
 *
 * Loaded AFTER the MCP App Bridge and slyds.js. Registers app-side tools
 * (slide navigation) that the host or LLM can invoke without a server
 * round-trip, and listens for toolresult events to refresh previews
 * after edits.
 *
 * Depends on globals from slyds.js:
 *   window.changeSlide(n)        — relative navigation
 *   window.showSlide(n)          — absolute navigation (0-based)
 *   window.slydsGetCurrentSlide() — current position (1-based)
 *   window.slydsTotalSlides()    — total slide count
 */
(function() {
  var PREFIX = '[slyds-app]';

  // --- Diagnostics ---
  // Logs lifecycle events to the host dev console for debugging preview
  // rendering issues (e.g. VS Code iframe not showing content).
  function log(msg) {
    console.log(PREFIX, msg);
  }

  log('loaded — slides: ' + (window.slydsTotalSlides ? window.slydsTotalSlides() : '?') +
      ', slyds.js: ' + (typeof window.changeSlide === 'function' ? 'ok' : 'MISSING'));

  if (typeof MCPApp === 'undefined') {
    log('bridge not available — running in standalone mode');
    return;
  }

  log('bridge found — waiting for connection...');

  MCPApp.on('connected', function(data) {
    var ctx = data.hostContext || {};
    log('connected to host' +
        ' | theme: ' + (ctx.theme || 'none') +
        ' | capabilities: ' + Object.keys(data.capabilities || {}).join(', '));
  });

  // Log if bridge fails to connect (2s timeout in bridge).
  setTimeout(function() {
    if (!MCPApp.connected) {
      log('WARNING: bridge not connected after 3s — host may not support MCP Apps');
    }
  }, 3000);

  // --- App-side tools ---
  // Host or LLM can call these directly on the iframe.
  MCPApp.onlisttools = function() {
    return [
      { name: "next_slide", description: "Navigate to next slide" },
      { name: "prev_slide", description: "Navigate to previous slide" },
      { name: "goto_slide", description: "Jump to a specific slide by position",
        inputSchema: {
          type: "object",
          properties: { position: { type: "number", description: "1-based slide position" } },
          required: ["position"]
        }
      },
      { name: "get_current_slide", description: "Get current slide position and total count" }
    ];
  };

  MCPApp.oncalltool = function(params) {
    log('tool call: ' + params.name);
    switch (params.name) {
      case "next_slide":
        window.changeSlide(1);
        return { content: [{ type: "text", text: "Navigated to slide " + window.slydsGetCurrentSlide() }] };
      case "prev_slide":
        window.changeSlide(-1);
        return { content: [{ type: "text", text: "Navigated to slide " + window.slydsGetCurrentSlide() }] };
      case "goto_slide":
        var pos = params.arguments && params.arguments.position;
        if (!pos || pos < 1) {
          return { isError: true, content: [{ type: "text", text: "Invalid position: " + pos }] };
        }
        window.showSlide(pos - 1); // showSlide is 0-based
        return { content: [{ type: "text", text: "Jumped to slide " + pos }] };
      case "get_current_slide":
        return { content: [{ type: "text", text: JSON.stringify({
          position: window.slydsGetCurrentSlide(),
          total: window.slydsTotalSlides()
        }) }] };
      default:
        return { isError: true, content: [{ type: "text", text: "Unknown tool: " + params.name }] };
    }
  };

  // --- Live edit ---
  // Refresh preview when host pushes mutation tool results.
  MCPApp.on('toolresult', function(data) {
    log('toolresult: ' + data.tool);
    if (data.tool === 'edit_slide' || data.tool === 'add_slide' || data.tool === 'remove_slide') {
      log('refreshing preview...');
      location.reload();
    }
  });

  // Theme changes from host.
  MCPApp.on('hostcontextchanged', function(data) {
    var ctx = data.hostContext || {};
    log('host context changed — theme: ' + (ctx.theme || 'none'));
  });
})();
