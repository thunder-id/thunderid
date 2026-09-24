// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTabs} from '@docusaurus/theme-common/internal';
import TabItem from '@theme/TabItem';
import React, {JSX, PropsWithChildren, ReactElement, useRef} from 'react';
import {CodeGroupTabsProvider} from './CodeGroupContext';
import {PACKAGE_MANAGER_ICONS} from './packageManagerIcons';

interface CodeGroupProps {
  /**
   * The default tab to show
   */
  defaultValue?: string;
  /**
   * Group ID for syncing tabs across the page
   */
  groupId?: string;
}

interface CodeBlockProps {
  label?: string;
  lang?: string;
  children?: React.ReactNode;
}

/**
 * CodeGroup component for displaying multiple code blocks in tabs
 *
 * @example
 * ```tsx
 * <CodeGroup>
 *   <CodeBlock lang="bash" label="npm">
 *     npm install @example/react
 *   </CodeBlock>
 *   <CodeBlock lang="bash" label="Yarn">
 *     yarn add @example/react
 *   </CodeBlock>
 * </CodeGroup>
 * ```
 */
export default function CodeGroup({
  defaultValue = undefined,
  groupId = 'code-group',
  children = null,
}: PropsWithChildren<CodeGroupProps>): JSX.Element {
  // Convert children to array
  const childArray = React.Children.toArray(children) as ReactElement<CodeBlockProps>[];

  // Get the first child's label as default if not specified
  const firstLabel = childArray[0]?.props?.label;
  const defaultTab = defaultValue ?? firstLabel?.toLowerCase();

  // `TabItem` elements built only to feed `useTabs()`'s value/label extraction — never
  // rendered. Driving selection through the hook ourselves (instead of `@theme/Tabs`'s own
  // rendering) is what lets the tab switcher live inside the active child's own
  // `.tid-codeblock__bar` rather than as a separate `<ul>` floating above the code block.
  const tabItems = childArray.map((child, index) => {
    const label = child.props?.label ?? `Tab ${index + 1}`;
    return (
      <TabItem key={label} value={label.toLowerCase()} label={label}>
        {null}
      </TabItem>
    );
  });

  const {selectedValue, selectValue, tabValues} = useTabs({children: tabItems, groupId, defaultValue: defaultTab});

  // Stable across re-renders (this component never remounts when its active child does),
  // so it survives the unmount/remount a tab switch triggers below — see CodeGroupContext.
  const focusOnMountRef = useRef(false);

  const tabs = tabValues.map((tabValue) => ({
    value: tabValue.value,
    label: tabValue.label ?? tabValue.value,
    icon: PACKAGE_MANAGER_ICONS[tabValue.value],
    active: tabValue.value === selectedValue,
    onSelect: () => {
      focusOnMountRef.current = true;
      selectValue(tabValue.value);
    },
  }));

  const activeChild =
    childArray.find((child, index) => (child.props?.label ?? `Tab ${index + 1}`).toLowerCase() === selectedValue) ??
    childArray[0];

  return <CodeGroupTabsProvider value={{tabs, focusOnMountRef}}>{activeChild}</CodeGroupTabsProvider>;
}
