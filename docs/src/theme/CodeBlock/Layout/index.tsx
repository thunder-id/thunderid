// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCodeBlockContext} from '@docusaurus/theme-common/internal';
import Buttons from '@theme/CodeBlock/Buttons';
import Container from '@theme/CodeBlock/Container';
import Content from '@theme/CodeBlock/Content';
import clsx from 'clsx';
import {JSX, KeyboardEvent, useEffect, useId, useRef} from 'react';
import {useCodeGroupTabs} from '@site/src/components/CodeGroupContext';

/**
 * Same chrome as the SDKs & Tools registry's `CodeCard` (Ecosystem/Detail/primitives.tsx):
 * a bar that always shows — title on the left, language badge + copy button on the right —
 * rather than theme-classic's default, which only renders a bar when a `title="..."`
 * metastring is given and otherwise leaves the copy button absolutely positioned and
 * hover-only. Making every code block carry the same bar is what makes an .mdx doc's code
 * block and a Ecosystem entry's code block read as the same component, not two systems.
 */
export default function CodeBlockLayout({className = undefined}: {className?: string}): JSX.Element {
  const {metadata} = useCodeBlockContext();
  // Set only when this block is a `<CodeGroup>` child — its package-manager tab switcher
  // renders on the bar's left, in the same slot a `title` would otherwise take, instead of
  // floating above the code block as a separate `<ul>`.
  const groupTabs = useCodeGroupTabs();
  const hasBar = Boolean(metadata.title) || Boolean(metadata.language) || Boolean(groupTabs);
  const baseId = useId();
  const panelId = `${baseId}-panel`;
  const tabRefs = useRef<(HTMLButtonElement | null)[]>([]);

  // `CodeGroup` renders only the active child, so a tab switch unmounts this whole component
  // and mounts a fresh one for the newly active tab — which would otherwise drop focus to
  // `<body>` instead of following the selection. `focusOnMountRef` (set by `CodeGroup` right
  // before the switch, and stable across that unmount/remount because `CodeGroup` itself never
  // remounts) tells this fresh instance to claim focus for its own active tab once.
  useEffect(() => {
    if (!groupTabs?.focusOnMountRef.current) return;
    groupTabs.focusOnMountRef.current = false;
    const activeIndex = groupTabs.tabs.findIndex((tab) => tab.active);
    tabRefs.current[activeIndex]?.focus();
    // Intentionally runs once per mount, not on every `groupTabs` identity change.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Arrow keys both move focus and activate the tab (WAI-ARIA's "automatic activation" tabs
  // pattern, matching how @theme/Tabs itself behaves) rather than just moving focus without
  // selecting.
  const handleTabsKeyDown = (event: KeyboardEvent<HTMLDivElement>): void => {
    if (!groupTabs || (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight')) return;
    event.preventDefault();
    const {tabs} = groupTabs;
    const current = tabs.findIndex((tab) => tab.active);
    const delta = event.key === 'ArrowRight' ? 1 : -1;
    const nextIndex = (current + delta + tabs.length) % tabs.length;
    tabs[nextIndex]?.onSelect();
    // The button this focuses belongs to the about-to-unmount instance, not the fresh one
    // `onSelect` causes to mount — `focusOnMountRef` above is what makes the new instance's
    // own tab actually end up focused.
    tabRefs.current[nextIndex]?.focus();
  };

  return (
    <Container as="div" className={clsx(className, metadata.className, 'tid-codeblock')}>
      {hasBar && (
        <div className="tid-codeblock__bar">
          {groupTabs ? (
            <div className="tid-codeblock__tabs" role="tablist" tabIndex={-1} onKeyDown={handleTabsKeyDown}>
              {groupTabs.tabs.map((tab, index) => (
                <button
                  key={tab.value}
                  ref={(el) => {
                    tabRefs.current[index] = el;
                  }}
                  id={`${baseId}-tab-${tab.value}`}
                  type="button"
                  role="tab"
                  aria-selected={tab.active}
                  aria-controls={panelId}
                  tabIndex={tab.active ? 0 : -1}
                  className={clsx('tid-codeblock__tab', tab.active && 'tid-codeblock__tab--active')}
                  onClick={tab.onSelect}
                >
                  {tab.icon && <img src={tab.icon} alt="" className="codegroup-tab-icon" />}
                  {tab.label}
                </button>
              ))}
            </div>
          ) : (
            <span className="tid-codeblock__title">{metadata.title}</span>
          )}
          <div className="tid-codeblock__meta">
            {metadata.language && <span className="tid-codeblock__lang">{metadata.language}</span>}
            <Buttons />
          </div>
        </div>
      )}
      <div
        className="tid-codeblock__content"
        {...(groupTabs
          ? {
              role: 'tabpanel',
              id: panelId,
              'aria-labelledby': `${baseId}-tab-${groupTabs.tabs.find((tab) => tab.active)?.value}`,
              // The highlighted code itself isn't focusable, so the panel needs to be, per the
              // WAI-ARIA tabs pattern, for keyboard users to reach its content after Tab-ing to
              // the active tab.
              tabIndex: 0,
            }
          : {})}
      >
        <Content />
        {/* No title and no language (a bare ``` fence) is rare, but still needs a way to
            copy the snippet — put the button inline instead of losing it silently. */}
        {!hasBar && <Buttons />}
      </div>
    </Container>
  );
}
