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

import * as path from 'path';
import {intro, outro, text, spinner, note, cancel, isCancel} from '@clack/prompts';
import colors from 'picocolors';
import Product from './constants/Product';
import {deploy} from './deploy';
import {downloadAndExtract, getLatestThunderVersion} from './download';
import {findThunderRoot, runSetup, runStart} from './setup';
import {readState, writeState, markSetupComplete} from './state';

function parseCliArgs(argv: string[]): {forceSetup: boolean; installDir?: string; forwardedArgs: string[]} {
  let forceSetup = false;
  let installDir: string | undefined;
  const forwardedArgs: string[] = [];

  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i];
    if (arg === '--setup') {
      forceSetup = true;
    } else if (arg === '--install-dir') {
      if (i + 1 >= argv.length) {
        process.stderr.write('Error: --install-dir requires a value\n');
        process.exit(1);
      }
      installDir = argv[++i];
    } else {
      forwardedArgs.push(arg);
    }
  }

  return {forceSetup, installDir, forwardedArgs};
}

async function main(): Promise<void> {
  const rawArgs = process.argv.slice(2);
  if (rawArgs[0] === 'deploy') {
    await deploy();
    return;
  }

  // eslint-disable-next-line no-console
  console.clear();

  const {forceSetup, installDir: cliInstallDir, forwardedArgs} = parseCliArgs(rawArgs);

  const s = spinner();
  s.start('Fetching latest Thunder release...');
  let VERSION: string;
  try {
    VERSION = await getLatestThunderVersion();
    s.stop(`Latest Thunder release: v${VERSION}`);
  } catch (err) {
    s.stop('Could not fetch latest Thunder release.');
    process.stderr.write(`\nError: ${(err as Error).message}\n`);
    process.exit(1);
  }

  const state = readState();
  const versionState = state.installs[VERSION];
  const alreadyInstalled = Boolean(versionState?.installPath && findThunderRoot(versionState.installPath) !== null);

  const BRAND_BLUE = '\x1b[38;2;54;136;255m';
  const RESET = '\x1b[0m';
  const GREY = '\x1b[38;2;128;128;128m';

  const thunderLines = [
    ' _____ _                     _           ',
    '|_   _| |                   | |          ',
    '  | | | |__  _   _ _ __   __| | ___ _ __ ',
    "  | | | '_ \\| | | | '_ \\ / _` |/ _ \\ '__|",
    '  | | | | | | |_| | | | | (_| |  __/ |   ',
    '  \\_/ |_| |_|\\__,_|_| |_|\\__,_|\\___|_|   ',
  ];
  const idLines = [
    ' ___________ ',
    '|_   _|  _  \\',
    '  | | | | | |',
    '  | | | | | |',
    ' _| |_| |/ / ',
    ' \\___/|___/  ',
  ];
  const banner = thunderLines.map((t, i) => `  ${BRAND_BLUE}${t}${RESET}${GREY}${idLines[i]}${RESET}`).join('\n');

  intro(`\n${banner}\n\n${colors.dim('· High-performance open-source identity stack, engineered for developers')}\n`);

  let installPath: string;

  if (alreadyInstalled && versionState.setupComplete && !forceSetup) {
    installPath = versionState.installPath;
    note(`${Product.NAME} v${VERSION} is ready\n${installPath}`, `Starting ${Product.NAME}`);
    try {
      runStart(installPath, forwardedArgs);
    } catch (err) {
      process.stderr.write(`\nFailed to start ${Product.NAME}: ${(err as Error).message}\n`);
      process.exit(1);
    }
    return;
  }

  if (alreadyInstalled) {
    installPath = versionState.installPath;
    if (forceSetup) {
      note(`Re-running setup for ${Product.NAME} v${VERSION}\n${installPath}`, 'Setup requested');
    } else {
      note(`Using ${Product.NAME} v${VERSION}\n${installPath}`, 'Already installed');
    }
  } else {
    const defaultPath = path.join(process.cwd(), VERSION);
    const nonInteractivePath = cliInstallDir ?? process.env.THUNDER_INSTALL_DIR;

    if (nonInteractivePath) {
      installPath = nonInteractivePath;
    } else {
      const rawInstallPath = await text({
        message: 'Install directory',
        placeholder: defaultPath,
        defaultValue: defaultPath,
      });

      if (isCancel(rawInstallPath)) {
        cancel('Installation cancelled.');
        process.exit(0);
      }

      installPath = rawInstallPath || defaultPath;
    }

    const dl = spinner();
    dl.start(`Downloading Thunder v${VERSION}...`);

    try {
      await downloadAndExtract(VERSION, installPath, (msg) => dl.message(msg));
    } catch (err) {
      dl.stop('Download failed.');
      process.stderr.write(`\nError: ${(err as Error).message}\n`);
      process.exit(1);
    }

    dl.stop(`${Product.NAME} v${VERSION} installed to ${installPath}`);
    writeState(VERSION, installPath);

    outro(`Running ${Product.NAME} setup for the first time...`);
  }

  try {
    runSetup(installPath, forwardedArgs);
    markSetupComplete(VERSION);
  } catch (err) {
    process.stderr.write(`\nSetup failed: ${(err as Error).message}\n`);
    process.exit(1);
  }

  note(`Setup complete for ${Product.NAME} v${VERSION}\n${installPath}`, `Starting ${Product.NAME}`);

  try {
    runStart(installPath, forwardedArgs);
  } catch (err) {
    process.stderr.write(`\nSetup succeeded but failed to start ${Product.NAME}: ${(err as Error).message}\n`);
    process.exit(1);
  }
}

void main();
