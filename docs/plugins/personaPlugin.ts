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

interface DocFrontMatter {
  persona?: string;
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
 * Reads the `persona` frontmatter field from every doc and exposes a
 * `{ personaMap: Record<string, string> }` global data object.
 *
 * The map key is the doc ID (e.g. "guides/flows/build-a-flow") and the
 * value is the persona string ("app" or "iam"). Pages without a `persona`
 * field are omitted — they are always visible regardless of the selected persona.
 */
export default function personaPlugin(): Plugin {
  return {
    name: 'product-persona-plugin',

    allContentLoaded({
      allContent,
      actions,
    }: {
      allContent: AllContent;
      actions: {setGlobalData: (data: unknown) => void};
    }) {
      const {setGlobalData} = actions;

      const docsContent = allContent['docusaurus-plugin-content-docs']?.default as LoadedContent | undefined;

      const personaMap: Record<string, string> = {};

      if (docsContent?.loadedVersions) {
        for (const version of docsContent.loadedVersions) {
          for (const doc of version.docs) {
            const persona = doc.frontMatter?.persona;
            if (typeof persona === 'string' && persona.length > 0) {
              personaMap[doc.id] = persona;
            }
          }
        }
      }

      setGlobalData({personaMap});
    },
  };
}
