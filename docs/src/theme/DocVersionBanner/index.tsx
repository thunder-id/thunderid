// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import type {PropVersionMetadata} from '@docusaurus/plugin-content-docs';
import {
  useActivePlugin,
  useDocVersionSuggestions,
  useDocsPreferredVersion,
  useDocsVersion,
  type GlobalVersion,
} from '@docusaurus/plugin-content-docs/client';
import Translate from '@docusaurus/Translate';
import {Box} from '@wso2/oxygen-ui';
import {ArrowRight} from '@wso2/oxygen-ui-icons-react';
import clsx from 'clsx';
import {JSX, useLayoutEffect, useRef} from 'react';

const ACCENT = '#F5B446';

// Docusaurus's own sidebar/mobile-menu breakpoint (see src/css/custom.css) — full-bleed and
// sticky positioning are desktop-only below, so this needs to match exactly.
const MOBILE_BREAKPOINT = 996;

/**
 * `calc(50% - 50vw)` full-bleed only works when the element's containing block is itself
 * centered in the viewport. Here it's nested inside the doc content column, to the right of
 * the sidebar, so that container's own left/right offsets need to be measured and cancelled
 * out directly instead. Recomputed on resize and whenever the column's own size changes (the
 * sidebar collapsing/expanding doesn't fire a window resize).
 *
 * Skipped below the mobile breakpoint: Docusaurus's mobile nav slides in via a `transform` on
 * an ancestor, and a transformed ancestor breaks `position: sticky`'s normal viewport-relative
 * behavior, which otherwise drags this element into the slide-out menu's own stacking context.
 */
function useFullBleed<T extends HTMLElement>() {
  const ref = useRef<T>(null);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return undefined;

    // Deferred to a frame: mutating the element's own margins from inside a ResizeObserver
    // callback that's watching its parent, synchronously, is exactly the pattern that trips
    // the browser's ResizeObserver loop-detection ("ResizeObserver loop completed with
    // undelivered notifications"). Breaking the observe -> mutate -> observe cycle across a
    // frame boundary avoids it.
    let frame = 0;
    const apply = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => {
        if (window.innerWidth <= MOBILE_BREAKPOINT) {
          el.style.marginLeft = '';
          el.style.marginRight = '';
          el.style.marginTop = '';
          return;
        }
        el.style.marginLeft = '0';
        el.style.marginRight = '0';
        el.style.marginTop = '0';
        const rect = el.getBoundingClientRect();
        el.style.marginLeft = `${-rect.left}px`;
        el.style.marginRight = `${rect.right - window.innerWidth}px`;

        // The doc content column has its own top padding before this element (breadcrumb
        // spacing etc.), which otherwise leaves a gap between the sticky navbar and the
        // banner's natural (unscrolled) position. Pull it up flush against the navbar.
        const navbar = document.querySelector('nav.navbar');
        if (navbar) {
          const gap = rect.top - navbar.getBoundingClientRect().bottom;
          if (gap > 0) el.style.marginTop = `${-gap}px`;
        }
      });
    };

    apply();
    window.addEventListener('resize', apply);
    const resizeObserver = new ResizeObserver(apply);
    if (el.parentElement) resizeObserver.observe(el.parentElement);

    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener('resize', apply);
      resizeObserver.disconnect();
    };
  }, []);

  return ref;
}

function BannerMessage({banner}: {banner: PropVersionMetadata['banner']}): JSX.Element {
  return banner === 'unmaintained' ? (
    <Translate id="theme.docs.versions.unmaintainedVersionLabel" description="The label used to tell the user that he's browsing an unmaintained doc version">
      You&apos;re reading docs for a version that&apos;s no longer maintained.
    </Translate>
  ) : (
    <Translate id="theme.docs.versions.unreleasedVersionLabel" description="The label used to tell the user that he's browsing an unreleased doc version">
      You&apos;re reading unreleased docs. Features and APIs may change before release.
    </Translate>
  );
}

export default function DocVersionBanner({className = undefined}: {className?: string}): JSX.Element | null {
  const versionMetadata = useDocsVersion();
  const {pluginId} = useActivePlugin({failfast: true})!;
  const {savePreferredVersionName} = useDocsPreferredVersion(pluginId);
  const {latestDocSuggestion, latestVersionSuggestion} = useDocVersionSuggestions(pluginId);
  const fullBleedRef = useFullBleed<HTMLDivElement>();

  if (!versionMetadata.banner) {
    return null;
  }

  const getVersionMainDoc = (version: GlobalVersion) => version.docs.find((doc) => doc.id === version.mainDocId)!;
  const latestVersionSuggestedDoc = latestDocSuggestion ?? getVersionMainDoc(latestVersionSuggestion);

  return (
    <Box
      ref={fullBleedRef}
      role="note"
      className={clsx('theme-doc-version-banner', className)}
      sx={{
        // Sticky on desktop, so the banner stays visible while scrolling a normal doc page
        // instead of disappearing the moment you scroll past it — the API reference page's
        // `useHeaderOffset`/`ApiReferenceActionBar` already reserve this banner's height in
        // Scalar's own layout, so there's no longer anything for a sticky banner to collide
        // with there either (that page's own content is a fixed-position overlay with its own
        // internal scroll, so the outer window never actually scrolls there regardless).
        // `relative`, not sticky, below the mobile breakpoint: Docusaurus's mobile nav slides
        // in via a `transform` on an ancestor, and a transformed ancestor breaks `position:
        // sticky`'s normal viewport-relative behavior, dragging this into the slide-out menu's
        // own stacking context.
        //
        // `zIndex` is desktop-only too, for a related reason: the mobile docs sidebar overlay
        // doesn't actually remove this banner from the DOM, just transforms it out of view —
        // a `zIndex` here would still create a stacking context and win over that overlay
        // (which sits at a lower z-index), painting straight through it.
        //
        // 50, not the navbar's own 200: the navbar's *dropdown menus* (e.g. the version
        // switcher) render at z-index 100 — lower than the navbar bar itself, but this only
        // needs to clear ordinary scrolled page content, not out-rank the navbar's own popovers.
        position: 'relative',
        [`@media (min-width: ${MOBILE_BREAKPOINT + 1}px)`]: {
          position: 'sticky',
          top: 'var(--ifm-navbar-height)',
          zIndex: 50,
        },
        borderBottom: '1px solid color-mix(in srgb, #F5B446 30%, var(--oxygen-palette-background-default))',
        background: 'color-mix(in srgb, #F5B446 10%, var(--oxygen-palette-background-default))',
        '[data-theme="light"] &': {
          borderBottom: '1px solid color-mix(in srgb, #F5B446 45%, var(--oxygen-palette-background-default))',
          background: 'color-mix(in srgb, #F5B446 16%, var(--oxygen-palette-background-default))',
        },
      }}
    >
      <Box
        sx={{
          maxWidth: '1400px',
          mx: 'auto',
          px: {xs: 2, md: 4},
          py: '8px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexWrap: 'wrap',
          gap: '8px 14px',
        }}
      >
        <Box
          component="span"
          sx={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: '6px',
            px: '8px',
            py: '2px',
            borderRadius: '6px',
            bgcolor: 'rgba(245,180,70,0.14)',
            color: ACCENT,
            fontFamily: "'JetBrains Mono', monospace",
            fontSize: '11px',
            fontWeight: 500,
            letterSpacing: '0.02em',
            textTransform: 'lowercase',
            flexShrink: 0,
          }}
        >
          <Box component="span" sx={{width: '6px', height: '6px', borderRadius: '50%', bgcolor: ACCENT, boxShadow: '0 0 0 3px rgba(245,180,70,0.18)'}} />
          {versionMetadata.label}
        </Box>

        <Box
          component="span"
          sx={{
            fontSize: '13px',
            lineHeight: 1.5,
            color: 'rgba(255,255,255,0.75)',
            '[data-theme="light"] &': {color: 'rgba(0,0,0,0.65)'},
          }}
        >
          <BannerMessage banner={versionMetadata.banner} />
        </Box>

        <Box
          component={Link}
          to={latestVersionSuggestedDoc.path}
          onClick={() => savePreferredVersionName(latestVersionSuggestion.name)}
          sx={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: '5px',
            fontSize: '13px',
            fontWeight: 500,
            color: ACCENT,
            textDecoration: 'none',
            flexShrink: 0,
            transition: 'opacity 0.15s',
            '&:hover': {opacity: 0.8, color: ACCENT},
          }}
        >
          <Translate id="theme.docs.versions.latestVersionSuggestionLabel" description="The label used to tell the user to check the latest version">
            View latest
          </Translate>
          <Box component="span" sx={{fontFamily: "'JetBrains Mono', monospace", fontSize: '12px'}}>
            {latestVersionSuggestion.label}
          </Box>
          <ArrowRight size={13} />
        </Box>
      </Box>
    </Box>
  );
}
