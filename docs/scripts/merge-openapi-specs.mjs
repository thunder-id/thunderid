#!/usr/bin/env node

// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {readFileSync, writeFileSync, readdirSync, existsSync, mkdirSync} from 'fs';
import {join, dirname} from 'path';
import {fileURLToPath} from 'url';
import {createLogger} from '@thunderid/logger';
import {parse, stringify} from 'yaml';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const logger = createLogger('merge-openapi-specs');

const API_DIR = join(__dirname, '..', '..', 'api');
const STATIC_DIR = join(__dirname, '..', 'static', 'api');
const PRODUCT_CONFIG_PATH = join(__dirname, '..', 'docusaurus.product.config.ts');

function readProductConfig(configPath) {
  const content = readFileSync(configPath, 'utf8');
  const nameMatch = content.match(/project\s*:\s*\{[^}]*?name\s*:\s*['"]([^'"]+)['"]/s);
  return nameMatch ? nameMatch[1] : 'Unknown Project';
}

const projectName = readProductConfig(PRODUCT_CONFIG_PATH);

// Resolve version path from --version-path <path> CLI arg, defaulting to 'next'
const versionPathArgIndex = process.argv.indexOf('--version-path');
const versionPath = versionPathArgIndex !== -1 ? process.argv[versionPathArgIndex + 1] : 'next';

const OUTPUT_FILE = join(STATIC_DIR, versionPath, 'combined.yaml');

const GROUP_CONFIG_PATH = join(__dirname, '..', 'api-groups.config.yaml');

function loadGroupConfig(configPath) {
  const config = parse(readFileSync(configPath, 'utf8'));
  return {
    hiddenTags: new Set(config.hiddenTags ?? []),
    fileGroupNames: config.fileGroupNames ?? {},
    explicitSubGroups: config.explicitSubGroups ?? [],
    groupOrder: config.groupOrder ?? [],
  };
}

function mergeOpenAPISpecs() {
  logger.info(`🔄 Merging OpenAPI specifications (version path: ${versionPath})...`);

  const {hiddenTags, fileGroupNames, explicitSubGroups, groupOrder} = loadGroupConfig(GROUP_CONFIG_PATH);

  // Dynamically read all YAML files from the API directory
  const API_FILES = readdirSync(API_DIR)
    .filter((file) => file.endsWith('.yaml') && file !== 'combined.yaml')
    .sort();

  if (API_FILES.length === 0) {
    throw new Error('No API specification files found in the directory.');
  }

  logger.info(`📁 Found ${API_FILES.length} API specification files`);

  // Base structure from the first spec
  const firstSpec = parse(readFileSync(join(API_DIR, API_FILES[0]), 'utf8'));

  const combined = {
    openapi: firstSpec.openapi || '3.0.3',
    info: {
      title: `${projectName} API Reference`,
      version: '1.0',
      description: `Complete API reference for ${projectName} identity and access management.`,
      license: firstSpec.info?.license || {
        name: 'Apache 2.0',
        url: 'https://www.apache.org/licenses/LICENSE-2.0.html',
      },
    },
    // Explicitly defined so the output is stable regardless of which spec file
    // happens to sort first. Every spec uses the same server URL and security scheme.
    servers: [
      {
        url: 'https://{host}:{port}',
        variables: {
          host: {default: 'localhost'},
          port: {default: '8090'},
        },
      },
    ],
    security: [{OAuth2: ['system']}],
    tags: [],
    paths: {},
    components: {
      schemas: {},
      securitySchemes: {},
      responses: {},
      parameters: {},
    },
  };

  const tagSourceFile = {};

  // Process each API spec
  API_FILES.forEach((file) => {
    logger.info(`  ➜ Processing ${file}...`);
    const specPath = join(API_DIR, file);
    const spec = parse(readFileSync(specPath, 'utf8'));

    // Merge tags
    if (spec.tags) {
      spec.tags.forEach((tag) => {
        if (!combined.tags.find((t) => t.name === tag.name)) {
          combined.tags.push(tag);
        }
        if (!tagSourceFile[tag.name]) {
          tagSourceFile[tag.name] = file;
        }
      });
    }

    // Merge paths
    if (spec.paths) {
      const specSecurity = Object.prototype.hasOwnProperty.call(spec, 'security') ? spec.security : undefined;

      Object.entries(spec.paths).forEach(([path, pathItem]) => {
        let resolvedPathItem = pathItem;

        if (specSecurity !== undefined) {
          resolvedPathItem = {...pathItem};
          for (const method of ['get', 'post', 'put', 'delete', 'patch', 'head', 'options', 'trace']) {
            const op = resolvedPathItem[method];
            if (op && !Object.prototype.hasOwnProperty.call(op, 'security')) {
              resolvedPathItem[method] = {...op, security: specSecurity};
            }
          }
        }

        if (combined.paths[path]) {
          combined.paths[path] = {...combined.paths[path], ...resolvedPathItem};
        } else {
          combined.paths[path] = resolvedPathItem;
        }
      });
    }

    // Merge components
    if (spec.components) {
      Object.entries(spec.components).forEach(([section, entries]) => {
        combined.components[section] = {
          ...combined.components[section],
          ...entries,
        };
      });
    }
  });

  // Sort tags alphabetically
  combined.tags.sort((a, b) => a.name.localeCompare(b.name));

  // Build sidebar tag groups. All groups are sorted by groupOrder from config.
  // Remaining visible tags auto-derive a group from their source spec filename.
  // Tags in hiddenTags or whose source file maps to null in fileGroupNames are excluded.
  const claimedTags = new Set(explicitSubGroups.flatMap((g) => g.tags));
  const autoGroupMap = new Map();

  for (const [tag, sourceFile] of Object.entries(tagSourceFile)) {
    if (hiddenTags.has(tag) || claimedTags.has(tag)) continue;

    const mappedName = fileGroupNames[sourceFile];
    if (mappedName === null) continue;

    // Files not listed in fileGroupNames fall back to "Other" rather than
    // deriving a Title Case name from the filename. This prevents unconfigured
    // spec files from silently creating stray sidebar groups.
    const groupName = mappedName ?? 'Other';
    if (!autoGroupMap.has(groupName)) autoGroupMap.set(groupName, []);
    autoGroupMap.get(groupName).push(tag);
  }

  // Warn about tags suppressed by a null fileGroupNames entry but not claimed
  // by any explicitSubGroup — they will be hidden from the sidebar entirely.
  const allGroupedTags = new Set([...claimedTags, ...hiddenTags]);
  for (const tags of autoGroupMap.values()) {
    tags.forEach((t) => allGroupedTags.add(t));
  }
  for (const {name} of combined.tags) {
    if (!allGroupedTags.has(name)) {
      const sourceFile = tagSourceFile[name] ?? 'unknown';
      logger.warn(
        `⚠️  Tag "${name}" (from ${sourceFile}) is suppressed by a null fileGroupNames entry but not claimed by any explicitSubGroup — it will be hidden from the sidebar. Either claim it in explicitSubGroups or remove the null entry.`,
      );
    }
  }

  // Log a notice for any tags that fell into "Other" so developers know a
  // spec file needs to be wired up in api-groups.config.yaml.
  const otherTags = autoGroupMap.get('Other');
  if (otherTags?.length) {
    logger.warn(
      `⚠️  ${otherTags.length} tag(s) have no api-groups.config.yaml entry and will appear under "Other": ${otherTags.join(', ')}. Add the source spec file(s) to fileGroupNames to assign them properly.`,
    );
  }

  // Combine all groups then sort by groupOrder. Groups absent from groupOrder
  // are appended alphabetically after all ordered entries.
  const allGroups = [...explicitSubGroups, ...[...autoGroupMap.entries()].map(([name, tags]) => ({name, tags}))];

  if (groupOrder.length > 0) {
    const orderIndex = new Map(groupOrder.map((name, i) => [name, i]));
    allGroups.sort((a, b) => {
      const ai = orderIndex.has(a.name) ? orderIndex.get(a.name) : Infinity;
      const bi = orderIndex.has(b.name) ? orderIndex.get(b.name) : Infinity;
      if (ai !== bi) return ai - bi;
      return a.name.localeCompare(b.name);
    });
  }

  combined['x-tagGroups'] = allGroups;

  // Ensure the output directory exists (including any version subdirectory)
  const outputDir = dirname(OUTPUT_FILE);

  if (!existsSync(outputDir)) {
    mkdirSync(outputDir, {recursive: true});
    logger.info(`📁 Created output directory: ${outputDir}`);
  }

  // Write the combined spec
  writeFileSync(OUTPUT_FILE, stringify(combined), 'utf8');
  logger.info(`✅ Combined API spec written to ${OUTPUT_FILE}`);
  logger.info(`📊 Stats: ${combined.tags.length} tags, ${Object.keys(combined.paths).length} paths`);
}

try {
  mergeOpenAPISpecs();
} catch (error) {
  logger.error('❌ Error merging API specs:', error);
  process.exit(1);
}
