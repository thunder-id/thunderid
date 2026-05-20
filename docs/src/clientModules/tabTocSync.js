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

let observer = null;
let docsLandingReloadHandlerBound = false;

function isHeadingVisible(id) {
  const el = document.getElementById(id);
  if (!el) return null; // heading not in DOM — don't hide
  // offsetParent is null when the element or any ancestor has display:none
  return el.offsetParent !== null;
}

function syncTocWithTabs() {
  const tabPanels = document.querySelectorAll('[role="tabpanel"]');
  if (tabPanels.length === 0) return;

  document.querySelectorAll('.table-of-contents__link').forEach((link) => {
    const href = link.getAttribute('href');
    if (!href?.startsWith('#')) return;
    const id = href.slice(1);
    const li = link.closest('li');
    if (!li) return;

    const visible = isHeadingVisible(id);
    // visible===null means heading not in DOM (don't touch it)
    if (visible === false) {
      li.style.display = 'none';
    } else {
      li.style.display = '';
    }
  });
}

function bindDocsLandingReloadLinks() {
  if (docsLandingReloadHandlerBound) return;

  const normalizePathname = (path) => {
    if (!path || path === '/') return '/';
    return path.replace(/\/+$/, '');
  };

  const handler = (event) => {
    const target = event.target;
    if (!(target instanceof Element)) return;

    const anchor = target.closest('.sidebar-doc-home > a.menu__link');
    if (!(anchor instanceof HTMLAnchorElement)) return;

    const href = anchor.getAttribute('href');
    if (!href) return;

    const targetUrl = new URL(anchor.href, window.location.origin);
    const currentPath = normalizePathname(window.location.pathname);
    const targetPath = normalizePathname(targetUrl.pathname);

    // Only force a hard reload when re-clicking the same destination page.
    if (currentPath !== targetPath) return;

    event.preventDefault();
    window.location.assign(targetUrl.href);
  };

  // In dev/HMR, modules can re-evaluate and leave stale listeners attached.
  // Keep one global listener instance to prevent duplicate navigation handling.
  const globalKey = '__thunderDocsLandingReloadClickHandler';
  const previous = window[globalKey];
  if (typeof previous === 'function') {
    document.removeEventListener('click', previous);
  }

  document.addEventListener('click', handler);
  window[globalKey] = handler;

  docsLandingReloadHandlerBound = true;
}

function setup() {
  syncTocWithTabs();

  if (observer) observer.disconnect();

  observer = new MutationObserver((mutations) => {
    let shouldSync = false;
    for (const mutation of mutations) {
      // Tab panel toggled hidden/visible
      if (mutation.type === 'attributes') {
        shouldSync = true;
        break;
      }
      // TOC links added to DOM (lazy-rendered when user opens "On this page")
      if (mutation.type === 'childList') {
        for (const node of mutation.addedNodes) {
          if (node.nodeType === 1 && node.querySelector?.('.table-of-contents__link')) {
            shouldSync = true;
            break;
          }
        }
      }
      if (shouldSync) break;
    }
    if (shouldSync) syncTocWithTabs();
  });

  observer.observe(document.body, {
    attributes: true,
    attributeFilter: ['hidden'],
    childList: true,
    subtree: true,
  });
}

// Fires once when the client JS bundle is first loaded (initial hard page load).
export function onClientEntry() {
  bindDocsLandingReloadLinks();
  // Defer until after React has hydrated and set hidden attributes on inactive panels.
  requestAnimationFrame(() => requestAnimationFrame(setup));
}

// Fires after every SPA route change.
export function onRouteDidUpdate() {
  bindDocsLandingReloadLinks();
  requestAnimationFrame(setup);
}
