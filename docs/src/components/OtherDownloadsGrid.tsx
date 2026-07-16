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

/* eslint-disable react-refresh/only-export-components -- Shared styled primitives and a helper icon, imported by multiple pages, not an HMR component boundary. */

import {styled} from '@wso2/oxygen-ui';
import React from 'react';

/**
 * Styled primitives for the "grouped by OS" download grid, shared between the
 * releases page's release card and the standalone DownloadCard component.
 */

export function OtherDownloadsActionIcon(): React.ReactElement {
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4M7 10l5 5 5-5M12 15V3" />
    </svg>
  );
}

export const OtherDownloadsGrid = styled('div')<{ownerState?: {columnMinWidth?: string; compact?: boolean; fixedColumns?: number}}>(
  ({ownerState}) => ({
    display: 'grid',
    gridTemplateColumns: ownerState?.fixedColumns
      ? `repeat(${ownerState.fixedColumns}, minmax(0, 1fr))`
      : `repeat(auto-fit, minmax(${ownerState?.columnMinWidth ?? '300px'}, 1fr))`,
    gap: '0.75rem',
    '@media (max-width: 700px)': {
      gridTemplateColumns: '1fr',
    },
  }),
);

export const OtherDownloadsCard = styled('section')<{ownerState?: {compact?: boolean}}>({
  display: 'flex',
  flexDirection: 'column',
  borderRadius: '12px',
  border: '1px solid var(--ifm-color-emphasis-200)',
  background: 'var(--oxygen-palette-background-paper)',
  overflow: 'hidden',
  '[data-theme="dark"] &': {
    borderColor: 'rgba(255, 255, 255, 0.09)',
    background: 'rgba(255, 255, 255, 0.025)',
  },
});

export const OtherDownloadsHeader = styled('header')({
  display: 'flex',
  alignItems: 'center',
  gap: '0.55rem',
  padding: '0.85rem 1rem 0.7rem',
  borderBottom: '1px solid var(--ifm-color-emphasis-200)',
  '[data-theme="dark"] &': {
    borderBottomColor: 'rgba(255, 255, 255, 0.07)',
  },
});

export const OtherDownloadsHeaderTitle = styled('h4')({
  margin: 0,
  fontSize: '0.82rem',
  fontWeight: 600,
  color: 'var(--ifm-color-emphasis-800)',
});

export const OtherDownloadsOsIcon = styled('span')({
  display: 'inline-flex',
  alignItems: 'center',
  width: '1rem',
  height: '1rem',
  color: 'var(--ifm-color-emphasis-700)',
});

export const OtherDownloadsArchitectures = styled('div')({
  display: 'flex',
  flexDirection: 'column',
  gap: '0.15rem',
  padding: '0.5rem',
});

export const OtherDownloadsArchitecture = styled('a')<{ownerState?: {recommended?: boolean}}>(({ownerState}) => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: '0.75rem',
  padding: '0.6rem',
  borderRadius: '7px',
  color: 'inherit',
  textDecoration: 'none',
  transition: 'background 0.15s ease, box-shadow 0.15s ease',
  '& svg': {
    flexShrink: 0,
    color: ownerState?.recommended ? 'var(--ifm-color-primary)' : 'var(--ifm-color-emphasis-400)',
    transition: 'color 0.15s ease',
  },
  ...(ownerState?.recommended && {
    background: 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.09)',
    boxShadow: 'inset 0 0 0 1px rgb(var(--oxygen-palette-primary-mainChannel) / 0.35)',
  }),
  '&:hover': {
    textDecoration: 'none',
    background: ownerState?.recommended
      ? 'rgb(var(--oxygen-palette-primary-mainChannel) / 0.14)'
      : 'var(--ifm-hover-overlay)',
  },
  ...(!ownerState?.recommended && {
    '[data-theme="dark"] &:hover': {
      background: 'rgba(255, 255, 255, 0.05)',
    },
  }),
  '&:hover svg': {
    color: 'var(--ifm-color-primary)',
  },
}));

export const OtherDownloadsArchitectureInfo = styled('div')({
  minWidth: 0,
});

export const OtherDownloadsArchitectureTitle = styled('div')({
  overflowWrap: 'anywhere',
  color: 'var(--ifm-color-emphasis-800)',
  fontSize: '0.82rem',
  fontWeight: 500,
});

export const OtherDownloadsArchitectureMeta = styled('div')({
  marginTop: '0.12rem',
  color: 'var(--ifm-color-emphasis-500)',
  fontSize: '0.72rem',
  fontVariantNumeric: 'tabular-nums',
});

export const OtherDownloadsArchitectureBadge = styled('span')({
  display: 'inline-block',
  marginTop: '0.2rem',
  fontStyle: 'normal',
  fontSize: '0.62rem',
  fontWeight: 700,
  textTransform: 'uppercase',
  letterSpacing: '0.06em',
  color: 'var(--ifm-color-primary)',
});
