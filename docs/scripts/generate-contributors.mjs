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

const PROJECT_NAME = DocusaurusProductConfig.project.name;
const GITHUB_REPO = DocusaurusProductConfig.project.source.github.fullName;
const GITHUB_ORG = GITHUB_REPO.split('/')[0];

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

// Discovered from the org itself rather than a hardcoded list, so a newly created repo
// picks up contributors here without this script needing an update. Archived repos and
// forks are skipped: an archived repo is retired, and a fork's contributors are upstream's,
// not this org's own.
async function fetchOrgRepos() {
  const orgReposApiUrl = `https://api.github.com/orgs/${GITHUB_ORG}/repos`;

  logger.info(`Fetching repositories for org ${GITHUB_ORG}...`);

  const repos = [];
  let nextUrl = `${orgReposApiUrl}?type=public&per_page=100`;

  while (nextUrl) {
    const {body, headers} = await fetchJsonWithHeaders(nextUrl);

    if (!Array.isArray(body)) {
      throw new Error(`Unexpected response format from ${nextUrl}`);
    }

    repos.push(...body);

    const linkHeader = headers.get('link') || '';
    const nextLinkMatch = linkHeader.match(/<([^>]+)>\s*;\s*rel="next"/i);

    nextUrl = nextLinkMatch ? nextLinkMatch[1] : null;
  }

  return repos.filter((repo) => !repo.archived && !repo.fork).map((repo) => repo.full_name);
}

async function fetchRepoContributors(fullName) {
  const contributorsApiUrl = `https://api.github.com/repos/${fullName}/contributors`;

  try {
    logger.info(`Fetching contributors from ${contributorsApiUrl}...`);

    const contributors = [];
    let nextUrl = `${contributorsApiUrl}?per_page=100&anon=false`;

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
  } catch (error) {
    // Keep the other repos aggregating even if one is unreachable (rate-limited, private,
    // renamed, or not created yet) — matches generate-sdk-releases.mjs's per-repo handling.
    logger.warn(`Failed to fetch contributors for ${fullName}: ${error.message}`);

    return [];
  }
}

// The same person contributes to more than one repo, so merge by login and sum their
// contributions across every repo before filtering/shaping — a contributor who is only
// active in an SDK repo, not this docs repo, would otherwise never appear at all.
function mergeContributorsByLogin(perRepoLists) {
  const merged = new Map();

  for (const contributors of perRepoLists) {
    for (const contributor of contributors) {
      const existing = merged.get(contributor.login);

      if (existing) {
        existing.contributions += contributor.contributions;
      } else {
        merged.set(contributor.login, {...contributor});
      }
    }
  }

  // GitHub's own /contributors response is pre-sorted by contributions descending; the merge
  // above loses that ordering, and the UI's default "Commits" sort relies on the data already
  // being sorted rather than re-sorting client-side.
  return [...merged.values()].sort((a, b) => b.contributions - a.contributions);
}

function shouldInclude(contributor) {
  if (contributor.type === 'Bot') return false;
  const login = contributor.login.toLowerCase();

  return !IGNORED_LOGINS.has(login) && !login.includes('[bot]');
}

async function generate() {
  try {
    let repos;

    try {
      repos = await fetchOrgRepos();
    } catch (error) {
      // A failed org listing (rate limit, network) shouldn't zero out the whole file — fall
      // back to just this repo rather than letting the outer catch write an empty result.
      logger.warn(`Failed to list ${GITHUB_ORG} repos, falling back to ${GITHUB_REPO}: ${error.message}`);
      repos = [GITHUB_REPO];
    }

    const perRepo = await Promise.all(repos.map(fetchRepoContributors));
    const merged = mergeContributorsByLogin(perRepo);
    const contributors = merged
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
