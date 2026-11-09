// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {createLucideIcon} from '@wso2/oxygen-ui-icons-react';

/**
 * A bot with a cog badge, the icon for agent types. It pairs with `Bot` for agents the way `UserRoundCog` pairs with
 * the user icons for user types. Lucide has no such icon, so it is drawn on Lucide's 24-unit grid from two Lucide
 * icons: the cog badge of `UserRoundCog`, unchanged, and the head of `Bot`, shrunk and moved up and to the left with
 * its antenna, left ear and eyes, without the right ear, and with the lower right corner left open, so every part
 * stays two units clear of the cog, as Lucide's own cog icons do. Being a Lucide icon, it takes the same props as the
 * other Oxygen icons.
 */
const BotCogIcon = createLucideIcon('bot-cog', [
  ['path', {d: 'M10 7V3H6', key: 'antenna'}],
  ['path', {d: 'M16 10.5V9a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v5a2 2 0 0 0 2 2h4.5', key: 'head'}],
  ['path', {d: 'M2 11.5h2', key: 'ear'}],
  ['path', {d: 'M7 10.5v2', key: 'left-eye'}],
  ['path', {d: 'M13 10.5v2', key: 'right-eye'}],
  ['path', {d: 'm14.305 19.53.923-.382', key: 'cog-tooth-1'}],
  ['path', {d: 'm15.228 16.852-.923-.383', key: 'cog-tooth-2'}],
  ['path', {d: 'm16.852 15.228-.383-.923', key: 'cog-tooth-3'}],
  ['path', {d: 'm16.852 20.772-.383.924', key: 'cog-tooth-4'}],
  ['path', {d: 'm19.148 15.228.383-.923', key: 'cog-tooth-5'}],
  ['path', {d: 'm19.53 21.696-.382-.924', key: 'cog-tooth-6'}],
  ['path', {d: 'm20.772 16.852.924-.383', key: 'cog-tooth-7'}],
  ['path', {d: 'm20.772 19.148.924.383', key: 'cog-tooth-8'}],
  ['circle', {cx: '18', cy: '18', r: '3', key: 'cog'}],
]);

export default BotCogIcon;
