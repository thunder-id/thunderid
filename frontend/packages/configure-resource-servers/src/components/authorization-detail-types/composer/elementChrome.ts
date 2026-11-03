// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The name of the canvas as a container, which places a row's toolbar by the room the canvas has. */
export const CANVAS_CONTAINER = 'consent-canvas';

/** The id of a field's row on the card, which takes focus back from the field's settings. */
export function fieldRowId(key: string): string {
  return `${key}-row`;
}

/** The outline an element of the card shows when hovered or selected, as elements of a flow screen do. */
export default function elementChrome(selected: boolean, hasIssues: boolean): Record<string, unknown> {
  const marked = selected || hasIssues;
  return {
    position: 'relative',
    borderRadius: 1,
    outline: '1.5px dashed',
    outlineStyle: marked ? 'solid' : 'dashed',
    outlineColor: hasIssues ? 'error.main' : selected ? 'primary.main' : 'transparent',
    outlineOffset: 1,
    bgcolor: selected ? 'action.selected' : 'transparent',
    cursor: 'pointer',
    transition: 'background-color 0.15s ease-in-out, outline-color 0.15s ease-in-out',
    '&:hover': {
      outlineColor: hasIssues ? 'error.main' : 'primary.main',
      bgcolor: selected ? 'action.selected' : 'action.hover',
    },
    '&:focus-visible': {outlineStyle: 'solid', outlineWidth: 2, outlineColor: 'primary.main'},
    // A hidden toolbar takes no pointer, and only a pointer that hovers reveals it, so a tap on a touch
    // screen selects the row, which shows the toolbar, rather than reaching a control it cannot see.
    '& > .canvas-toolbar': {opacity: marked ? 1 : 0, pointerEvents: marked ? 'auto' : 'none'},
    '&:focus-within > .canvas-toolbar': {opacity: 1, pointerEvents: 'auto'},
    '@media (hover: hover)': {'&:hover > .canvas-toolbar': {opacity: 1, pointerEvents: 'auto'}},
  };
}

/** The id of the summary on the card, which takes focus back from the summary settings. */
export const CANVAS_SUMMARY_ID = 'consent-canvas-summary';
