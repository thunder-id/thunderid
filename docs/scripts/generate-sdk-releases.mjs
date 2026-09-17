// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {existsSync, mkdirSync, writeFileSync} from 'fs';
import {join, dirname} from 'path';
import {fileURLToPath} from 'url';
import {createLogger} from '@thunderid/logger';

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const OUTPUT_FILE = join(__dirname, '..', 'static', 'data', 'sdk-releases.json');

const logger = createLogger('generate-sdk-releases');

// javascript-sdks is a monorepo where each package is tagged independently, e.g. `sdk/react/v0.3.3`.
// The other SDK repos each publish a single package, so they're keyed off the repo's own releases.
const SDK_REPOS = [
  {
    id: 'javascript',
    name: 'JavaScript SDKs',
    fullName: 'thunder-id/javascript-sdks',
    strategy: 'monorepo',
    tagPattern: /^sdk\/([a-z0-9-]+)\/v(.+)$/,
    packageNamePrefix: '@thunderid/',
    knownPackages: [
      'better-auth',
      'browser',
      'express',
      'javascript',
      'nextjs',
      'node',
      'nuxt',
      'react',
      'react-router',
      'tanstack-router',
      'vue',
    ],
  },
  {
    id: 'ios',
    name: 'iOS SDK',
    fullName: 'thunder-id/ios-sdks',
    strategy: 'single',
    packageId: 'ios',
    packageName: 'ThunderID',
  },
  {
    id: 'android',
    name: 'Android SDK',
    fullName: 'thunder-id/android-sdks',
    strategy: 'single',
    packageId: 'android',
    packageName: 'dev.thunderid:android',
  },
  {
    id: 'flutter',
    name: 'Flutter SDK',
    fullName: 'thunder-id/flutter-sdks',
    strategy: 'single',
    packageId: 'flutter',
    packageName: 'thunderid_flutter',
  },
];

function getGitHubHeaders() {
  return {
    'User-Agent': 'ThunderID-Docs-SDK-Releases-Generator',
    ...(process.env.GITHUB_TOKEN ? {Authorization: `token ${process.env.GITHUB_TOKEN}`} : {}),
  };
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

async function fetchReleases(fullName) {
  const releasesApiUrl = `https://api.github.com/repos/${fullName}/releases`;

  logger.info(`Fetching releases from ${releasesApiUrl}...`);

  const releases = [];
  let nextUrl = `${releasesApiUrl}?per_page=100`;

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
}

function sanitizeReleaseBody(body = '', fullName, releaseTag) {
  let sanitized = body
    .replace(/<p\s+align="left">\s*<img\s+src="([^"]+)"\s+alt="([^"]*)"\s+width="([^"]+)">\s*<\/p>/g, '![$2]($1)')
    .replace(/<p\s+align="left">/g, '')
    .replace(/<\/p>/g, '')
    .replace(/<img\s+src="([^"]+)"\s+alt="([^"]*)"\s+width="([^"]+)">/g, '![$2]($1)');

  sanitized = sanitized.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (match, alt, src) => {
    if (!src.startsWith('http')) {
      const githubRawUrl = `https://raw.githubusercontent.com/${fullName}/${releaseTag}/${src}`;

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
        return segment.replace(/\{([^}]+)\}/g, '\\{$1\\}');
      }
      return segment;
    })
    .join('`');

  return sanitized;
}

function cleanChangeText(text) {
  return text.replace(/\s+by\s+@[\w-]+\s+in\s+https:\/\/github\.com\/\S+/, '').trim();
}

function isNewContributorLine(text) {
  return /made their first contribution/i.test(text);
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
      const isEmptyOrLink = !cleanedText || cleanedText.startsWith('http');
      const isNewContributor = isNewContributorLine(cleanedText);

      if (cleanedText && !isFullChangelog && !isEmptyOrLink && !isNewContributor) {
        categories[currentCategory].push(cleanedText);
      }
    }
  }

  return categories;
}

function buildEmptyChanges() {
  return {breaking: [], bugs: [], features: [], improvements: []};
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

function buildAssets(release) {
  return (release.assets || []).map((asset) => ({
    contentType: asset.content_type,
    downloadCount: asset.download_count,
    downloadUrl: asset.browser_download_url,
    id: asset.id,
    name: asset.name,
    sizeBytes: asset.size,
    sizeLabel: formatBytes(asset.size),
    updatedAt: asset.updated_at,
  }));
}

function buildRepositoryInfo(config) {
  const url = `https://github.com/${config.fullName}`;

  return {
    fullName: config.fullName,
    url,
    releasesUrl: `${url}/releases`,
  };
}

function buildMissingPackageEntry(config, packageId, packageName) {
  return {
    assets: [],
    body: null,
    changes: buildEmptyChanges(),
    hasRelease: false,
    htmlUrl: null,
    isPrerelease: false,
    latestVersion: null,
    packageId,
    packageName,
    publishedAt: null,
    publishedDateLabel: null,
    repository: buildRepositoryInfo(config),
    sdkId: config.id,
    sdkName: config.name,
    tagName: null,
  };
}

function buildPackageEntryFromRelease(config, release, packageId, packageName, version) {
  const sanitizedBody = sanitizeReleaseBody(release.body, config.fullName, release.tag_name);

  return {
    assets: buildAssets(release),
    body: sanitizedBody,
    changes: extractChanges(sanitizedBody),
    hasRelease: true,
    htmlUrl: release.html_url,
    isPrerelease: release.prerelease,
    latestVersion: version,
    packageId,
    packageName,
    publishedAt: release.published_at,
    publishedDateLabel: new Date(release.published_at).toLocaleDateString('en-US', {
      day: 'numeric',
      month: 'long',
      year: 'numeric',
    }),
    repository: buildRepositoryInfo(config),
    sdkId: config.id,
    sdkName: config.name,
    tagName: release.tag_name,
  };
}

function buildMonorepoPackages(config, releases) {
  const latestByPackage = new Map();

  for (const release of releases) {
    if (release.draft) continue;

    const match = release.tag_name.match(config.tagPattern);

    if (!match) continue;

    const [, packageId, version] = match;
    const existing = latestByPackage.get(packageId);

    if (!existing || new Date(release.published_at) > new Date(existing.release.published_at)) {
      latestByPackage.set(packageId, {release, version});
    }
  }

  return config.knownPackages.map((packageId) => {
    const packageName = `${config.packageNamePrefix}${packageId}`;
    const entry = latestByPackage.get(packageId);

    if (!entry) {
      return buildMissingPackageEntry(config, packageId, packageName);
    }

    return buildPackageEntryFromRelease(config, entry.release, packageId, packageName, entry.version);
  });
}

function buildSinglePackage(config, releases) {
  const publishedReleases = releases.filter((release) => !release.draft);
  const latest = publishedReleases.find((release) => !release.prerelease) ?? publishedReleases[0];

  if (!latest) {
    return [buildMissingPackageEntry(config, config.packageId, config.packageName)];
  }

  const version = latest.tag_name.replace(/^v/i, '');

  return [buildPackageEntryFromRelease(config, latest, config.packageId, config.packageName, version)];
}

function buildFallbackPackages(config) {
  return config.strategy === 'monorepo'
    ? config.knownPackages.map((packageId) =>
        buildMissingPackageEntry(config, packageId, `${config.packageNamePrefix}${packageId}`),
      )
    : [buildMissingPackageEntry(config, config.packageId, config.packageName)];
}

async function buildPackageRows(config) {
  try {
    const releases = await fetchReleases(config.fullName);

    return config.strategy === 'monorepo'
      ? buildMonorepoPackages(config, releases)
      : buildSinglePackage(config, releases);
  } catch (error) {
    // Keep the other SDK repos generating even if one is unreachable (e.g. rate-limited or not public yet).
    logger.warn(`Failed to fetch releases for ${config.fullName}: ${error.message}`);

    return buildFallbackPackages(config);
  }
}

function buildFallbackSdkReleasesData() {
  return {
    generatedAt: new Date().toISOString(),
    releases: SDK_REPOS.flatMap((config) => buildFallbackPackages(config)),
  };
}

function writeSdkReleasesData(data) {
  mkdirSync(dirname(OUTPUT_FILE), {recursive: true});
  writeFileSync(OUTPUT_FILE, `${JSON.stringify(data, null, 2)}\n`, 'utf8');
}

async function generate() {
  try {
    const releases = (await Promise.all(SDK_REPOS.map(buildPackageRows))).flat();

    writeSdkReleasesData({generatedAt: new Date().toISOString(), releases});
    logger.info(`SDK release data generated at ${OUTPUT_FILE}`);
  } catch (error) {
    if (existsSync(OUTPUT_FILE)) {
      logger.error('❌ Failed to generate SDK release data — keeping existing file:', error);

      return;
    }

    logger.error('❌ Failed to generate SDK release data — writing fallback data:', error);
    writeSdkReleasesData(buildFallbackSdkReleasesData());
    logger.info(`Fallback SDK release data generated at ${OUTPUT_FILE}`);
  }
}

generate();
