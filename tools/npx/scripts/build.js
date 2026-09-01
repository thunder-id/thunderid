// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const { execFileSync } = require('child_process');
const fs = require('fs');
const path = require('path');
const { version } = require('../package.json');

const cliDir = path.resolve(__dirname, '../../cli');
const cliDist = path.join(cliDir, 'dist');
const npxDist = path.resolve(__dirname, '../dist');

// The published package version is what `thunderid --version` reports, so pass it
// to the build scripts for injection via -ldflags.
const buildEnv = { ...process.env, VERSION: version };

if (process.platform === 'win32') {
  execFileSync(
    'powershell.exe',
    [
      '-ExecutionPolicy',
      'Bypass',
      '-File',
      path.join(cliDir, 'scripts', 'build.ps1'),
    ],
    { stdio: 'inherit', env: buildEnv },
  );
} else {
  execFileSync('bash', [path.join(cliDir, 'scripts', 'build.sh')], {
    stdio: 'inherit',
    env: buildEnv,
  });
}

fs.mkdirSync(npxDist, { recursive: true });
for (const file of fs.readdirSync(cliDist)) {
  fs.copyFileSync(path.join(cliDist, file), path.join(npxDist, file));
}

console.log('Done. Binaries available in npx/dist/');
