// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {existsSync, mkdirSync, readFileSync, writeFileSync} from 'fs';
import {join, dirname} from 'path';
import {fileURLToPath} from 'url';
import {createLogger} from '@thunderid/logger';
import DocusaurusProductConfig from '../docusaurus.product.config.ts';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const OUTPUT_FILE = join(__dirname, '..', 'static', 'data', 'releases.json');
const CONTRIBUTORS_DATA_FILE = join(__dirname, '..', 'static', 'data', 'contributors.json');

const GITHUB_REPO = DocusaurusProductConfig.project.source.github.fullName;
const GITHUB_REPO_URL = DocusaurusProductConfig.project.source.github.url;
const PROJECT_NAME = DocusaurusProductConfig.project.name;
const GITHUB_REPO_API_URL = `https://api.github.com/repos/${GITHUB_REPO}`;
const GITHUB_RELEASES_API_URL = `${GITHUB_REPO_API_URL}/releases`;

const logger = createLogger('generate-changelog');

const userProfileCache = new Map();
const userProfileFetchFailures = new Set();

const ignoredContributorUsernames = [
  '123',
  'asgardeo',
  'copilot',
  'dependabot',
  'example',
  'renovate',
  'thunder',
  'wso2',
  '7',
].map((u) => u.toLowerCase());

function getGitHubHeaders() {
  return {
    'User-Agent': `${PROJECT_NAME}-Docs-Changelog-Generator`,
    ...(process.env.GITHUB_TOKEN ? {Authorization: `token ${process.env.GITHUB_TOKEN}`} : {}),
  };
}

async function fetchJson(url) {
  const {body} = await fetchJsonWithHeaders(url);

  return body;
}

async function fetchJsonWithHeaders(url) {
  const response = await fetch(url, {
    headers: getGitHubHeaders(),
  });

  if (!response.ok) {
    const error = new Error(`Failed to fetch ${url}: ${response.status} ${response.statusText}`);

    error.status = response.status;
    throw error;
  }

  return {
    body: await response.json(),
    headers: response.headers,
  };
}

function getUserProfileCacheKey(username) {
  return username.toLowerCase();
}

function buildFallbackContributorProfile(username) {
  return {
    avatarUrl: null,
    profileUrl: `https://github.com/${username}`,
    username,
  };
}

function setCachedContributorProfile(username, profile) {
  if (!username) {
    return;
  }

  userProfileCache.set(getUserProfileCacheKey(username), {
    avatarUrl: profile.avatarUrl ?? null,
    profileUrl: profile.profileUrl || `https://github.com/${username}`,
    username: profile.username || username,
  });
}

function getCachedContributorProfile(username) {
  const profile = userProfileCache.get(getUserProfileCacheKey(username));

  if (!profile) {
    return null;
  }

  return {
    ...profile,
    profileUrl: profile.profileUrl || `https://github.com/${username}`,
    username,
  };
}

function readJsonFile(filePath) {
  if (!existsSync(filePath)) {
    return null;
  }

  try {
    return JSON.parse(readFileSync(filePath, 'utf8'));
  } catch (error) {
    logger.warn(`Could not read cached contributor profiles from ${filePath}: ${error.message}`);

    return null;
  }
}

function seedContributorProfileCache() {
  const contributorsData = readJsonFile(CONTRIBUTORS_DATA_FILE);

  for (const contributor of contributorsData?.contributors || []) {
    setCachedContributorProfile(contributor.login, {
      avatarUrl: contributor.avatarUrl,
      profileUrl: contributor.htmlUrl,
      username: contributor.login,
    });
  }

  const releasesData = readJsonFile(OUTPUT_FILE);
  const releaseEntries = [releasesData?.latestRelease, ...(releasesData?.releases || [])].filter(Boolean);

  for (const release of releaseEntries) {
    for (const contributor of [...(release.contributors || []), ...(release.newContributors || [])]) {
      if (contributor.avatarUrl) {
        setCachedContributorProfile(contributor.username, contributor);
      }
    }
  }
}

async function fetchRepository() {
  logger.info(`Fetching repository metadata from ${GITHUB_REPO_API_URL}...`);

  try {
    return await fetchJson(GITHUB_REPO_API_URL);
  } catch (error) {
    logger.error('Error fetching repository metadata:', error);

    return null;
  }
}

async function fetchReleases() {
  logger.info(`Fetching releases from ${GITHUB_RELEASES_API_URL}...`);

  try {
    const releases = [];
    let nextUrl = `${GITHUB_RELEASES_API_URL}?per_page=100`;

    while (nextUrl) {
      const {body, headers} = await fetchJsonWithHeaders(nextUrl);

      if (!Array.isArray(body)) {
        throw new Error(`Failed to fetch ${nextUrl}: unexpected response format`);
      }

      releases.push(...body);

      const linkHeader = headers.get('link') || '';
      const nextLinkMatch = linkHeader.match(/<([^>]+)>\s*;\s*rel="next"/i);

      nextUrl = nextLinkMatch ? nextLinkMatch[1] : null;
    }

    return releases;
  } catch (error) {
    logger.error('Error fetching releases:', error);
    throw error;
  }
}

function sanitizeReleaseBody(body = '', releaseTag) {
  let sanitized = body
    .replace(/<p\s+align="left">\s*<img\s+src="([^"]+)"\s+alt="([^"]*)"\s+width="([^"]+)">\s*<\/p>/g, '![$2]($1)')
    .replace(/<p\s+align="left">/g, '')
    .replace(/<\/p>/g, '')
    .replace(/<img\s+src="([^"]+)"\s+alt="([^"]*)"\s+width="([^"]+)">/g, '![$2]($1)');

  sanitized = sanitized.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (match, alt, src) => {
    if (!src.startsWith('http')) {
      const githubRawUrl = `https://raw.githubusercontent.com/${GITHUB_REPO}/${releaseTag}/${src}`;

      return `![${alt}](${githubRawUrl})`;
    }

    return match;
  });

  sanitized = sanitized.replace(/<([a-zA-Z_-]+)>/g, (match, word) => {
    const htmlTags = ['div', 'img', 'p', 'span', 'a', 'strong', 'em', 'br', 'hr'];

    if (!htmlTags.includes(word.toLowerCase())) {
      return `&lt;${word}&gt;`;
    }

    return match;
  });

  // Escape curly braces that look like variables (e.g., {userId}) to prevent MDX ReferenceErrors.
  // We only escape these if they are OUTSIDE of backticks (inline code blocks).
  const segments = sanitized.split('`');
  sanitized = segments
    .map((segment, index) => {
      // Even indices are outside backticks, odd indices are inside (assuming balanced backticks)
      if (index % 2 === 0) {
        // Escape { and } using a backslash for MDX compatibility
        return segment.replace(/\{([^}]+)\}/g, '\\{$1\\}');
      }
      return segment;
    })
    .join('`');

  return sanitized;
}

function extractContributorsFromBody(body) {
  const mentions = body.match(/@([a-zA-Z0-9-]+)/g) || [];
  const uniqueContributors = [...new Set(mentions.map((mention) => mention.substring(1)))];

  return uniqueContributors.filter((username) => !ignoredContributorUsernames.includes(username.toLowerCase()));
}

function cleanChangeText(text) {
  return text.replace(/\s+by\s+@[\w-]+\s+in\s+https:\/\/github\.com\/\S+/, '').trim();
}

function isNewContributorLine(text) {
  return /made their first contribution/i.test(text);
}

function extractNewContributorUsernames(body) {
  const newContributorsMatch = body.match(/New Contributors[\s\S]*?(?=###|$)/);

  if (!newContributorsMatch) {
    return [];
  }

  const mentions = (newContributorsMatch[0] || '').match(/@([a-zA-Z0-9-]+)/g) || [];
  const usernames = [...new Set(mentions.map((mention) => mention.substring(1)))];

  return usernames.filter((username) => !ignoredContributorUsernames.includes(username.toLowerCase()));
}

function extractChanges(body) {
  const categories = {
    breaking: [],
    bugs: [],
    features: [],
    improvements: [],
  };

  const lines = body.split('\n');
  let currentCategory = null;

  for (const line of lines) {
    const trimmed = line.trim();

    if (trimmed.includes('Breaking Changes') || trimmed.includes('⚠️')) {
      currentCategory = 'breaking';
    } else if (trimmed.includes('Features') || trimmed.includes('🚀')) {
      currentCategory = 'features';
    } else if (trimmed.includes('Improvements') || trimmed.includes('✨')) {
      currentCategory = 'improvements';
    } else if (trimmed.includes('Bug Fixes') || trimmed.includes('🐛')) {
      currentCategory = 'bugs';
    } else if (trimmed.startsWith('*') && currentCategory) {
      const cleanedText = cleanChangeText(trimmed.substring(1).trim());
      const isFullChangelog = cleanedText.toLowerCase().includes('full changelog');
      const isNoteAboutSampleApp = cleanedText.toLowerCase().includes('note the id of the sample app');
      const isEmptyOrLink = !cleanedText || cleanedText.startsWith('http');
      const isNewContributor = isNewContributorLine(cleanedText);

      if (cleanedText && !isFullChangelog && !isNoteAboutSampleApp && !isEmptyOrLink && !isNewContributor) {
        categories[currentCategory].push(cleanedText);
      }
    }
  }

  return categories;
}

function formatBytes(bytes) {
  if (!bytes) {
    return '0 B';
  }

  const units = ['B', 'KB', 'MB', 'GB'];
  const unitIndex = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const value = bytes / 1024 ** unitIndex;

  return `${value.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function pickPrimaryAsset(assets) {
  if (assets.length === 0) {
    return null;
  }

  return (
    assets.find((asset) => /thunder(?:id)?-.*\.(zip|tgz|tar\.gz)$/i.test(asset.name)) ||
    assets.find((asset) => /\.(zip|tgz|tar\.gz)$/i.test(asset.name)) ||
    assets[0]
  );
}

async function fetchContributorProfile(username) {
  const cachedProfile = getCachedContributorProfile(username);

  if (cachedProfile) {
    return cachedProfile;
  }

  try {
    const data = await fetchJson(`https://api.github.com/users/${username}`);
    const profile = {
      avatarUrl: data.avatar_url,
      profileUrl: data.html_url,
      username: data.login || username,
    };

    setCachedContributorProfile(username, profile);

    return getCachedContributorProfile(username) || profile;
  } catch (error) {
    if (error.status !== 404) {
      userProfileFetchFailures.add(`${username}: ${error.status || error.message}`);
    }

    return buildFallbackContributorProfile(username);
  }
}

async function mapWithConcurrency(items, limit, mapper) {
  const results = new Array(items.length);
  let nextIndex = 0;

  async function worker() {
    while (nextIndex < items.length) {
      const currentIndex = nextIndex;

      nextIndex += 1;
      results[currentIndex] = await mapper(items[currentIndex], currentIndex);
    }
  }

  await Promise.all(Array.from({length: Math.min(limit, items.length)}, worker));

  return results;
}

async function buildContributorProfiles(usernames) {
  return mapWithConcurrency(usernames, 4, fetchContributorProfile);
}

async function buildReleaseEntry(release) {
  const sanitizedBody = sanitizeReleaseBody(release.body, release.tag_name);
  const contributors = await buildContributorProfiles(extractContributorsFromBody(sanitizedBody));
  const newContributors = await buildContributorProfiles(extractNewContributorUsernames(sanitizedBody));
  const changes = extractChanges(sanitizedBody);
  const assets = (release.assets || []).map((asset) => ({
    contentType: asset.content_type,
    downloadCount: asset.download_count,
    downloadUrl: asset.browser_download_url,
    id: asset.id,
    name: asset.name,
    sizeBytes: asset.size,
    sizeLabel: formatBytes(asset.size),
    updatedAt: asset.updated_at,
  }));
  const primaryAsset = pickPrimaryAsset(assets);

  return {
    assets,
    body: sanitizedBody,
    changes,
    contributors,
    htmlUrl: release.html_url,
    id: release.id,
    isDraft: release.draft,
    isLatest: false,
    isPrerelease: release.prerelease,
    name: release.name || release.tag_name,
    newContributors,
    primaryDownloadUrl: primaryAsset?.downloadUrl || release.html_url,
    publishedAt: release.published_at,
    publishedDateLabel: new Date(release.published_at).toLocaleDateString('en-US', {
      day: 'numeric',
      month: 'long',
      year: 'numeric',
    }),
    tagName: release.tag_name,
  };
}

async function buildChangelogData(repository, releases) {
  const publishedReleases = releases.filter((release) => !release.draft);
  const releaseEntries = await Promise.all(publishedReleases.map(buildReleaseEntry));

  if (userProfileFetchFailures.size > 0) {
    logger.warn(
      `Could not fetch ${userProfileFetchFailures.size} contributor avatar profile(s). Using initials fallback for: ${[
        ...userProfileFetchFailures,
      ].join(', ')}`,
    );
  }

  const latestEntry = releaseEntries.find((entry) => !entry.isPrerelease) ?? releaseEntries[0];
  if (latestEntry) {
    latestEntry.isLatest = true;
  }

  return {
    generatedAt: new Date().toISOString(),
    latestRelease: latestEntry || null,
    releases: releaseEntries,
    repository: {
      description: repository?.description || `${PROJECT_NAME} release notes and downloads.`,
      forks: repository?.forks_count || 0,
      fullName: repository?.full_name || GITHUB_REPO,
      stars: repository?.stargazers_count || 0,
      subscribers: repository?.subscribers_count || 0,
      url: repository?.html_url || GITHUB_REPO_URL,
      releasesUrl: `${repository?.html_url || GITHUB_REPO_URL}/releases`,
    },
  };
}

function buildFallbackChangelogData() {
  return {
    generatedAt: new Date().toISOString(),
    latestRelease: null,
    releases: [],
    repository: {
      description: `${PROJECT_NAME} release notes and downloads.`,
      forks: 0,
      fullName: GITHUB_REPO,
      stars: 0,
      subscribers: 0,
      url: GITHUB_REPO_URL,
      releasesUrl: `${GITHUB_REPO_URL}/releases`,
    },
  };
}

function writeChangelogData(changelogData) {
  mkdirSync(dirname(OUTPUT_FILE), {recursive: true});
  writeFileSync(OUTPUT_FILE, `${JSON.stringify(changelogData, null, 2)}\n`, 'utf8');
}

async function generate() {
  seedContributorProfileCache();

  try {
    const [repository, releases] = await Promise.all([fetchRepository(), fetchReleases()]);
    const changelogData = await buildChangelogData(repository, releases);

    writeChangelogData(changelogData);
    logger.info(`Changelog data generated at ${OUTPUT_FILE}`);
  } catch (error) {
    if (existsSync(OUTPUT_FILE)) {
      logger.error('❌ Failed to generate changelog — keeping existing file:', error);

      return;
    }

    logger.error('❌ Failed to generate changelog — writing fallback release data:', error);
    writeChangelogData(buildFallbackChangelogData());
    logger.info(`Fallback changelog data generated at ${OUTPUT_FILE}`);
  }
}

generate();
