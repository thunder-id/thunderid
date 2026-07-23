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
    return {src: resolveLogoUri(value, seedText).imgSrc ?? '', type: 'image'};
  }
  if (isUrl(value)) {
    return {src: value, type: 'image'};
  }
  // Backwards compatibility: a bare, unprefixed emoji character.
  return {char: value, type: 'emoji'};
}
