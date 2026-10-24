// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {execSync} from 'child_process';
import {readFileSync} from 'fs';
import {dirname, join} from 'path';
import {fileURLToPath} from 'url';
import {createLogger} from '@thunderid/logger';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const PRODUCT_CONFIG_PATH = join(__dirname, '..', 'docusaurus.product.config.ts');

function readProductConfig(configPath) {
  const content = readFileSync(configPath, 'utf8');
  const nameMatch = content.match(/project\s*:\s*\{[^}]*?name\s*:\s*['"]([^'"]+)['"]/s);
  const projectName = nameMatch ? nameMatch[1] : 'Unknown Project';
  const emojiMatch = content.match(/project\s*:\s*\{[^}]*emoji\s*:\s*['"]([^'"]+)['"]/s);
  const projectEmoji = emojiMatch ? emojiMatch[1] : '';
  return {projectName, projectEmoji};
}

const {projectName, projectEmoji} = readProductConfig(PRODUCT_CONFIG_PATH);

const logger = createLogger('prebuild');

/**
 * Execute a command and handle errors
 */
function executeScript(scriptName, scriptPath) {
  logger.info(`\n🔄 Running ${scriptName}...`);
  try {
    execSync(`node ${scriptPath}`, {
      stdio: 'inherit',
      cwd: join(__dirname, '..'),
      env: process.env,
    });
    logger.info(`✅ ${scriptName} completed successfully\n`);
  } catch (error) {
    logger.error(`❌ ${scriptName} failed: ${error.message}`);
    process.exit(1);
  }
}

/**
 * Main function to generate all documentation artifacts
 */
async function generateDocs() {
  logger.info(`${projectEmoji} ${projectName} Documentation Generator\n`);
  logger.info('Generating documentation artifacts...\n');

  // Generate OpenAPI specs
  executeScript('API Specs Generator', join(__dirname, 'merge-openapi-specs.mjs'));

  // Generate Postman collections from OpenAPI specs
  executeScript('Postman Collections Generator', join(__dirname, 'generate-postman-collections.mjs'));

  // Generate changelog
  executeScript('Changelog Generator', join(__dirname, 'generate-changelog.mjs'));

  // Generate contributors
  executeScript('Contributors Generator', join(__dirname, 'generate-contributors.mjs'));

  // Generate ecosystem (SDK, sample) release data
  executeScript('Ecosystem Releases Generator', join(__dirname, 'generate-ecosystem-releases.mjs'));

  // Copy LLM prompt files into static/docs/
  executeScript('Prompts Generator', join(__dirname, 'generate-prompts.mjs'));

  logger.info('🎉 All documentation artifacts generated successfully!\n');
}

generateDocs();
