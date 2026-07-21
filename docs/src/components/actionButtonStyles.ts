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

// Shared muted styling for secondary header actions (Share, Copy as markdown) so they
// read as a consistent group rather than primary CTAs. Carries the same fill + border +
// padding weight as getSplitButtonStyles()'s "main" cell (just neutral instead of accent
// colored) so the two buttons in the stack read as siblings rather than mismatched controls.
export function getActionButtonSx(isLight: boolean) {
  return {
    fontSize: '13px',
    fontWeight: 500,
    textTransform: 'none' as const,
    borderRadius: '8px',
    py: 1,
    px: 1.25,
    borderColor: isLight ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.1)',
    bgcolor: isLight ? 'rgba(0,0,0,0.025)' : 'rgba(255,255,255,0.03)',
    color: isLight ? 'rgba(0,0,0,0.68)' : 'rgba(255,255,255,0.72)',
    '&:hover': {
      borderColor: isLight ? 'rgba(0,0,0,0.22)' : 'rgba(255,255,255,0.2)',
      bgcolor: isLight ? 'rgba(0,0,0,0.05)' : 'rgba(255,255,255,0.06)',
      color: 'text.primary',
    },
  };
}

// Accent-tinted split-button styling for the primary "Open in <assistant>" action —
// the tinted blue reads as a step up from the neutral "Copy as markdown" ghost button,
// and the two halves (direct action vs. dropdown trigger) are separated by a divider
// so each half's click target is legible.
export function getSplitButtonStyles(isLight: boolean) {
  const accent = isLight
    ? {border: 'rgba(37,96,217,0.25)', bg: 'rgba(37,96,217,0.05)', text: '#2560d9', hover: 'rgba(37,96,217,0.08)', divider: 'rgba(37,96,217,0.18)', chevron: 'rgba(37,96,217,0.55)'}
    : {border: 'rgba(54,136,255,0.28)', bg: 'rgba(54,136,255,0.06)', text: '#7fb3ff', hover: 'rgba(54,136,255,0.1)', divider: 'rgba(54,136,255,0.22)', chevron: 'rgba(54,136,255,0.6)'};

  return {
    container: {
      display: 'flex',
      alignItems: 'stretch',
      borderRadius: '8px',
      border: '1px solid',
      borderColor: accent.border,
      bgcolor: accent.bg,
      overflow: 'hidden',
    },
    main: {
      flex: 1,
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      gap: '7px',
      border: 'none',
      bgcolor: 'transparent',
      py: 1,
      px: 1.25,
      fontSize: '13px',
      fontWeight: 500,
      color: accent.text,
      cursor: 'pointer',
      transition: 'background-color 0.15s',
      '&:hover': {bgcolor: accent.hover},
    },
    divider: {
      width: '1px',
      flexShrink: 0,
      bgcolor: accent.divider,
    },
    chevron: {
      display: 'flex',
      alignItems: 'center',
      justifyContent: 'center',
      border: 'none',
      bgcolor: 'transparent',
      px: 1.375,
      color: accent.chevron,
      cursor: 'pointer',
      transition: 'background-color 0.15s',
      '&:hover': {bgcolor: accent.hover},
    },
  };
}
