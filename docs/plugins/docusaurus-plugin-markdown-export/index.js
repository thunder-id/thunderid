// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const fs = require('fs');
const path = require('path');
const {processMarkdownFile} = require('./mdxProcessor');

/**
 * Generates a clean .md file for every doc page of every docs version at
 * build time, mirroring each doc's real Docusaurus permalink:
 *
 *   content/getting-started/foo.mdx (current, permalink /docs/next/getting-started/foo)
 *     → build/docs/next/getting-started/foo.md
 *     → served at /docs/next/getting-started/foo.md
 *
 *   versioned_docs/version-v1.0.x/getting-started/foo.mdx (permalink /docs/v1.0.x/getting-started/foo)
 *     → build/docs/v1.0.x/getting-started/foo.md
 *     → served at /docs/v1.0.x/getting-started/foo.md
 *
 *   community/overview.mdx (permalink /community/overview)
 *     → build/community/overview.md
 *     → served at /community/overview.md
 *
 * Deriving the output path from `doc.permalink` (rather than re-deriving a
 * slug from the source file path) keeps this in lockstep with
 * docusaurus-plugin-llms-txt, including for index/category-root docs whose
 * permalink has no trailing "/index" segment.
 */
module.exports = function pluginMarkdownExport(context) {
  const {siteDir, siteConfig} = context;
  const siteUrl = (siteConfig?.url || '').replace(/\/$/, '');
  const baseUrl = siteConfig?.baseUrl || '/';

  let loadedVersions = null;

  function siteRelativePath(source) {
    return source.startsWith('@site/') ? source.slice('@site/'.length) : source;
  }

  // `permalink` already includes baseUrl; strip it so the result is
  // relative to `outDir`, which Docusaurus serves mounted at baseUrl.
  function stripBaseUrl(permalink) {
    return permalink.startsWith(baseUrl) ? permalink.slice(baseUrl.length - 1) : permalink;
  }

  async function exportVersion(outDir, version) {
    let written = 0;

    for (const doc of version.docs || []) {
      const fullPath = path.join(siteDir, siteRelativePath(doc.source));
      // Trim trailing slashes (e.g. the index doc's "/docs/next/") to match
      // how docusaurus-plugin-llms-txt derives its .md links.
      const relPermalink = stripBaseUrl(doc.permalink.replace(/\/+$/, ''));
      const outputPath = path.join(outDir, relPermalink + '.md');
      const docUrlPath = (baseUrl + relPermalink).replace(/\/+/g, '/');

      try {
        const source = fs.readFileSync(fullPath, 'utf-8');
        const cleaned = await processMarkdownFile(source, {}, path.dirname(fullPath), {
          docUrlPath,
          siteUrl,
        });

        fs.mkdirSync(path.dirname(outputPath), {recursive: true});
        fs.writeFileSync(outputPath, cleaned);
        written++;
      } catch (err) {
        console.error(`[markdown-export] Error processing ${fullPath}: ${err.message}`);
      }
    }

    return written;
  }

  return {
    name: 'docusaurus-plugin-markdown-export',

    async allContentLoaded({allContent}) {
      const docsPlugin = allContent?.['docusaurus-plugin-content-docs'];
      // Collect every docs plugin instance: the versioned "default" one and the
      // unversioned "community" one, which has a single "current" version.
      const versions = Object.values(docsPlugin ?? {}).flatMap((instance) => instance?.loadedVersions ?? []);
      if (versions.length === 0) {
        console.warn('[markdown-export] docs plugin content not found; skipping');
        return;
      }
      loadedVersions = versions;
    },

    async postBuild({outDir}) {
      if (!loadedVersions) {
        console.warn('[markdown-export] no loaded versions; skipping');
        return;
      }

      let written = 0;
      for (const version of loadedVersions) {
        written += await exportVersion(outDir, version);
      }

      console.log(
        `[markdown-export] Wrote ${written} .md files across ${loadedVersions.length} version(s)`,
      );
    },
  };
};
