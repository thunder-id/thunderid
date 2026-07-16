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

import Link from '@docusaurus/Link';
import type {BlogSidebar as BlogSidebarProp} from '@docusaurus/plugin-content-blog';
import type {BlogPostContextValue} from '@docusaurus/plugin-content-blog/client';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import TOC from '@theme/TOC';
import {Box, Typography, styled} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import {formatDate} from './helpers';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import useReadingProgress from '../../hooks/useReadingProgress';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import AIPageActions from '@site/src/components/AIPageActions';
import GithubIcon from '@site/src/components/icons/GithubIcon';

const TocContainer = styled(Box)(({theme}) => ({
  // The outer <aside> is already the sticky/scroll anchor for this column (it also
  // holds the action buttons, "more posts", and the GitHub card below the TOC), so
  // the TOC's own built-in sticky + max-height + internal scroll must be neutralized
  // here — otherwise it reserves near-full-viewport height and the actions below it
  // end up overlapping it instead of following it.
  '& .blog-post-toc': {
    position: 'static',
    maxHeight: 'none',
    overflow: 'visible',
  },
  '& .table-of-contents': {
    margin: 0,
    padding: 0,
    listStyle: 'none',
    borderLeft: 'none',
  },
  '& .table-of-contents li': {
    margin: 0,
  },
  '& .table-of-contents__link': {
    display: 'block',
    padding: '6px 0 6px 12px',
    borderLeft: '2px solid transparent',
    fontSize: '13px',
    lineHeight: 1.5,
    color: theme.vars?.palette.text.secondary,
    transition: 'color 0.15s, border-color 0.15s',
  },
  '& .table-of-contents__link:hover': {
    color: theme.vars?.palette.text.primary,
  },
  '& .table-of-contents__link--active': {
    color: theme.vars?.palette.primary.main,
    borderLeftColor: theme.vars?.palette.primary.main,
    fontWeight: 500,
  },
}));

interface BlogPostSidebarProps {
  content: BlogPostContextValue;
  sidebar: BlogSidebarProp;
}

export default function BlogPostSidebar({content, sidebar}: BlogPostSidebarProps): JSX.Element {
  const isLight = !useIsDarkMode();
  const progress = useReadingProgress();
  const {siteConfig} = useDocusaurusContext();
  const {project} = siteConfig.customFields?.product as DocusaurusProductConfig;
  const {toc, metadata, frontMatter} = content;
  const {hide_table_of_contents: hideToc, toc_min_heading_level: tocMin, toc_max_heading_level: tocMax} = frontMatter;

  const morePosts = sidebar.items.filter((item) => item.permalink !== metadata.permalink).slice(0, 4);

  return (
    <Box component="aside" sx={{position: {md: 'sticky'}, top: {md: 96}}}>
      <Box sx={{height: 2, bgcolor: isLight ? 'rgba(0,0,0,0.06)' : 'rgba(255,255,255,0.06)', borderRadius: 2, mb: 3.5, overflow: 'hidden'}}>
        <Box
          sx={{
            height: '100%',
            width: `${progress * 100}%`,
            background: 'linear-gradient(90deg,#2560d9,#8bf9fa)',
            borderRadius: 2,
          }}
        />
      </Box>

      {!hideToc && toc.length > 0 && (
        <Box sx={{mb: 4}}>
          <Typography
            component="div"
            sx={{
              fontFamily: 'monospace',
              fontSize: '10px',
              textTransform: 'uppercase',
              letterSpacing: '0.12em',
              color: isLight ? 'rgba(0,0,0,0.3)' : 'rgba(255,255,255,0.3)',
              mb: 1.5,
              pl: 1.5,
            }}
          >
            On this page
          </Typography>
          <TocContainer>
            <TOC toc={toc} minHeadingLevel={tocMin} maxHeadingLevel={tocMax} className="blog-post-toc" />
          </TocContainer>
          <Box sx={{mt: 2, pl: 1.5}}>
            <AIPageActions variant='list' />
          </Box>
        </Box>
      )}

      {morePosts.length > 0 && (
        <Box sx={{borderTop: '1px solid', borderColor: isLight ? 'rgba(0,0,0,0.07)' : 'rgba(255,255,255,0.07)', pt: 3, mb: 3}}>
          <Typography
            component="div"
            sx={{
              fontFamily: 'monospace',
              fontSize: '10px',
              textTransform: 'uppercase',
              letterSpacing: '0.12em',
              color: isLight ? 'rgba(0,0,0,0.3)' : 'rgba(255,255,255,0.3)',
              mb: 2,
            }}
          >
            More from the blog
          </Typography>
          <Box sx={{display: 'flex', flexDirection: 'column', gap: 1.5}}>
            {morePosts.map((item) => (
              <Box
                key={item.permalink}
                component={Link}
                to={item.permalink}
                sx={{
                  display: 'block',
                  p: 1.75,
                  border: '1px solid',
                  borderColor: isLight ? 'rgba(0,0,0,0.06)' : 'rgba(255,255,255,0.06)',
                  borderRadius: '10px',
                  bgcolor: isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.02)',
                  transition: 'all 0.2s ease',
                  '&:hover': {borderColor: 'rgba(54,136,255,0.3)', bgcolor: 'rgba(54,136,255,0.04)'},
                }}
              >
                <Typography sx={{fontSize: '13px', fontWeight: 600, lineHeight: 1.35, color: 'text.primary', mb: 0.5, textWrap: 'pretty'}}>
                  {item.title}
                </Typography>
                <Typography sx={{fontFamily: 'monospace', fontSize: '10.5px', color: isLight ? 'rgba(0,0,0,0.34)' : 'rgba(255,255,255,0.34)'}}>
                  {formatDate(String(item.date))}
                </Typography>
              </Box>
            ))}
          </Box>
        </Box>
      )}

      <Box
        sx={{
          p: 2.25,
          borderRadius: '12px',
          bgcolor: 'rgba(54,136,255,0.07)',
          border: '1px solid rgba(54,136,255,0.18)',
        }}
      >
        <Typography sx={{fontSize: '13px', fontWeight: 600, color: 'text.primary', mb: 0.75}}>Star on GitHub</Typography>
        <Typography sx={{fontSize: '12px', lineHeight: 1.55, color: 'text.secondary', mb: 1.75}}>
          Follow the open-source build — issues, RFCs, and releases.
        </Typography>
        <Box
          component="a"
          href={project.source.github.url}
          target="_blank"
          rel="noopener noreferrer"
          sx={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            gap: 0.875,
            py: 1.125,
            px: 1.75,
            background: 'linear-gradient(135deg,#2560d9,#3688ff)',
            borderRadius: '8px',
            fontSize: '13px',
            fontWeight: 600,
            color: '#fff',
            textDecoration: 'none',
            transition: 'all 0.2s ease',
            '&:hover': {transform: 'translateY(-1px)', boxShadow: '0 8px 20px rgba(54,136,255,0.4)'},
          }}
        >
          <GithubIcon size={13} />
          View on GitHub
        </Box>
      </Box>
    </Box>
  );
}
