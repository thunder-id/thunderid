#!/usr/bin/env node

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Generates Postman collections, environments, and globals from OpenAPI specifications.
 * Product name and slug are derived from docusaurus.product.config.ts to keep the
 * product name easily changeable.
 *
 * Usage:
 *   node scripts/generate-postman-collections.mjs [--version-path <path>]
 *
 * Options:
 *   --version-path  The versioned subdirectory under static/api/ to write output into.
 *                   Defaults to 'next' (the unreleased/current version).
 *
 * Output layout under static/api/<versionPath>/postman/:
 *   collections/
 *     <slug>-api-postman-collection.json   — single merged collection
 *     modules/                             — individual collections per spec
 *       <name>.postman.json
 *
 * Note: Only top-level YAML files in api/ are processed. Subdirectories (e.g. WIP/)
 * are intentionally skipped to match the stable specs served in the docs.
 */

import {readFileSync, writeFileSync, readdirSync, existsSync, mkdirSync} from 'fs';
import {join, dirname} from 'path';
import {fileURLToPath} from 'url';
import {promisify} from 'util';
import {createLogger} from '@thunderid/logger';
import Converter from 'openapi-to-postmanv2';

const convert = promisify(Converter.convert.bind(Converter));

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const logger = createLogger('generate-postman-collections');

const API_DIR = join(__dirname, '..', '..', 'api');
const STATIC_DIR = join(__dirname, '..', 'static', 'api');
const PRODUCT_CONFIG_PATH = join(__dirname, '..', 'docusaurus.product.config.ts');

// Resolve version path from --version-path <path> CLI arg, defaulting to 'next'
const versionPathArgIndex = process.argv.indexOf('--version-path');
const versionPath = versionPathArgIndex !== -1 ? process.argv[versionPathArgIndex + 1] : 'next';

const OUTPUT_DIR = join(STATIC_DIR, versionPath, 'postman');

// Spec files to skip (WIP / not yet stable, or APIs the server does not register)
const SKIP_FILES = new Set(['design.yaml', 'scim.yaml']);

const CONVERT_OPTIONS = {
  folderStrategy: 'Tags',
  requestNameSource: 'Fallback',
  indentCharacter: '  ',
  collapseFolders: true,
  optimizeConversion: false,
  strictRequestNames: false,
  includeAuthInfoInExample: true,
  exampleParametersResolution: 'Schema',
  enableOptionalParameters: false,
  disabledParametersValidation: false,
  keepImplicitHeaders: false,
};

/**
 * Extract project name and slug from docusaurus.product.config.ts using regex so the
 * script stays free of a TypeScript compilation step. The name drives display strings.
 */
function readProductConfig(configPath) {
  const content = readFileSync(configPath, 'utf8');

  // Match the top-level `name:` field inside the `project:` block  (first string value)
  const nameMatch = content.match(/project\s*:\s*\{[^}]*?name\s*:\s*['"]([^'"]+)['"]/s);
  const projectName = nameMatch ? nameMatch[1] : 'Unknown Project';
  const projectSlug = projectName.toLowerCase();
  const projectEmoji = content.match(/project\s*:\s*\{[^}]*emoji\s*:\s*['"]([^'"]+)['"]/s)?.[1] || '';

  const fileNameMatch = content.match(/output\s*:\s*['"]([^'"]+)['"]/);
  const collectionFileName = fileNameMatch ? fileNameMatch[1] : `${projectSlug}-api-postman-collection.json`;

  return {projectName, projectSlug, projectEmoji, collectionFileName};
}

/**
 * Ensure a directory exists, creating it recursively if needed.
 */
function ensureDir(dirPath) {
  if (!existsSync(dirPath)) {
    mkdirSync(dirPath, {recursive: true});
  }
}

/**
 * Replace hardcoded OAuth2 URLs with collection variable references.
 */
function replaceOAuth2Urls(obj) {
  if (obj === null || typeof obj !== 'object') return;

  if (Array.isArray(obj)) {
    for (const item of obj) replaceOAuth2Urls(item);
    return;
  }

  if ((obj.key === 'accessTokenUrl' || obj.key === 'authUrl') && typeof obj.value === 'string') {
    obj.value = obj.value.replace(/https?:\/\/localhost:\d+/g, '{{baseUrl}}');
  }

  for (const value of Object.values(obj)) replaceOAuth2Urls(value);
}

/**
 * Rewrite the OAuth2 auth block from authorization_code to client_credentials.
 * Client credentials requires only a token endpoint POST — no browser redirect —
 * making it far simpler for API testing workflows.
 */
function rewriteAuthToClientCredentials(collection) {
  if (collection?.auth?.type !== 'oauth2') return;
  collection.auth.oauth2 = [
    {key: 'grant_type', value: 'client_credentials'},
    {key: 'accessTokenUrl', value: '{{baseUrl}}/oauth2/token'},
    {key: 'clientId', value: '{{clientId}}'},
    {key: 'clientSecret', value: '{{clientSecret}}'},
    {key: 'scope', value: 'system'},
    {key: 'addTokenTo', value: 'header'},
  ];

  const existing = new Set((collection.variable ?? []).map((v) => v.key));
  if (!existing.has('clientId')) {
    collection.variable.push({key: 'clientId', value: '', type: 'string'});
  }
  if (!existing.has('clientSecret')) {
    collection.variable.push({key: 'clientSecret', value: '', type: 'string'});
  }
}

/**
 * Convert a single OpenAPI spec file to a Postman collection.
 */
async function convertSpec(specPath) {
  const specContent = readFileSync(specPath, 'utf8');
  const result = await convert({type: 'string', data: specContent}, CONVERT_OPTIONS);

  if (!result.result) {
    throw new Error(`Conversion failed for ${specPath}: ${result.reason}`);
  }

  const collection = result.output[0].data;
  replaceOAuth2Urls(collection);
  rewriteAuthToClientCredentials(collection);

  return collection;
}

/**
 * Generate individual Postman collections for each OpenAPI spec, writing them
 * flat under the modules/ subdirectory of collectionsDir.
 */
async function generateModuleCollections(specFiles, collectionsDir) {
  const collections = [];
  const modulesDir = join(collectionsDir, 'modules');

  ensureDir(modulesDir);

  for (const file of specFiles) {
    const specPath = join(API_DIR, file);
    const specName = file.replace(/\.yaml$/, '');
    const outputPath = join(modulesDir, `${specName}.postman.json`);

    logger.info(`  Converting ${file}...`);
    const collection = await convertSpec(specPath);

    writeFileSync(outputPath, JSON.stringify(collection, null, 2), 'utf8');
    logger.info(`  Written to ${outputPath}`);

    collections.push({name: specName, collection});
  }

  return collections;
}

/**
 * Generate a single combined Postman collection from all individual collections.
 *
 * Merges all items (folders/requests) into one top-level collection.
 * Variables are merged and deduplicated across all collections (first occurrence wins).
 */
function generateCombinedCollection(collections, projectName, projectEmoji) {
  const combined = {
    info: {
      name: projectEmoji ? `${projectEmoji} ${projectName} API` : `${projectName} API`,
      description: `Complete API collection for ${projectName} identity and access management.`,
      schema: 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json',
    },
    item: [],
    variable: [],
  };

  // Merge variables — deduplicate by key, first occurrence wins.
  const mergedVariables = new Map();

  for (const {collection} of collections) {
    for (const variable of collection.variable ?? []) {
      if (variable?.key && !mergedVariables.has(variable.key)) {
        mergedVariables.set(variable.key, variable);
      }
    }
  }

  combined.variable = Array.from(mergedVariables.values());

  const authSource = collections.find(({collection}) => collection.auth)?.collection;

  if (authSource?.auth) {
    combined.auth = authSource.auth;
  }

  for (const {collection} of collections) {
    if (collection.item) {
      combined.item.push(...collection.item);
    }
  }

  return combined;
}

async function main() {
  logger.info(`Generating Postman collections (version path: ${versionPath})...`);

  const {projectName, projectSlug, projectEmoji, collectionFileName} = readProductConfig(PRODUCT_CONFIG_PATH);

  logger.info(`Product: ${projectEmoji} ${projectName} (slug: ${projectSlug})`);

  // Only top-level YAML files are processed — subdirectories are intentionally skipped.
  const specFiles = readdirSync(API_DIR)
    .filter((file) => file.endsWith('.yaml') && !SKIP_FILES.has(file))
    .sort();

  if (specFiles.length === 0) {
    throw new Error('No OpenAPI spec files found in the api/ directory.');
  }

  logger.info(`Found ${specFiles.length} spec file(s)`);

  const collectionsDir = join(OUTPUT_DIR, 'collections');

  ensureDir(collectionsDir);

  logger.info('Generating module collections...');
  const collections = await generateModuleCollections(specFiles, collectionsDir);

  logger.info('Generating combined collection...');
  const combined = generateCombinedCollection(collections, projectName, projectEmoji);
  const combinedPath = join(collectionsDir, collectionFileName);

  writeFileSync(combinedPath, JSON.stringify(combined, null, 2), 'utf8');
  logger.info(`Combined collection written to ${combinedPath}`);

  logger.info(`Done. ${collections.length} module collection(s) + 1 combined collection generated.`);
}

main().catch((error) => {
  logger.error('Error generating Postman collections:', error);
  process.exit(1);
});
