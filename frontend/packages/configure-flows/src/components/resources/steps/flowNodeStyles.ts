// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, styled, type BoxProps, type CSSObject, type Theme} from '@wso2/oxygen-ui';
import type {ComponentType} from 'react';

/**
 * Outcome a source handle stands for. Each maps onto a palette color below.
 */
export type OutcomeHandleKind = 'success' | 'failure' | 'incomplete';

const OUTCOME_COLORS: Record<OutcomeHandleKind, 'success' | 'error' | 'warning'> = {
  failure: 'error',
  incomplete: 'warning',
  success: 'success',
};

// `theme.vars` only exists under the CSS variables provider; reading through the
// plain theme keeps these styles resolvable when one is not mounted.
const palette = (theme: Theme): Theme['palette'] => (theme.vars ?? theme).palette as Theme['palette'];

/**
 * Blends a node surface color towards the primary color, used for the hover
 * tints and the call step's primary wash.
 *
 * @param theme - The active theme.
 * @param surface - The surface color to blend from.
 * @param percentage - How much of the surface color to keep.
 * @returns A `color-mix()` expression.
 */
export const mixWithPrimary = (theme: Theme, surface: string, percentage: number): string =>
  `color-mix(in srgb, ${surface} ${percentage}%, ${palette(theme).primary.main})`;

/**
 * Builds a light/dark pair of styles from the executor surface grey of each scheme.
 *
 * @param theme - The active theme.
 * @param build - Style factory receiving the scheme's surface color.
 * @returns The scheme scoped styles.
 */
export const executionSurfaceScheme = (theme: Theme, build: (surface: string) => CSSObject): CSSObject => ({
  ...theme.applyStyles('light', build(palette(theme).grey[100])),
  ...theme.applyStyles('dark', build(palette(theme).grey[900])),
});

/**
 * Per scheme shadow shared by every node card and chip on the canvas.
 *
 * @param theme - The active theme.
 * @returns The shadow styles.
 */
export const nodeShadowMixin = (theme: Theme): CSSObject => ({
  ...theme.applyStyles('light', {
    boxShadow:
      '0 10px 22px 0 rgba(6, 6, 14, 0.1), 0 24px 48px 0 rgba(199, 211, 234, 0.05) inset, 0 1px 1px 0 rgba(199, 211, 234, 0.12) inset',
  }),
  ...theme.applyStyles('dark', {
    boxShadow:
      '0 24px 32px 0 rgba(6, 6, 14, 0.7), 0 24px 48px 0 rgba(199, 211, 234, 0.05) inset, 0 1px 1px 0 rgba(199, 211, 234, 0.12) inset',
  }),
});

/**
 * Executor surface: the body of an executor card, the compact chip and the stack chip.
 *
 * @param theme - The active theme.
 * @returns The surface styles.
 */
export const executionSurfaceMixin = (theme: Theme): CSSObject =>
  executionSurfaceScheme(theme, (surface: string) => ({
    backgroundColor: surface,
    color: palette(theme).text.primary,
    '&:hover': {backgroundColor: mixWithPrimary(theme, surface, 92)},
  }));

/**
 * Call step surface: the executor surface with a permanent primary wash so a call
 * step body does not read as an executor.
 *
 * @param theme - The active theme.
 * @returns The surface styles.
 */
export const callSurfaceMixin = (theme: Theme): CSSObject =>
  executionSurfaceScheme(theme, (surface: string) => ({
    backgroundColor: mixWithPrimary(theme, surface, 92),
    color: palette(theme).text.primary,
    '&:hover': {backgroundColor: mixWithPrimary(theme, surface, 86)},
  }));

export interface OutcomeHandleWrapperProps {
  /**
   * Outcome the wrapped handle stands for.
   */
  kind: OutcomeHandleKind;
  /**
   * Rendered diameter of the handle dot.
   */
  handleSize: number;
}

/**
 * Positions an outcome handle around a node and colors it by outcome. Wrapping the
 * handle is what gives it a tooltip anchor, so the handle itself is laid out
 * statically inside.
 */
const OutcomeHandleWrapper: ComponentType<BoxProps & OutcomeHandleWrapperProps> = styled(Box, {
  shouldForwardProp: (prop: string) => prop !== 'kind' && prop !== 'handleSize',
})<OutcomeHandleWrapperProps>(({theme, kind, handleSize}) => ({
  alignItems: 'center',
  display: 'flex',
  justifyContent: 'center',
  position: 'absolute',
  ...(kind === 'success' && {right: -6, top: '50%', transform: 'translateY(-50%)'}),
  ...(kind === 'failure' && {bottom: -6, left: '50%', transform: 'translateX(-50%)'}),
  ...(kind === 'incomplete' && {left: '50%', top: -6, transform: 'translateX(-50%)'}),
  // The class is doubled so these win over the canvas-wide `.react-flow__handle`
  // sizing rule in DecoratedVisualFlow without reaching for `!important`.
  '& .react-flow__handle.react-flow__handle': {
    backgroundColor: palette(theme)[OUTCOME_COLORS[kind]].main,
    border: `${handleSize > 8 ? 2 : 1}px solid`,
    borderColor: palette(theme)[OUTCOME_COLORS[kind]].dark,
    bottom: 'auto',
    height: handleSize,
    left: 'auto',
    position: 'relative',
    right: 'auto',
    top: 'auto',
    transform: 'none',
    width: handleSize,
  },
}));

export default OutcomeHandleWrapper;
