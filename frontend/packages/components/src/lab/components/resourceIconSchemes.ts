// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {EMOJI_URI_SCHEME as EMOJI_SCHEME, resolveLogoUri, isAvatarUri} from '@thunderid/react';
import {isAbsoluteUrl as isUrl} from '@thunderid/utils';

export type ResolvedResourceIcon = {type: 'emoji'; char: string} | {type: 'image'; src: string};

/**
 * Resolves any resource-icon spec (`emoji:`, `avatar:`, or a raw URL/emoji) into a
 * renderable representation.
 */
export function resolveResourceIcon(value: string, seedText = ''): ResolvedResourceIcon {
  if (value.startsWith(EMOJI_SCHEME)) {
    return {char: value.slice(EMOJI_SCHEME.length), type: 'emoji'};
  }
  if (isAvatarUri(value)) {
    try {
      return {src: resolveLogoUri(value, seedText).imgSrc ?? '', type: 'image'};
    } catch {
      return {char: '', type: 'emoji'};
    }
  }
  if (isUrl(value)) {
    return {src: value, type: 'image'};
  }
  // Backwards compatibility: a bare, unprefixed emoji character.
  return {char: value, type: 'emoji'};
}
