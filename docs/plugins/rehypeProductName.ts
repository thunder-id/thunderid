// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Plugin} from 'unified';

interface Options {
  productName: string;
  productSlug: string;
  /**
   * Additional token-to-value substitutions. Each key is the literal token
   * (e.g. `{{ConsoleUrl}}`) replaced with the corresponding value.
   */
  replacements?: Record<string, string>;
  /**
   * Substitutions that differ between the current (Next) docs and the versioned docs, picked by
   * whether the page lives under `versioned_docs/`. Each set is keyed like `replacements`.
   */
  perVersion?: {current: Record<string, string>; versioned: Record<string, string>};
}

/** Reports whether a page is part of a cut docs version rather than the current (Next) docs. */
function isVersionedDoc(filePath: string | undefined): boolean {
  return typeof filePath === 'string' && /[\\/]versioned_docs[\\/]/.test(filePath);
}

interface HastNode {
  type: string;
  tagName?: string;
  value?: string;
  children?: HastNode[];
}

function applyReplacements(value: string, replacements: Record<string, string>): string {
  let result = value;
  for (const [token, replacement] of Object.entries(replacements)) {
    result = result.replaceAll(token, replacement);
  }
  return result;
}

function replaceInTextNode(node: HastNode, replacements: Record<string, string>): void {
  if (node.type === 'text' && typeof node.value === 'string') {
    node.value = applyReplacements(node.value, replacements);
  }
}

/**
 * Recursively visits every `code` element in the hast tree — both fenced code
 * blocks (`pre > code`) and inline code spans — and replaces token occurrences
 * in their text children with the configured values.
 */
function replaceInCodeNodes(node: HastNode, replacements: Record<string, string>): void {
  if (node.type === 'element' && node.tagName === 'code' && node.children) {
    for (const child of node.children) {
      replaceInTextNode(child, replacements);
    }
    // Don't recurse into code children — text nodes already handled above.
    return;
  }

  if (node.children) {
    for (const child of node.children) {
      replaceInCodeNodes(child, replacements);
    }
  }
}

const rehypeProductName: Plugin<[Options]> = function (options: Options) {
  const {productName, productSlug, replacements: extra = {}, perVersion} = options;
  const replacements: Record<string, string> = {
    '{{ProductName}}': productName,
    '{{productSlug}}': productSlug,
    ...extra,
  };

  return (tree: unknown, file: {path?: string}) => {
    const versionReplacements = perVersion
      ? isVersionedDoc(file?.path)
        ? perVersion.versioned
        : perVersion.current
      : {};
    replaceInCodeNodes(tree as HastNode, {...replacements, ...versionReplacements});
  };
};

export default rehypeProductName;
