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

const { execSync } = require('child_process');
const fs = require('fs');
const path = require('path');

const cliDir = path.resolve(__dirname, '../../cli');
const cliDist = path.join(cliDir, 'dist');
const npxDist = path.resolve(__dirname, '../dist');

if (process.platform === 'win32') {
  execSync(
    `powershell.exe -ExecutionPolicy Bypass -File "${path.join(cliDir, 'scripts', 'build.ps1')}"`,
    { stdio: 'inherit' },
  );
} else {
  execSync(`bash "${path.join(cliDir, 'scripts', 'build.sh')}"`, {
    stdio: 'inherit',
  });
}

fs.mkdirSync(npxDist, { recursive: true });
for (const file of fs.readdirSync(cliDist)) {
  fs.copyFileSync(path.join(cliDist, file), path.join(npxDist, file));
}

console.log('Done. Binaries available in npx/dist/');
