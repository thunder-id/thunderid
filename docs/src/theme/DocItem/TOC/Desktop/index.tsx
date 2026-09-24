// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useDoc} from '@docusaurus/plugin-content-docs/client';
import TOCDesktop from '@theme-original/DocItem/TOC/Desktop';
import {Box} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import AIPageActions from '@site/src/components/AIPageActions';

export default function DocItemTOCDesktopWrapper(): JSX.Element {
  const {metadata, frontMatter} = useDoc();
  const isHomePage = metadata.id === 'index';
  const showButtons = !isHomePage && !frontMatter.hide_title;

  return (
    // A single sticky container for the TOC + actions together — the TOC's own built-in
    // sticky positioning is neutralized in custom.css, otherwise it would stay pinned
    // independently while this actions sibling (plain normal-flow content) keeps
    // scrolling with the page underneath it. No max-height/overflow here (matching the
    // SDK detail page's rail, Ecosystem/Detail/Detail.tsx's `Rail`): everything renders
    // at its natural full height, and ordinary `position: sticky` handles the rest —
    // it stays pinned while there's room, then scrolls with the page once its own
    // content is taller than the viewport, rather than being force-capped into an
    // internally-scrolling box that hides "Explore with AI" until scrolled into view.
    // `flexShrink: 0` on both children is required, not decorative: a `position: sticky`
    // flex container apparently still feeds an implicit available-height into the flex
    // algorithm even with no `maxHeight` set, so without this the two children shrink
    // and visibly overlap instead of stacking at their natural sizes. `TOCDesktop`
    // itself (not a wrapping `Box` around it) has to be the flex item that gets it —
    // wrapping it introduces a plain block parent, and `.theme-doc-toc-desktop`'s own
    // `margin-top: 2.7rem` (custom.css) then collapses through that block wrapper,
    // desyncing the wrapper's measured height from its child's real one and
    // reproducing the same overlap. `flexShrink: 0` on `.theme-doc-toc-desktop` itself
    // is set in custom.css instead, since it has no `sx` prop to set it on here.
    <Box
      sx={{
        position: 'sticky',
        top: 'calc(var(--ifm-navbar-height) + 1rem)',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <TOCDesktop />
      {showButtons && (
        <Box sx={{flexShrink: 0, mt: 2, pl: '0.75rem', pb: 2}}>
          <AIPageActions variant="list" />
        </Box>
      )}
    </Box>
  );
}
