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

import type {EmbeddedFlowComponent} from '@thunderid/react';

/**
 * Finds a component by id within the mock component tree (including nested
 * `components`).
 */
export function findComponentById(root: EmbeddedFlowComponent, id: string): EmbeddedFlowComponent | null {
  if (root.id === id) {
    return root;
  }
  for (const child of root.components ?? []) {
    const found = findComponentById(child, id);
    if (found) {
      return found;
    }
  }
  return null;
}

/**
 * Resolves the wired action ref for a click that landed on a rich-text link.
 * The preview renders inside an iframe, so the SDK's own anchor handler, which
 * gates on `instanceof Element`, silently bails (the anchor belongs to the
 * iframe's realm, not the host bundle's). We compensate here using the
 * `data-action-ref` the renderer stamps on wired anchors — realm-safe because it
 * only calls DOM methods on the element, never an `instanceof` check.
 */
export function resolveAnchorActionRef(target: EventTarget | null): string | null {
  // Click targets are elements, but fall back to `parentElement` defensively for
  // any node that lacks `closest` (e.g. an SVG glyph inside the link). Both are
  // realm-safe DOM lookups — no `instanceof`.
  const node = target as (Element & {parentElement: HTMLElement | null}) | null;
  const element = typeof node?.closest === 'function' ? node : (node?.parentElement ?? null);
  return element?.closest('a')?.getAttribute('data-action-ref') ?? null;
}

/**
 * Resolves the most specific component for a hover/focus event: the gate's
 * button adapters carry their component id in the DOM, so a block with several
 * actions can be previewed per button instead of as one unit. A hovered
 * rich-text link is resolved the same way the click handler resolves it — via
 * its `data-action-ref` — as a synthetic component carrying just that ref, so a
 * link sharing a container with another wired component (e.g. a button) always
 * previews its own transition rather than whichever option the subtree search
 * happens to find first.
 */
export function resolveHoverTarget(
  component: EmbeddedFlowComponent,
  target: EventTarget | null,
): EmbeddedFlowComponent {
  const buttonId = (target as HTMLElement | null)?.closest?.('button')?.id;
  if (buttonId) {
    return findComponentById(component, buttonId) ?? component;
  }
  const anchorRef = resolveAnchorActionRef(target);
  if (anchorRef) {
    return {id: anchorRef} as EmbeddedFlowComponent;
  }
  return component;
}
