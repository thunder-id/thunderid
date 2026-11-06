// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The color an environment is marked with, for dark and light color schemes. */
export interface EnvironmentColor {
  dark: string;
  light: string;
}

/**
 * The colors environments are marked with, by their place in the pipeline, repeating after the last.
 * The light colors are darker, so an environment's name reads on a light background too.
 */
export const ENVIRONMENT_COLORS: readonly EnvironmentColor[] = [
  {dark: '#5CD1FF', light: '#0369A1'},
  {dark: '#FF8FC7', light: '#BE185D'},
  {dark: '#B79CFF', light: '#6D28D9'},
  {dark: '#6EE7A8', light: '#047857'},
];

/** The color of the environment at a place in the pipeline. */
export function environmentColor(index: number): EnvironmentColor {
  return ENVIRONMENT_COLORS[
    ((index % ENVIRONMENT_COLORS.length) + ENVIRONMENT_COLORS.length) % ENVIRONMENT_COLORS.length
  ];
}
