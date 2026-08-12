// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Children, isValidElement, useEffect, type ReactElement, PropsWithChildren} from 'react';
import applyAttributes from './utils/applyAttributes';

export type HelmetProps = PropsWithChildren;

/**
 * A lightweight, provider-free document head manager inspired by react-helmet.
 *
 * Declaratively manage `<title>`, `<meta>`, `<link>`, `<script>`, `<style>`,
 * `<base>`, and `<noscript>` tags by passing them as JSX children. Tags are
 * appended to `document.head` on mount and removed on unmount, keeping the
 * document head in sync with the React tree.
 *
 * Multiple `<Helmet>` instances can coexist — each manages only the nodes it
 * created. The last mounted instance wins for `document.title`.
 *
 * @example
 * <Helmet>
 *   <title>My Page</title>
 *   <meta name="description" content="Page description" />
 *   <link rel="icon" href="/favicon.ico" />
 * </Helmet>
 */
export default function Helmet({children}: HelmetProps): null {
  useEffect(() => {
    const nodes: Element[] = [];

    Children.forEach(children, (child) => {
      if (!isValidElement(child)) return;

      const {type, props} = child as ReactElement<Record<string, unknown>>;

      if (type === 'title') {
        const text = (props as {children?: unknown}).children;
        if (typeof text === 'string' || typeof text === 'number' || typeof text === 'bigint') {
          document.title = String(text);
        } else if (Array.isArray(text)) {
          document.title = text
            .filter((t) => typeof t === 'string' || typeof t === 'number' || typeof t === 'bigint')
            .join('');
        }
        return;
      }

      const el = document.createElement(type as string);
      el.setAttribute('data-helmet', 'true');
      applyAttributes(el, props);

      if (type === 'style' || type === 'script' || type === 'noscript') {
        const content = (props as {children?: unknown}).children;
        if (typeof content === 'string' || typeof content === 'number' || typeof content === 'bigint') {
          el.textContent = String(content);
        }
      }

      document.head.appendChild(el);
      nodes.push(el);
    });

    return () => {
      nodes.forEach((node) => node.parentNode?.removeChild(node));
    };
  }, [children]);

  return null;
}
