// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import BrowserOnly from '@docusaurus/BrowserOnly';
import {useDocsVersion} from '@docusaurus/plugin-content-docs/client';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import {useCallback, useEffect, useState} from 'react';
import {createPortal} from 'react-dom';
import ApiReference from './ApiReference';
import ApiReferenceActionBar from './ApiReferenceActionBar';
import MobileApiReference from './MobileApiReference';
import PostmanButton from './PostmanButton';

/**
 * Renders the API reference for the currently active Docusaurus doc version, with a
 * ThunderID-owned action bar (Postman collection download today, room for more later) docked
 * above the Scalar viewport.
 *
 * The combined OpenAPI spec is expected to live at:
 *   static/api/<versionPath>/combined.yaml
 *
 * The Postman collection is expected to live at:
 *   static/api/<versionPath>/postman/thunderid-api-postman-collection.json
 *
 * The version path follows the convention:
 *   - Docusaurus "current" version (labeled "Next") → 'next'
 *   - Any other version (e.g. '1.1.0') → the version name as-is
 *
 * This matches both the `path` values in docusaurus.config.ts `versions` config
 * and the directory names under static/api/.
 */

// Rendered inside BrowserOnly so window is always available.
// Detects the viewport and switches between the dedicated mobile UI and the
// full Scalar desktop viewer — initialised eagerly to avoid a layout flash.
function ApiReferenceSwitch({
  specUrl,
  collectionUrl,
  downloadFileName,
  onDesktopLoaded,
}: {
  specUrl: string;
  collectionUrl: string;
  downloadFileName: string;
  onDesktopLoaded: () => void;
}) {
  const [isMobile, setIsMobile] = useState(() => window.matchMedia('(max-width: 996px)').matches);

  useEffect(() => {
    const mq = window.matchMedia('(max-width: 996px)');
    // Re-check after mount: the lazy initialiser can read the wrong value on some
    // mobile browsers before the viewport meta tag has been fully applied.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setIsMobile(mq.matches);
    const handler = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, []);

  if (isMobile) {
    return createPortal(
      <MobileApiReference collectionUrl={collectionUrl} downloadFileName={downloadFileName} specUrl={specUrl} />,
      document.body,
    );
  }

  return <ApiReference onLoaded={onDesktopLoaded} specUrl={specUrl} />;
}

/**
 * Distance from the viewport top to where `ApiReferenceActionBar` should render: the navbar's
 * height, plus the doc-version banner's, when one is rendered (it isn't on every version).
 * Mirrors `ApiReference`'s own `useHeaderOffset`, minus the action bar's own height — the two
 * stack: banner, then this bar, then Scalar's content.
 */
function useHeaderTop(): number {
  const [top, setTop] = useState(0);

  useEffect(() => {
    const navbar = document.querySelector<HTMLElement>('.navbar');
    if (!navbar) return undefined;

    const update = () => {
      const banner = document.querySelector<HTMLElement>('.theme-doc-version-banner');
      setTop(navbar.getBoundingClientRect().bottom + (banner?.getBoundingClientRect().height ?? 0));
    };

    update();
    const ro = new ResizeObserver(update);
    ro.observe(navbar);
    const banner = document.querySelector<HTMLElement>('.theme-doc-version-banner');
    if (banner) ro.observe(banner);
    return () => ro.disconnect();
  }, []);

  return top;
}

export default function ApiVersionReference() {
  const {siteConfig} = useDocusaurusContext();
  const {version} = useDocsVersion();
  const headerTop = useHeaderTop();

  const versionPath = version === 'current' ? 'next' : version;
  const productConfig = siteConfig.customFields?.product as {postman: {collection: {output: string}}};
  const specUrl = `${siteConfig.baseUrl}api/${versionPath}/combined.yaml`;
  const postmanCollectionUrl = `${siteConfig.baseUrl}api/${versionPath}/postman/collections/${productConfig.postman.collection.output}`;

  // On mobile the CSS hides all tag-section-containers and shows only the one
  // whose inner <section id="{tag.id}"> matches the URL hash (:target).
  // When Scalar finishes loading and there is no hash yet (fresh page load),
  // click the first navigation link so the user lands on a populated view
  // instead of an empty content area.
  const handleApiLoaded = useCallback(() => {
    if (typeof window === 'undefined' || window.location.hash) return;
    setTimeout(() => {
      const firstNavLink = document.querySelector<HTMLAnchorElement>('.apis-page aside a[href^="#"]:not([href="#"])');
      firstNavLink?.click();
    }, 50);
  }, []);

  return (
    <>
      {/* ThunderID's own action bar — desktop only (mobile has its own self-contained UI) */}
      <BrowserOnly>
        {() => {
          if (window.matchMedia('(max-width: 996px)').matches) return null;
          return (
            <ApiReferenceActionBar top={headerTop}>
              <PostmanButton collectionUrl={postmanCollectionUrl} downloadFileName={productConfig.postman.collection.output} />
            </ApiReferenceActionBar>
          );
        }}
      </BrowserOnly>

      {/* API reference — switches between mobile and desktop renderers */}
      <BrowserOnly>
        {() => (
          <ApiReferenceSwitch
            collectionUrl={postmanCollectionUrl}
            downloadFileName={productConfig.postman.collection.output}
            onDesktopLoaded={handleApiLoaded}
            specUrl={specUrl}
          />
        )}
      </BrowserOnly>
    </>
  );
}
