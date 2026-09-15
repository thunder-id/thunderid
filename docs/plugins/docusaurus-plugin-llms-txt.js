// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const fs = require('fs');
const path = require('path');

/**
 * Docusaurus plugin that generates llms.txt and per-version llms-*.txt files
 * at build time. Each file lists every documentation page as a markdown link,
 * structured by sidebar category. This lets LLMs discover and fetch the full
 * site content efficiently.
 *
 * Generates:
 *   build/llms.txt          — canonical entry point (mirrors the last version)
 *   build/llms-next.txt     — current/bleeding-edge version
 */
module.exports = function pluginLlmsTxt(context, options = {}) {
  const {siteConfig} = context;
  const sidebarName = options.sidebarName || 'docsSidebar';

  let loadedVersions = null;

  function fileNameForVersion(version) {
    if (version.versionName === 'current') return 'llms-next.txt';
    return `llms-${version.versionName}.txt`;
  }

  function absoluteUrl(permalink) {
    if (!permalink) return '';
    if (/^https?:\/\//.test(permalink)) return permalink;
    const host = (siteConfig.url || '').replace(/\/$/, '');
    return host + permalink;
  }

  // Map a doc permalink to its .md equivalent:
  //   /docs/next/guides/foo  →  /docs/next/guides/foo.md
  //   /docs/next/            →  /docs/next.md
  function markdownUrl(permalink) {
    if (!permalink) return '';
    const trimmed = permalink.replace(/\/+$/, '');
    return absoluteUrl(trimmed + '.md');
  }

  function escapeInline(s) {
    if (!s) return '';
    return String(s).replace(/\s+/g, ' ').trim();
  }

  function formatBullet(doc, overrideLabel) {
    const title = escapeInline(overrideLabel || doc.title || doc.id);
    const url = markdownUrl(doc.permalink);
    return `- [${title}](${url})`;
  }

  function getDocByRef(ref, docsById) {
    return docsById.get(ref) || null;
  }

  // Recursively render sidebar items. `depth` is the heading level for
  // nested categories (starts at 3 for "###").
  function renderCategoryBody(items, depth, lines, docsById) {
    const directBullets = [];
    const subcategories = [];

    for (const item of items) {
      if (typeof item === 'string') {
        const doc = getDocByRef(item, docsById);
        if (doc) directBullets.push(formatBullet(doc));
      } else if (!item || typeof item !== 'object') {
        continue;
      } else if (item.type === 'doc' || item.type === 'ref') {
        const doc = getDocByRef(item.id, docsById);
        if (doc) directBullets.push(formatBullet(doc, item.label));
      } else if (item.type === 'category') {
        subcategories.push(item);
      } else if (item.type === 'link' && item.href) {
        directBullets.push(
          `- [${escapeInline(item.label || item.href)}](${item.href})`,
        );
      }
    }

    for (const b of directBullets) lines.push(b);
    if (directBullets.length) lines.push('');

    const heading = '#'.repeat(Math.min(depth, 6));
    for (const sub of subcategories) {
      lines.push(`${heading} ${escapeInline(sub.label)}`);
      lines.push('');
      if (sub.link?.type === 'doc') {
        const doc = getDocByRef(sub.link.id, docsById);
        if (doc) {
          lines.push(formatBullet(doc, sub.label));
          lines.push('');
        }
      }
      renderCategoryBody(sub.items || [], depth + 1, lines, docsById);
    }
  }

  function buildContent(version) {
    const sidebar = version.sidebars?.[sidebarName];
    const docsById = new Map();
    for (const d of version.docs || []) {
      docsById.set(d.id, d);
      if (d.unversionedId && !docsById.has(d.unversionedId)) {
        docsById.set(d.unversionedId, d);
      }
    }

    const versionLabel = version.label || version.versionName;
    const lines = [];
    lines.push(`# ${siteConfig.title} Documentation (${versionLabel})`);
    lines.push('');
    if (siteConfig.tagline) {
      lines.push(`> ${siteConfig.tagline}`);
      lines.push('');
    }

    if (!sidebar) {
      lines.push(`_No sidebar named "${sidebarName}" found for this version._`);
      return lines.join('\n') + '\n';
    }

    for (const item of sidebar) {
      if (typeof item === 'string') {
        const doc = getDocByRef(item, docsById);
        if (!doc) continue;
        lines.push(`## ${escapeInline(doc.title || doc.id)}`);
        lines.push('');
        lines.push(formatBullet(doc));
        lines.push('');
      } else if (item.type === 'doc' || item.type === 'ref') {
        const doc = getDocByRef(item.id, docsById);
        if (!doc) continue;
        lines.push(`## ${escapeInline(item.label || doc.title || doc.id)}`);
        lines.push('');
        lines.push(formatBullet(doc, item.label));
        lines.push('');
      } else if (item.type === 'category') {
        lines.push(`## ${escapeInline(item.label)}`);
        lines.push('');
        if (item.link?.type === 'doc') {
          const doc = getDocByRef(item.link.id, docsById);
          if (doc) {
            lines.push(formatBullet(doc, item.label));
            lines.push('');
          }
        }
        renderCategoryBody(item.items || [], 3, lines, docsById);
      } else if (item.type === 'link' && item.href) {
        lines.push(`## ${escapeInline(item.label || item.href)}`);
        lines.push('');
        lines.push(`- [${escapeInline(item.label || item.href)}](${item.href})`);
        lines.push('');
      }
    }

    return lines.join('\n').replace(/\n{3,}/g, '\n\n').trimEnd() + '\n';
  }

  return {
    name: 'docusaurus-plugin-llms-txt',

    async allContentLoaded({allContent}) {
      const docsPlugin = allContent?.['docusaurus-plugin-content-docs'];
      const docsContent = docsPlugin?.default;
      if (!docsContent?.loadedVersions) {
        console.warn('[llms-txt] docs plugin content not found; skipping');
        return;
      }
      loadedVersions = docsContent.loadedVersions;
    },

    async postBuild({outDir}) {
      if (!loadedVersions) {
        console.warn('[llms-txt] no loaded versions; skipping');
        return;
      }

      let written = 0;
      let lastVersionContent = null;

      for (const version of loadedVersions) {
        const content = buildContent(version);

        // Version-specific file at the root (e.g. llms-next.txt)
        const fileName = fileNameForVersion(version);
        fs.writeFileSync(path.join(outDir, fileName), content);
        written++;

        // Also write llms.txt inside the version's own docs path so /docs/llms.txt
        // (last version at the root) and /docs/next/llms.txt resolve alongside the docs
        // they describe. Use the version's resolved base path (version.path, e.g. "/docs"
        // or "/docs/next") so it follows lastVersion and custom routeBasePath instead of
        // assuming the URL segment equals the version name.
        const baseUrl = siteConfig.baseUrl || '/';
        const versionBase = version.path.startsWith(baseUrl)
          ? version.path.slice(baseUrl.length)
          : version.path.replace(/^\//, '');
        const versionedLlmsPath = path.join(outDir, versionBase, 'llms.txt');
        fs.mkdirSync(path.dirname(versionedLlmsPath), {recursive: true});
        fs.writeFileSync(versionedLlmsPath, content);
        written++;

        if (version.isLast) lastVersionContent = content;
      }

      // /llms.txt at the site root = canonical entry point (mirrors last/stable version).
      if (lastVersionContent) {
        fs.writeFileSync(path.join(outDir, 'llms.txt'), lastVersionContent);
        written++;
      } else {
        // Fall back to "current" version if nothing is flagged isLast.
        const current = loadedVersions.find((v) => v.versionName === 'current');
        if (current) {
          fs.writeFileSync(
            path.join(outDir, 'llms.txt'),
            buildContent(current),
          );
          written++;
        } else {
          console.warn('[llms-txt] no version flagged isLast; llms.txt not written');
        }
      }

      console.log(`[llms-txt] Wrote ${written} llms*.txt file(s) to build/`);
    },
  };
};
