/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {Children, isValidElement, useEffect, useMemo, type ReactElement, PropsWithChildren} from 'react';
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
  const childrenKey = useMemo(() => {
    const parts: string[] = [];
    Children.forEach(children, (child) => {
      if (!isValidElement(child)) return;
      const {type, props} = child as ReactElement<Record<string, unknown>>;
      try {
        parts.push(JSON.stringify({props, type: String(type)}));
      } catch {
        parts.push(String(type));
      }
    });
    return parts.join('\0');
  }, [children]);

  // eslint-disable-next-line react-hooks/exhaustive-deps
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
  }, [childrenKey]);

  return null;
}
