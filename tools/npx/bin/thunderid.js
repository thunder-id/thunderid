#!/usr/bin/env node
/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

const { spawnSync } = require('child_process');
const path = require('path');
const fs = require('fs');

const PRODUCT_NAME_LOWERCASE = 'thunderid';
const PLATFORM_MAP = { darwin: 'darwin', linux: 'linux', win32: 'win' };
const ARCH_MAP = { x64: 'x64', arm64: 'arm64' };

const platform = PLATFORM_MAP[process.platform];
const arch = ARCH_MAP[process.arch];

if (!platform || !arch) {
  process.stderr.write(`Unsupported platform: ${process.platform}/${process.arch}\n`);
  process.exit(1);
}

const ext = process.platform === 'win32' ? '.exe' : '';
const binaryName = `${PRODUCT_NAME_LOWERCASE}-${platform}-${arch}${ext}`;
const binaryPath = path.join(__dirname, '..', 'dist', binaryName);

if (!fs.existsSync(binaryPath)) {
  // Dev fallback: run via `go run` if the pre-built binary is absent.
  const cliDir = path.join(__dirname, '..', 'cli');
  if (fs.existsSync(path.join(cliDir, 'go.mod'))) {
    const result = spawnSync('go', ['run', '.', ...process.argv.slice(2)], {
      cwd: cliDir,
      stdio: 'inherit',
      env: process.env,
    });
    process.exit(result.status ?? 1);
  }
  process.stderr.write(
    `Binary not found: ${binaryPath}\nRun "pnpm build" to compile the Go CLI.\n`,
  );
  process.exit(1);
}

if (process.platform !== 'win32') {
  try {
    fs.chmodSync(binaryPath, 0o755);
  } catch {
    // ignore — may already be executable
  }
}

const result = spawnSync(binaryPath, process.argv.slice(2), {
  stdio: 'inherit',
  env: process.env,
});

process.exit(result.status ?? 1);
