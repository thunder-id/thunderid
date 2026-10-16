// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {processAdmonitionProps} from '@docusaurus/theme-common';
import type {Props} from '@theme/Admonition';
import {Alert, AlertTitle} from '@wso2/oxygen-ui';
import {JSX} from 'react';

// Docusaurus's own admonition types (note/tip/info/warning/danger, plus the
// undocumented legacy aliases secondary/important/success/caution) collapsed onto
// MUI Alert's four severities, so every `:::type` block in the docs renders as an
// Oxygen UI Alert instead of Infima's default admonition styling.
const SEVERITY: Record<string, 'success' | 'info' | 'warning' | 'error'> = {
  note: 'info',
  tip: 'success',
  info: 'info',
  warning: 'warning',
  danger: 'error',
  secondary: 'info',
  important: 'info',
  success: 'success',
  caution: 'warning',
};

// `props.title` is only set when the author writes a custom title (`:::tip Custom`).
// Docusaurus's own per-type components supply this same default label otherwise;
// replicated here since this file replaces that dispatch entirely.
const DEFAULT_TITLES: Record<string, string> = {
  note: 'Note',
  tip: 'Tip',
  info: 'Info',
  warning: 'Warning',
  danger: 'Danger',
  secondary: 'Note',
  important: 'Info',
  success: 'Success',
  caution: 'Warning',
};

export default function Admonition(unprocessedProps: Props): JSX.Element {
  const {type, title, children} = processAdmonitionProps(unprocessedProps);
  const severity = SEVERITY[type] ?? 'info';
  const heading = title ?? DEFAULT_TITLES[type] ?? type;

  return (
    <Alert severity={severity} sx={{my: '1rem'}}>
      {heading && <AlertTitle>{heading}</AlertTitle>}
      {children}
    </Alert>
  );
}
