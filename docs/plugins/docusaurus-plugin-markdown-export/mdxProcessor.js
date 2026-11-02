// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const productConfig = require('../../docusaurus.product.config');

const PRODUCT_NAME = productConfig.project.name;
const CONSOLE_URL = productConfig.local.consoleUrl;
const WAYFINDER_SAMPLE_URL = productConfig.local.samples.wayfinderUrl;
const WAYFINDER_MAIL_URL = productConfig.local.samples.wayfinderMailUrl;

/**
 * Clean an MDX source file into plain Markdown suitable for LLM consumption.
 * Removes imports, replaces known JSX components, and strips leftover JSX.
 */
async function processMarkdownFile(content, _constants, _sourceDir, linkContext) {
  let result = content;

  result = removeImports(result);
  const { frontmatter, body } = extractFrontmatter(result);
  result = body;
  result = replacePlaceholders(result);
  // Admonitions run as a whole-document pass, before anything else: a
  // `:::type ... :::` block's body often contains inline code spans, and
  // splitting on those first would cut the block in half so the regex never
  // sees a complete ":::...:::" span to convert.
  result = processAdmonitions(result);
  // convertJsxToMarkdown runs as a single pass over the whole document (with
  // fenced/inline code protected internally, see its own code-span
  // handling) rather than over pre-split segments: a wrapper element like
  // `<Stepper>` can contain dozens of inline code spans between its open and
  // close tags, so splitting the document on every code span first would
  // scatter one element's open/close tags across many independent chunks,
  // each losing the others' tag-stack state.
  result = convertJsxToMarkdown(result);
  result = transformOutsideCodeSpans(result, stripStrayExpressions);
  result = rewriteRelativeDocLinks(result, linkContext);
  result = cleanupWhitespace(result);

  if (frontmatter.title) {
    // Docusaurus resolves `{{ProductName}}`-style placeholders in `title` during its
    // own build; this export path bypasses that, so it must resolve them the same way
    // it already does for the body, or the exported heading is left with raw `{{...}}`.
    const resolvedTitle = replacePlaceholders(frontmatter.title);
    // Frontmatter `title` is plain text, not live JSX; if it looks like a
    // component name (e.g. `<ThunderIDProvider />`), render it as inline
    // code rather than emitting a raw, unescaped tag into the heading.
    const heading = /[<>]/.test(resolvedTitle) ? `\`${resolvedTitle}\`` : resolvedTitle;
    if (!result.trimStart().startsWith(`# ${heading}`)) {
      result = `# ${heading}\n\n${result}`;
    }
  }

  return result;
}

function extractFrontmatter(content) {
  const match = content.match(/^---\n([\s\S]*?)\n---\n?/);
  if (!match) return { frontmatter: {}, body: content };

  const frontmatter = {};
  const titleMatch = match[1].match(/^title:\s*(.+)/m);
  if (titleMatch) frontmatter.title = titleMatch[1].trim().replace(/^['"]|['"]$/g, '');

  return { frontmatter, body: content.slice(match[0].length) };
}

function removeImports(content) {
  return content.replace(/^import\s+[\s\S]*?from\s+['"][^'"]+['"];?\s*$/gm, '');
}

/** Replace `{{...}}` frontmatter-style placeholders that may have leaked into the body. */
function replacePlaceholders(content) {
  let result = content;
  result = result.replace(/\{\{ProductName\}\}/g, PRODUCT_NAME);
  result = result.replace(/\{\{productSlug\}\}/g, PRODUCT_NAME.toLowerCase());
  result = result.replace(/\{\{ConsoleUrl\}\}/g, CONSOLE_URL);
  result = result.replace(/\{\{WayFinderSampleUrl\}\}/g, WAYFINDER_SAMPLE_URL);
  result = result.replace(/\{\{WayFinderMailUrl\}\}/g, WAYFINDER_MAIL_URL);
  return result;
}

// Docusaurus admonition types have no "important"/"caution" equivalent split
// the way GitHub does; map each onto the closest GitHub alert type.
const ADMONITION_TO_GITHUB_ALERT = {
  note: 'NOTE',
  info: 'NOTE',
  tip: 'TIP',
  important: 'IMPORTANT',
  warning: 'WARNING',
  danger: 'CAUTION',
  caution: 'CAUTION',
};

/**
 * Convert Docusaurus admonitions (:::note, :::tip[, title], etc.) to GitHub
 * alert blockquotes (`> [!NOTE]`), which render natively on GitHub and most
 * Markdown viewers without needing custom CSS/JS.
 *
 * Docusaurus supports both `:::type` and `:::type Custom Title` (this repo
 * uses the latter); GitHub alerts have no title slot, so a custom title is
 * kept as a bold first line inside the blockquote.
 */
function processAdmonitions(content) {
  return content.replace(
    /^([ \t]*):::(\w+)[ \t]*(.*)\n([\s\S]*?)\n\1:::[ \t]*$/gm,
    (match, indent, type, rawTitle, body) => {
      const alertType = ADMONITION_TO_GITHUB_ALERT[type.toLowerCase()] || 'NOTE';
      const title = rawTitle.trim();
      const bodyLines = body
        .split('\n')
        .map((l) => (l.startsWith(indent) ? l.slice(indent.length) : l.replace(/^[ \t]+/, '')));
      while (bodyLines.length && !bodyLines[0].trim()) bodyLines.shift();
      while (bodyLines.length && !bodyLines[bodyLines.length - 1].trim()) bodyLines.pop();
      const lines = title ? [`**${title}**`, '', ...bodyLines] : bodyLines;
      const quoted = lines.map((l) => (l ? `${indent}> ${l}` : `${indent}>`)).join('\n');
      return `${indent}> [!${alertType}]\n${quoted}`;
    },
  );
}

/**
 * Run `transform` only over the parts of `content` that are not fenced code
 * blocks or inline code spans (backtick or tilde fences of any length, and
 * backtick spans of any width — see findCodeSpan), leaving those verbatim so
 * sample code (which often contains JSX that looks like our own components,
 * e.g. `<ThunderIDProvider>`) is never rewritten.
 */
function transformOutsideCodeSpans(content, transform) {
  let result = '';
  let plainStart = 0;
  let i = 0;
  while (i < content.length) {
    if (content[i] === '`' || content[i] === '~') {
      const end = findCodeSpan(content, i);
      if (end !== -1) {
        result += transform(content.slice(plainStart, i)) + content.slice(i, end);
        i = end;
        plainStart = i;
        continue;
      }
    }
    i++;
  }
  result += transform(content.slice(plainStart));
  return result;
}

/**
 * Find the tag starting at `pos` (content[pos] === '<'), respecting quoted
 * attribute values and brace-delimited JSX expressions (which may themselves
 * contain nested tags, e.g. `icon={<Boxes />}`), so the true end of the tag
 * is found instead of stopping at the first stray '>' inside an attribute.
 */
function parseTag(content, pos) {
  let i = pos + 1;
  let isClosing = false;
  if (content[i] === '/') {
    isClosing = true;
    i++;
  }

  const nameStart = i;
  while (i < content.length && /[A-Za-z0-9]/.test(content[i])) i++;
  if (i === nameStart) return null;
  const tagName = content.slice(nameStart, i);

  let braceDepth = 0;
  let quoteChar = null;
  const attrsStart = i;

  while (i < content.length) {
    const ch = content[i];
    if (quoteChar) {
      if (ch === quoteChar) quoteChar = null;
      i++;
      continue;
    }
    if (ch === '"' || ch === "'") {
      quoteChar = ch;
      i++;
      continue;
    }
    if (ch === '{') {
      braceDepth++;
      i++;
      continue;
    }
    if (ch === '}') {
      braceDepth = Math.max(0, braceDepth - 1);
      i++;
      continue;
    }
    if (braceDepth > 0) {
      i++;
      continue;
    }
    if (ch === '>') {
      const selfClosing = content[i - 1] === '/';
      const attrsEnd = selfClosing ? i - 1 : i;
      return {
        tagName,
        isClosing,
        selfClosing,
        attrsRaw: content.slice(attrsStart, attrsEnd),
        end: i + 1,
      };
    }
    i++;
  }

  return null; // unterminated tag; treat '<' as plain text
}

function getAttr(attrsRaw, name) {
  const match = attrsRaw.match(new RegExp(`\\b${name}\\s*=\\s*"([^"]*)"`));
  return match ? match[1] : undefined;
}

function dedent(text) {
  const trimmed = text.replace(/^\n+/, '').replace(/\s+$/, '');
  const lines = trimmed.split('\n');
  const indents = lines.filter((l) => l.trim()).map((l) => l.match(/^[ \t]*/)[0].length);
  const min = indents.length ? Math.min(...indents) : 0;
  return lines.map((l) => l.slice(min)).join('\n').trim();
}

// Block-level wrappers: always sit on their own line, so unwrapping trims
// their inner whitespace and re-adds exactly one blank line around the
// content, instead of compounding with whatever blank lines already
// surrounded the wrapper tag in the source.
const BLOCK_UNWRAP_TAGS = new Set(['TutorialHero', 'Stepper', 'CodeGroup', 'Tabs', 'NextSteps', 'section', 'Box']);

// Inline wrappers: appear mid-sentence, so unwrapping must not inject
// newlines around the content.
const INLINE_UNWRAP_TAGS = new Set(['span', 'Fragment']);

const REMOVE_TAGS = new Set(['SampleDownload', 'UseCaseBranchCards']);

/** Render a closed (or self-closing) element into its Markdown equivalent. */
function renderElement(tagName, attrsRaw, inner) {
  if (/^B2C/.test(tagName)) return '';
  if (REMOVE_TAGS.has(tagName)) return '';
  if (BLOCK_UNWRAP_TAGS.has(tagName)) return `\n${inner.trim()}\n`;
  if (INLINE_UNWRAP_TAGS.has(tagName)) return inner;

  switch (tagName) {
    case 'ProductName':
      return PRODUCT_NAME;
    case 'ConsoleUrl':
      return CONSOLE_URL + (getAttr(attrsRaw, 'path') || '');
    case 'WayFinderSampleUrl':
      return WAYFINDER_SAMPLE_URL + (getAttr(attrsRaw, 'path') || '');
    case 'WayFinderMailUrl':
      return WAYFINDER_MAIL_URL + (getAttr(attrsRaw, 'path') || '');
    case 'RunThunderID':
      return '```bash\nnpx thunderid\n```\n';
    case 'TutorialHeroItem':
      return `\n- ${inner.trim()}`;
    case 'TabItem': {
      const label = getAttr(attrsRaw, 'label');
      return label ? `\n**${label}**\n\n${inner}` : inner;
    }
    case 'CodeBlock': {
      const lang = getAttr(attrsRaw, 'lang') || 'text';
      const label = getAttr(attrsRaw, 'label');
      const code = dedent(inner);
      return `${label ? `**${label}**\n\n` : ''}\`\`\`${lang}\n${code}\n\`\`\`\n\n`;
    }
    case 'code':
      return `\`${inner.trim()}\``;
    case 'a': {
      const href = getAttr(attrsRaw, 'href') || '';
      const text = inner.replace(/\s+/g, ' ').trim();
      return `\n[${text}](${href})\n`;
    }
    case 'div':
      return `\n${inner.trim()}\n`;
    case 'NextStepsCard': {
      const title = getAttr(attrsRaw, 'title');
      const href = getAttr(attrsRaw, 'href');
      const description = getAttr(attrsRaw, 'description');
      if (!title || !href) return '';
      return description ? `- [${title}](${href}), ${description}\n` : `- [${title}](${href})\n`;
    }
    case 'WayfinderCast':
      return '_[Cast diagram: Wayfinder sample users]_';
    case 'WayfinderArchitecture':
      return '_[Architecture diagram: Wayfinder components]_';
    case 'WayfinderOrganization':
      return '_[Organization diagram: Wayfinder team structure]_';
    default:
      // Unknown component: best-effort fallback, keep the inner text.
      return inner;
  }
}

/**
 * Walk the content, tracking open JSX/HTML elements on a stack so nested
 * tags (including same-named siblings) close against their own matching
 * pair rather than the nearest closing tag of any element.
 */
// Per CommonMark, a code fence must start at the beginning of a line (with
// up to 3 leading spaces); a "```" appearing mid-sentence (e.g. prose giving
// ```toml``` as an example) is not a fence and must not be treated as one.
function isAtLineStart(content, i) {
  let j = i - 1;
  while (j >= 0 && (content[j] === ' ' || content[j] === '\t')) j--;
  return j < 0 || content[j] === '\n';
}

function runLength(content, i, ch) {
  let j = i;
  while (content[j] === ch) j++;
  return j - i;
}

/**
 * If `content[i]` begins a fenced code block or inline code span, returns its
 * exclusive end index; otherwise returns -1. Recognizes backtick *and* tilde
 * fences of any length >= 3 (closed only by a same-character run of at least
 * that length, per CommonMark) — not just a hardcoded "```" — and inline spans
 * delimited by one or more backticks, closed only by a run of exactly that
 * length, so e.g. `` `code` `` (a literal backtick inside) round-trips intact
 * instead of being cut short at the first single backtick.
 */
function findCodeSpan(content, i) {
  const ch = content[i];
  if (ch !== '`' && ch !== '~') return -1;
  const len = runLength(content, i, ch);

  if (isAtLineStart(content, i) && len >= 3) {
    // Fence: scan line by line (starting after the opening line, which may carry
    // an info string like "```yaml") for a line holding only a same-character
    // run of at least `len`.
    let pos = content.indexOf('\n', i + len);
    if (pos === -1) return content.length; // unterminated fence: rest of doc is code
    pos += 1;
    while (pos <= content.length) {
      const lineEnd = content.indexOf('\n', pos);
      const line = content.slice(pos, lineEnd === -1 ? content.length : lineEnd);
      const m = /^[ \t]{0,3}(`+|~+)[ \t]*$/.exec(line);
      if (m && m[1][0] === ch && m[1].length >= len) {
        return lineEnd === -1 ? content.length : lineEnd + 1;
      }
      if (lineEnd === -1) return content.length;
      pos = lineEnd + 1;
    }
    return content.length;
  }
  if (ch === '~') return -1; // tildes only ever open a fence, never inline code

  // Inline code span: closed only by a run of exactly `len` backticks, and only
  // within the rest of this line (this codebase's inline spans never wrap).
  const lineEnd = content.indexOf('\n', i);
  const searchEnd = lineEnd === -1 ? content.length : lineEnd;
  let j = i + len;
  while (j < searchEnd) {
    if (content[j] === '`') {
      const closeLen = runLength(content, j, '`');
      if (closeLen === len) return j + len;
      j += closeLen;
    } else {
      j++;
    }
  }
  return -1;
}

function convertJsxToMarkdown(content) {
  const root = { tagName: null, attrsRaw: '', buffer: '' };
  const stack = [root];
  let i = 0;

  while (i < content.length) {
    // Fenced code blocks and inline code spans are copied verbatim (no tag
    // parsing inside), but unlike a pre-split, this keeps the tag stack
    // intact across them, so a wrapper whose open/close tags straddle one
    // or more code spans still closes correctly.
    if (content[i] === '`' || content[i] === '~') {
      const end = findCodeSpan(content, i);
      if (end !== -1) {
        stack[stack.length - 1].buffer += content.slice(i, end);
        i = end;
        continue;
      }
    }
    if (content[i] === '<') {
      const tag = parseTag(content, i);
      if (tag) {
        if (tag.isClosing) {
          let frameIndex = -1;
          for (let d = stack.length - 1; d >= 1; d--) {
            if (stack[d].tagName === tag.tagName) {
              frameIndex = d;
              break;
            }
          }
          if (frameIndex !== -1) {
            // A mismatched close (e.g. `<Stepper><CodeGroup>text</Stepper>`) matches a
            // frame below the stack top: pop and render every intervening frame into its
            // parent first, so their buffered content isn't discarded along with them.
            while (stack.length > frameIndex) {
              const frame = stack.pop();
              const rendered = renderElement(frame.tagName, frame.attrsRaw, frame.buffer);
              stack[stack.length - 1].buffer += rendered;
            }
          }
          i = tag.end;
          continue;
        }
        if (tag.selfClosing) {
          stack[stack.length - 1].buffer += renderElement(tag.tagName, tag.attrsRaw, '');
          i = tag.end;
          continue;
        }
        stack.push({ tagName: tag.tagName, attrsRaw: tag.attrsRaw, buffer: '' });
        i = tag.end;
        continue;
      }
    }
    stack[stack.length - 1].buffer += content[i];
    i++;
  }

  // Unwind any unclosed frames (malformed/truncated JSX) by unwrapping them.
  while (stack.length > 1) {
    const frame = stack.pop();
    const rendered = renderElement(frame.tagName, frame.attrsRaw, frame.buffer);
    stack[stack.length - 1].buffer += rendered;
  }

  return root.buffer;
}

/** Remove stray `{expr}` JS expressions that weren't part of a recognized tag. */
function stripStrayExpressions(content) {
  return content.replace(/\{\/\*[\s\S]*?\*\/\}/g, '').replace(/\{[^}\n]{0,200}\}/g, '');
}

/** Rewrite relative doc links to absolute .md URLs. */
function rewriteRelativeDocLinks(content, linkContext) {
  if (!linkContext?.docUrlPath || !linkContext?.siteUrl) return content;

  // Split on code blocks to avoid rewriting links inside them
  const parts = content.split(/(```[\s\S]*?```)/g);
  for (let i = 0; i < parts.length; i++) {
    if (i % 2 === 1) continue; // skip code blocks
    parts[i] = parts[i].replace(
      /(`[^`\n]*`)|(!?)\[([^\]]+)\]\(([^)\s]+)(\s+"[^"]*")?\)/g,
      (match, codeSpan, bang, text, url, title) => {
        if (codeSpan !== undefined) return codeSpan;
        if (bang === '!') return match; // keep images
        const newUrl = rewriteDocLinkUrl(url, linkContext);
        return `[${text}](${newUrl}${title || ''})`;
      },
    );
  }
  return parts.join('');
}

function rewriteDocLinkUrl(url, { docUrlPath, siteUrl }) {
  if (/^([a-z][a-z0-9+.-]*:|\/\/)/i.test(url)) return url; // absolute
  if (url.startsWith('#')) return url; // anchor

  const hashIdx = url.indexOf('#');
  const queryIdx = url.indexOf('?');
  let cutIdx = url.length;
  if (hashIdx !== -1) cutIdx = Math.min(cutIdx, hashIdx);
  if (queryIdx !== -1) cutIdx = Math.min(cutIdx, queryIdx);

  let pathPart = url.slice(0, cutIdx);
  const tail = url.slice(cutIdx);

  if (!pathPart) return url;

  if (/\.mdx$/i.test(pathPart)) {
    pathPart = pathPart.replace(/\.mdx$/i, '.md');
  } else if (!/\.md$/i.test(pathPart)) {
    if (/\.[a-z0-9]{1,5}$/i.test(pathPart)) return url; // non-doc file, leave alone
    pathPart = pathPart.replace(/\/$/, '') + '.md';
  }

  let absPath;
  if (pathPart.startsWith('/')) {
    absPath = pathPart;
  } else {
    const baseDir = docUrlPath.replace(/\/[^/]*$/, '');
    absPath = resolvePath(baseDir + '/' + pathPart);
  }

  return siteUrl.replace(/\/$/, '') + absPath + tail;
}

function resolvePath(p) {
  const segments = p.split('/');
  const out = [];
  for (const seg of segments) {
    if (seg === '' || seg === '.') {
      if (out.length === 0 && seg === '') out.push('');
      continue;
    }
    if (seg === '..') {
      if (out.length > 1) out.pop();
      continue;
    }
    out.push(seg);
  }
  return out.join('/') || '/';
}

function cleanupWhitespace(content) {
  let result = content;
  // Drop incidental JSX-formatting indentation (1-3 spaces, as opposed to a
  // deliberate 4-space Markdown nesting indent) before synthetic bold labels
  // and bullet links, e.g. `  <NextStepsCard .../>` renders as `- [...]`
  // still carrying its source indentation, which would otherwise read as an
  // unintended nested list item.
  result = result.replace(/^[ \t]{1,3}(\*\*[^\n*]+\*\*|- \[.*)$/gm, '$1');
  result = result.replace(/\n{3,}/g, '\n\n');
  result = result.replace(/[ \t]+$/gm, '');
  result = result.trim() + '\n';
  return result;
}

module.exports = { processMarkdownFile };
