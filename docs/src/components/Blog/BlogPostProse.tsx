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

import MDXContent from '@theme/MDXContent';
import {Box, styled} from '@wso2/oxygen-ui';
import {JSX, ReactNode} from 'react';

// Typography for rendered blog post markdown. Uses descendant selectors because the
// heading/paragraph/list elements come from MDXContent, which is Docusaurus-controlled.
const Prose = styled(Box)(({theme}) => ({
  '& > h2': {
    fontSize: '26px',
    fontWeight: 700,
    letterSpacing: '-0.022em',
    lineHeight: 1.25,
    margin: '48px 0 14px',
    color: theme.vars?.palette.text.primary,
  },
  '& > h3': {
    fontSize: '18px',
    fontWeight: 600,
    letterSpacing: '-0.015em',
    lineHeight: 1.3,
    margin: '36px 0 10px',
    color: theme.vars?.palette.text.primary,
  },
  '& > p': {
    fontSize: '16.5px',
    lineHeight: 1.78,
    margin: '0 0 22px',
    color: theme.vars?.palette.text.secondary,
  },
  '& > ul, & > ol': {
    margin: '0 0 22px',
    paddingLeft: '22px',
  },
  '& > ul li, & > ol li': {
    fontSize: '16px',
    lineHeight: 1.72,
    marginBottom: '10px',
    color: theme.vars?.palette.text.secondary,
  },
  '& > ul': {listStyle: 'none', paddingLeft: 0},
  '& > ul > li': {position: 'relative', paddingLeft: '20px'},
  '& > ul > li::before': {
    content: '""',
    position: 'absolute',
    left: 0,
    top: '11px',
    width: '6px',
    height: '6px',
    borderRadius: '50%',
    background: theme.vars?.palette.primary.main,
  },
  '& > blockquote': {
    margin: '0 0 26px',
    padding: '18px 22px',
    borderLeft: `3px solid ${theme.vars?.palette.primary.main}`,
    background: 'rgba(54,136,255,0.06)',
    borderRadius: '0 10px 10px 0',
  },
  '& > blockquote p': {
    margin: 0,
    fontSize: '16px',
    fontStyle: 'italic',
    color: theme.vars?.palette.text.secondary,
  },
  '& > hr': {
    border: 'none',
    borderTop: `1px solid ${theme.vars?.palette.divider}`,
    margin: '40px 0',
  },
  '& a': {
    color: theme.vars?.palette.primary.main,
    textDecorationColor: 'rgba(54,136,255,0.4)',
  },
}));

export default function BlogPostProse({children}: {children: ReactNode}): JSX.Element {
  return (
    <Prose>
      <MDXContent>{children}</MDXContent>
    </Prose>
  );
}
