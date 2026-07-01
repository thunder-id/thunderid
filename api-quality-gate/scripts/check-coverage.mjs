#!/usr/bin/env node
// check-coverage.mjs
//
// Guard against the "escapes by simply not matching" failure mode: every
// operation in the spec must (a) have an operationId, and (b) be referenced by
// at least one contract test. A new endpoint that ships with no test fails CI.
//
// Heuristic for (b): grep the contract-tests dir for the operationId string.
// Adjust CONTRACT_DIR / FILE_GLOB to your harness.
//
// Usage: node scripts/check-coverage.mjs <openapi-spec-path>
// Deps: js-yaml

import fs from 'node:fs';
import path from 'node:path';
import yaml from 'js-yaml';

const SPEC = process.argv[2] || 'api/openapi.yaml';
const CONTRACT_DIR = process.env.CONTRACT_DIR || 'contract-tests';
const METHODS = ['get', 'post', 'put', 'patch', 'delete', 'head', 'options'];

const fail = (msg) => {
  console.error(`\u2717 ${msg}`);
  process.exit(1);
};

if (!fs.existsSync(SPEC)) fail(`Spec not found: ${SPEC}`);
const spec = yaml.load(fs.readFileSync(SPEC, 'utf8'));

const operations = [];
for (const [p, item] of Object.entries(spec.paths || {})) {
  for (const m of METHODS) {
    if (item && item[m]) operations.push({ p, m, operationId: item[m].operationId });
  }
}

const missingId = operations.filter((o) => !o.operationId);
if (missingId.length) {
  for (const o of missingId) console.error(`  ${o.m.toUpperCase()} ${o.p} has no operationId`);
  fail(`${missingId.length} operation(s) missing operationId.`);
}

// gather all contract-test text
let haystack = '';
const walk = (dir) => {
  if (!fs.existsSync(dir)) return;
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full);
    else haystack += '\n' + fs.readFileSync(full, 'utf8');
  }
};
walk(CONTRACT_DIR);

const untested = operations.filter((o) => !haystack.includes(o.operationId));
if (untested.length) {
  console.error(`No contract test references found in ${CONTRACT_DIR}/ for:`);
  for (const o of untested) console.error(`  ${o.operationId} (${o.m.toUpperCase()} ${o.p})`);
  fail(`${untested.length} operation(s) without contract-test coverage.`);
}

console.log(`\u2713 ${operations.length} operation(s), all have operationId and contract-test coverage.`);
