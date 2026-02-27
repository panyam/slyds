#!/usr/bin/env node
'use strict';

const args = process.argv.slice(2);
const command = args[0];

if (!command || command === '--help' || command === '-h') {
  console.log(`
slyds — Lightweight HTML presentation toolkit

Usage:
  slyds init "Talk Title"    Scaffold a new presentation
  slyds serve [dir]          Start a local dev server
  slyds build [dir]          Inline assets → single HTML file

Options:
  -n, --slides <N>    Number of slides for init (default: 3)
  --local             Copy slyds.js/css locally instead of using CDN
  -p, --port <port>   Port for serve (default: 3000)
  -h, --help          Show this help
`);
  process.exit(0);
}

switch (command) {
  case 'init':
    require('../lib/commands/init')(args.slice(1));
    break;
  case 'serve':
    require('../lib/commands/serve')(args.slice(1));
    break;
  case 'build':
    require('../lib/commands/build')(args.slice(1));
    break;
  default:
    console.error(`Unknown command: ${command}\nRun "slyds --help" for usage.`);
    process.exit(1);
}
