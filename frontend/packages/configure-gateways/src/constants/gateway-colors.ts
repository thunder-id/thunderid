// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The color a gateway is marked with, for dark and light color schemes. */
export interface GatewayColor {
  dark: string;
  light: string;
}

/**
 * The colors gateways are marked with, by their place in the pipeline, repeating after the last.
 * The light colors are darker, so a gateway's name reads on a light background too.
 */
export const GATEWAY_COLORS: readonly GatewayColor[] = [
  {dark: '#5CD1FF', light: '#0369A1'},
  {dark: '#FF8FC7', light: '#BE185D'},
  {dark: '#B79CFF', light: '#6D28D9'},
  {dark: '#6EE7A8', light: '#047857'},
];

/** The color of the gateway at a place in the pipeline. */
export function gatewayColor(index: number): GatewayColor {
  return GATEWAY_COLORS[((index % GATEWAY_COLORS.length) + GATEWAY_COLORS.length) % GATEWAY_COLORS.length];
}
