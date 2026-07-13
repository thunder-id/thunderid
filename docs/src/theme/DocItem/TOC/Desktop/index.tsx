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

import {useDoc} from '@docusaurus/plugin-content-docs/client';
import TOCDesktop from '@theme-original/DocItem/TOC/Desktop';
import {Box} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import AIPageActions from '@site/src/components/AIPageActions';

export default function DocItemTOCDesktopWrapper(): JSX.Element {
  const {metadata, frontMatter} = useDoc();
  const isHomePage = metadata.id === 'index';
  const showButtons = !isHomePage && !frontMatter.hide_title;

  return (
    // A single sticky container for the TOC + actions together — the TOC's own built-in
    // sticky positioning is neutralized in custom.css, otherwise it would stay pinned
    // independently while this actions sibling (plain normal-flow content) keeps
    // scrolling with the page underneath it. The TOC and actions are separate flex
    // items so a long TOC scrolls in its own region instead of pushing the actions
    // list out of view.
    <Box
      sx={{
        position: 'sticky',
        top: 'calc(var(--ifm-navbar-height) + 1rem)',
        maxHeight: 'calc(100vh - (var(--ifm-navbar-height) + 2rem))',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <Box sx={{minHeight: 0, overflowY: 'auto'}}>
        <TOCDesktop />
      </Box>
      {showButtons && (
        <Box sx={{flexShrink: 0, mt: 2, pl: '0.75rem', pb: 2}}>
          <AIPageActions variant="list" />
        </Box>
      )}
    </Box>
  );
}
