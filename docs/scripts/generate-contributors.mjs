// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {existsSync, mkdirSync, writeFileSync} from 'fs';
import {join, dirname} from 'path';
import {fileURLToPath} from 'url';
import {createLogger} from '@thunderid/logger';
import DocusaurusProductConfig from '../docusaurus.product.config.ts';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const OUTPUT_FILE = join(__dirname, '..', 'static', 'data', 'contributors.json');

const GITHUB_REPO = DocusaurusProductConfig.project.source.github.fullName;
const PROJECT_NAME = DocusaurusProductConfig.project.name;
const GITHUB_REPO_API_URL = `https://api.github.com/repos/${GITHUB_REPO}`;
const GITHUB_CONTRIBUTORS_API_URL = `${GITHUB_REPO_API_URL}/contributors`;

const logger = createLogger('generate-contributors');

const IGNORED_LOGINS = new Set(
  [
    '123',
    'asgardeo',
    'copilot',
    'dependabot',
    'example',
    'renovate',
    'thunder',
    'wso2',
    '7',
  ].map((u) => u.toLowerCase()),
);

function getGitHubHeaders() {
  return {
    'User-Agent': `${PROJECT_NAME}-Docs-Contributors-Generator`,
    ...(process.env.GITHUB_TOKEN ? {Authorization: `token ${process.env.GITHUB_TOKEN}`} : {}),
  };
}

async function fetchJsonWithHeaders(url) {
  const response = await fetch(url, {headers: getGitHubHeaders()});

  if (!response.ok) {
    const error = new Error(`Failed to fetch ${url}: ${response.status} ${response.statusText}`);

    error.status = response.status;
    throw error;
  }

  return {body: await response.json(), headers: response.headers};
}

async function fetchAllContributors() {
  logger.info(`Fetching contributors from ${GITHUB_CONTRIBUTORS_API_URL}...`);

  const contributors = [];
  let nextUrl = `${GITHUB_CONTRIBUTORS_API_URL}?per_page=100&anon=false`;

  while (nextUrl) {
    const {body, headers} = await fetchJsonWithHeaders(nextUrl);

    if (!Array.isArray(body)) {
      throw new Error(`Unexpected response format from ${nextUrl}`);
    }

    contributors.push(...body);

    const linkHeader = headers.get('link') || '';
    const nextLinkMatch = linkHeader.match(/<([^>]+)>\s*;\s*rel="next"/i);

    nextUrl = nextLinkMatch ? nextLinkMatch[1] : null;
  }

  return contributors;
}

function shouldInclude(contributor) {
  if (contributor.type === 'Bot') return false;
  const login = contributor.login.toLowerCase();

  return !IGNORED_LOGINS.has(login) && !login.includes('[bot]');
}

async function generate() {
  try {
    const all = await fetchAllContributors();
    const contributors = all
      .filter(shouldInclude)
      .map((c) => ({
        avatarUrl: c.avatar_url,
        contributions: c.contributions,
        htmlUrl: c.html_url,
        login: c.login,
      }));

    const totalCommits = contributors.reduce((sum, c) => sum + c.contributions, 0);
    const data = {
      contributors,
      generatedAt: new Date().toISOString(),
      totalCommits,
      totalContributors: contributors.length,
    };

    mkdirSync(dirname(OUTPUT_FILE), {recursive: true});
    writeFileSync(OUTPUT_FILE, `${JSON.stringify(data, null, 2)}\n`, 'utf8');
    logger.info(`Contributors data generated at ${OUTPUT_FILE} (${contributors.length} contributors, ${totalCommits.toLocaleString()} total commits)`);
  } catch (error) {
    if (existsSync(OUTPUT_FILE)) {
      logger.error('❌ Failed to generate contributors — keeping existing file:', error);

      return;
    }

    logger.error('❌ Failed to generate contributors — writing fallback:', error);

    const fallback = {
      contributors: [],
      generatedAt: new Date().toISOString(),
      totalCommits: 0,
      totalContributors: 0,
    };

    mkdirSync(dirname(OUTPUT_FILE), {recursive: true});
    writeFileSync(OUTPUT_FILE, `${JSON.stringify(fallback, null, 2)}\n`, 'utf8');
  }
}

generate();
