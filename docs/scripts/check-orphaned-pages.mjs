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

/**
 * Checks for doc pages that exist on disk but have no sidebar entry.
 * Exits with code 1 if any orphaned pages are found.
 *
 * Usage: node docs/scripts/check-orphaned-pages.mjs
 */

import {readFileSync, readdirSync, statSync} from 'fs';
import {join} from 'path';
import {fileURLToPath} from 'url';

const __dirname = fileURLToPath(new URL('.', import.meta.url));
const CONTENT_DIR = join(__dirname, '..', 'content');
const SIDEBARS_FILE = join(__dirname, '..', 'sidebars.ts');

// IDs that are intentionally excluded from the sidebar
const EXCLUDED_IDS = new Set([
  'index', // docs home
]);

function getAllDocIds(dir, base = '') {
  const ids = [];
  for (const entry of readdirSync(dir)) {
    const fullPath = join(dir, entry);
    const rel = base ? `${base}/${entry}` : entry;
    if (statSync(fullPath).isDirectory()) {
      ids.push(...getAllDocIds(fullPath, rel));
    } else if (entry.endsWith('.mdx') || entry.endsWith('.md')) {
      ids.push(rel.replace(/\.(mdx|md)$/, ''));
    }
  }
  return ids;
}

function getSidebarIds(sidebarsContent) {
  const ids = new Set();
  const idRegex = /\bid:\s*['"]([^'"]+)['"]/g;
  let match;
  while ((match = idRegex.exec(sidebarsContent)) !== null) {
    ids.add(match[1]);
  }
  return ids;
}

function collectAllSidebarIds() {
  const ids = new Set();
  const content = readFileSync(SIDEBARS_FILE, 'utf-8');
  for (const id of getSidebarIds(content)) ids.add(id);

  function scanDir(dir) {
    for (const entry of readdirSync(dir)) {
      const fullPath = join(dir, entry);
      if (statSync(fullPath).isDirectory()) {
        scanDir(fullPath);
      } else if (entry === 'sidebar.ts' || entry === 'sidebar.js') {
        const fileContent = readFileSync(fullPath, 'utf-8');
        for (const id of getSidebarIds(fileContent)) ids.add(id);
      }
    }
  }
  scanDir(CONTENT_DIR);
  return ids;
}

const allIds = getAllDocIds(CONTENT_DIR);
const sidebarIds = collectAllSidebarIds();

const orphaned = allIds.filter(
  (id) => !sidebarIds.has(id) && !EXCLUDED_IDS.has(id)
);

if (orphaned.length === 0) {
  console.log('✅ No orphaned pages found.');
  process.exit(0);
} else {
  console.error(`❌ Found ${orphaned.length} orphaned page(s) not in any sidebar:\n`);
  for (const id of orphaned.sort()) {
    console.error(`  - ${id}`);
  }
  console.error('\nAdd them to sidebars.ts or mark them with sidebar: false in frontmatter.');
  process.exit(1);
}
