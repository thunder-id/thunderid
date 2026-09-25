// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable react-refresh/only-export-components -- `API_REFERENCE_ACTION_BAR_HEIGHT` is shared with `ApiReference`'s own offset calculation, not an HMR component boundary. */

import {JSX, ReactNode} from 'react';
import {createPortal} from 'react-dom';

/**
 * Height of this bar. `ApiReference`'s own `useHeaderOffset` adds this to the navbar +
 * doc-version-banner offset so Scalar's fixed-position viewport starts below it instead of
 * underneath it — keep the two in sync if this value changes.
 */
export const API_REFERENCE_ACTION_BAR_HEIGHT = 44;

interface ApiReferenceActionBarProps {
  /** Distance from the viewport top to where this bar renders (navbar height + doc-version banner, when present). */
  top: number;
  children: ReactNode;
}

/**
 * A bar ThunderID owns outright, rendered directly above the API reference viewport — for the
 * Postman collection download today, and anywhere future page-level actions/settings belong.
 *
 * Deliberately separate from Scalar's own "Developer Tools / Configure / Share / Deploy" row:
 * that row only renders on localhost (Scalar hides it in production), so it's not a place to
 * dock anything that needs to exist for real users.
 */
export default function ApiReferenceActionBar({top, children}: ApiReferenceActionBarProps): JSX.Element {
  return createPortal(
    <div
      style={{
        position: 'fixed',
        top: `${top}px`,
        left: 0,
        right: 0,
        height: `${API_REFERENCE_ACTION_BAR_HEIGHT}px`,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'flex-end',
        gap: '8px',
        padding: '0 16px',
        background: 'var(--scalar-background-1, var(--oxygen-palette-background-default))',
        borderBottom: '1px solid var(--scalar-border-color, var(--ifm-color-emphasis-200))',
        zIndex: 201,
      }}
    >
      {children}
    </div>,
    document.body,
  );
}
