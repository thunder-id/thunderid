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
import TOCMobile from '@theme-original/DocItem/TOC/Mobile';
import {Box} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import AIPageActions from '@site/src/components/AIPageActions';

export default function DocItemTOCMobileWrapper(): JSX.Element {
  const {metadata, frontMatter} = useDoc();
  const isHomePage = metadata.id === 'index';
  const showButtons = !isHomePage && !frontMatter.hide_title;

  return (
    <>
      <TOCMobile />
      {showButtons && (
        // The mobile TOC wrapper stays mounted at all viewport widths (only CSS-hidden
        // above 996px), so this needs the same breakpoint to avoid double buttons.
        <Box sx={{mb: 2, '@media (min-width: 997px)': {display: 'none'}}}>
          <AIPageActions />
        </Box>
      )}
    </>
  );
}
