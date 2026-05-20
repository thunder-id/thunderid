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

/* eslint-disable @thunderid/copyright-header */

// TODO: We can remove this once NX sorts out their current limitations for including folder outside the workspace.
// Tracker: https://github.com/thunder-id/thunderid/issues/1199

import {spawn} from 'node:child_process';
import {resolve, join} from 'node:path';
import process from 'node:process';
import {createLogger} from '@thunderid/logger';

const logger = createLogger({level: 'info'});
const docsDir = resolve(process.cwd(), join('..', 'docs'));

logger.info('Building docs...', {docsDir});

const child = spawn('pnpm', ['run', 'build'], {
  cwd: docsDir,
  stdio: 'inherit',
  shell: true,
});

child.on('exit', (code) => {
  if (code !== 0) {
    logger.error('Docs build failed', {exitCode: code});
    process.exit(code);
  }

  logger.info('Docs build completed');
});
