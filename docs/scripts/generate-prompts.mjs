#!/usr/bin/env node

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {existsSync, mkdirSync, readdirSync, copyFileSync} from 'fs';
import {join, dirname, relative, sep} from 'path';
import {fileURLToPath} from 'url';
import {createLogger} from '@thunderid/logger';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const logger = createLogger('generate-prompts');

// LLM prompt .txt files live alongside the docs page they belong to
// (e.g. content/getting-started/connect-your-application/prompts/react/redirect-based.txt)
// so they stay versioned with the docs. They're mirrored here into static/docs/<version>/...
// so Docusaurus serves them as plain, CORS-fetchable .txt files (the console fetches them
// directly), matching the URL scheme used for every other doc-relative link in config.js.
const PROMPT_ROOT_SEGMENT = 'prompts';

// `versionPath` must match each version's served base path in docusaurus.config.ts:
// the current/"Next" docs live at /docs/next, and v1.0.x is the lastVersion served at
// the bare /docs root (empty segment).
const VERSION_ROOTS = [
  {sourceDir: join(__dirname, '..', 'content'), versionPath: 'next'},
  {sourceDir: join(__dirname, '..', 'versioned_docs', 'version-v1.0.x'), versionPath: ''},
];

const STATIC_DOCS_DIR = join(__dirname, '..', 'static', 'docs');

/** Recursively collect every file living under a `prompts/` directory. */
function findPromptFiles(dir, baseDir = dir) {
  const files = [];
  if (!existsSync(dir)) return files;

  for (const entry of readdirSync(dir, {withFileTypes: true})) {
    const fullPath = join(dir, entry.name);
    if (entry.isDirectory()) {
      files.push(...findPromptFiles(fullPath, baseDir));
    } else if (entry.isFile()) {
      const relativePath = relative(baseDir, fullPath);
      if (relativePath.split(sep).includes(PROMPT_ROOT_SEGMENT)) {
        files.push({fullPath, relativePath});
      }
    }
  }
  return files;
}

function generatePrompts() {
  logger.info('🔄 Copying LLM prompt files into static/docs/...');

  let written = 0;

  for (const {sourceDir, versionPath} of VERSION_ROOTS) {
    for (const {fullPath, relativePath} of findPromptFiles(sourceDir)) {
      const outputPath = join(STATIC_DOCS_DIR, versionPath, relativePath);
      mkdirSync(dirname(outputPath), {recursive: true});
      copyFileSync(fullPath, outputPath);
      written++;
    }
  }

  logger.info(`✅ Copied ${written} prompt file(s) into static/docs/{${VERSION_ROOTS.map((v) => v.versionPath || '(root)').join(',')}}`);
}

try {
  generatePrompts();
} catch (error) {
  logger.error('❌ Error copying prompt files:', error);
  process.exit(1);
}
