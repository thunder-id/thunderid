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

import type {Plugin, AllContent} from '@docusaurus/types';

export type Maturity = 'preview' | 'beta';

interface DocFrontMatter {
  maturity?: Maturity;
  [key: string]: unknown;
}

interface LoadedDoc {
  id: string;
  frontMatter: DocFrontMatter;
}

interface LoadedVersion {
  docs: LoadedDoc[];
}

interface LoadedContent {
  loadedVersions: LoadedVersion[];
}

/**
 * Reads the `maturity` frontmatter field from every doc and exposes a
 * `{ maturityMap: Record<string, Maturity> }` global data object.
 *
 * The map key is the doc ID and the value is the maturity level ("preview" or
 * "beta"). Pages without a `maturity` field are omitted — they have no special
 * maturity state.
 */
export default function maturityPlugin(): Plugin {
  return {
    name: 'product-maturity-plugin',

    allContentLoaded({
      allContent,
      actions,
    }: {
      allContent: AllContent;
      actions: {setGlobalData: (data: unknown) => void};
    }) {
      const {setGlobalData} = actions;

      const docsContent = allContent['docusaurus-plugin-content-docs']?.default as LoadedContent | undefined;

      const maturityMap: Record<string, Maturity> = {};

      if (docsContent?.loadedVersions) {
        for (const version of docsContent.loadedVersions) {
          for (const doc of version.docs) {
            const maturity = doc.frontMatter?.maturity;
            if (maturity === 'preview' || maturity === 'beta') {
              maturityMap[doc.id] = maturity;
            }
          }
        }
      }

      setGlobalData({maturityMap});
    },
  };
}
