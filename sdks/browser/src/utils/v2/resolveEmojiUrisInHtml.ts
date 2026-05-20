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

import {extractEmojiFromUri, isEmojiUri} from '@thunderid/javascript';

/**
 * Resolves `emoji:` URIs in an HTML string.
 *
 * Handles two forms:
 *   - `<img src="emoji:🐯" alt="tiger">` → `<span role="img" aria-label="tiger">🐯</span>`
 *   - Bare `emoji:🐯` text references → `🐯`
 *
 * @param html - The HTML string that may contain `emoji:` URIs.
 * @returns The HTML string with all `emoji:` URIs replaced.
 */
const resolveEmojiUrisInHtml = (html: string): string => {
  const withEmojiImages: string = html.replace(
    /<img([^>]*)src="(emoji:[^"]+)"([^>]*)\/?>/gi,
    (_match: string, pre: string, src: string, post: string): string => {
      const emoji: string = extractEmojiFromUri(src);
      if (!emoji) {
        return _match;
      }
      const altMatch: RegExpMatchArray | null = /alt="([^"]*)"/i.exec(pre + post);
      const label: string = altMatch ? altMatch[1] : emoji;
      return `<span role="img" aria-label="${label}">${emoji}</span>`;
    },
  );
  return withEmojiImages.replace(/emoji:([^\s"<>&]+)/g, (_: string, rest: string): string =>
    isEmojiUri(`emoji:${rest}`) ? rest : `emoji:${rest}`,
  );
};

export default resolveEmojiUrisInHtml;
