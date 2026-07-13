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
import type {BlogPostContextValue} from '@docusaurus/plugin-content-blog/client';
import {Box, Typography} from '@wso2/oxygen-ui';
import {ArrowLeft, ArrowRight} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import BlogAvatar from './BlogAvatar';
import useIsDarkMode from '../../hooks/useIsDarkMode';

export default function BlogPostFooterNav({content}: {content: BlogPostContextValue}): JSX.Element {
  const isLight = !useIsDarkMode();
  const {metadata} = content;
  const author = metadata.authors[0];
  const {prevItem, nextItem} = metadata;

  return (
    <Box sx={{mt: 6}}>
      {author && (
        <Box
          sx={{
            p: 3,
            border: '1px solid',
            borderColor: isLight ? 'rgba(0,0,0,0.08)' : 'rgba(255,255,255,0.08)',
            borderRadius: '16px',
            bgcolor: isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.02)',
            display: 'flex',
            gap: 2.25,
            alignItems: 'flex-start',
          }}
        >
          <BlogAvatar name={author.name ?? ''} imageURL={author.imageURL} size={52} />
          <Box>
            <Typography sx={{fontSize: '15px', fontWeight: 600, color: 'text.primary', mb: 0.5}}>{author.name}</Typography>
            {(author.description ?? author.title) && (
              <Typography sx={{fontSize: '12.5px', color: isLight ? 'rgba(0,0,0,0.4)' : 'rgba(255,255,255,0.4)'}}>
                {author.description ?? author.title}
              </Typography>
            )}
          </Box>
        </Box>
      )}

      {(prevItem ?? nextItem) && (
        <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', sm: '1fr 1fr'}, gap: 2, mt: 3}}>
          {prevItem ? (
            <Box
              component={Link}
              to={prevItem.permalink}
              sx={{
                p: 2.25,
                border: '1px solid',
                borderColor: isLight ? 'rgba(0,0,0,0.07)' : 'rgba(255,255,255,0.07)',
                borderRadius: '12px',
                bgcolor: isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.02)',
                transition: 'all 0.2s ease',
                '&:hover': {borderColor: 'rgba(54,136,255,0.35)', bgcolor: 'rgba(54,136,255,0.04)'},
              }}
            >
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 0.75,
                  fontFamily: 'monospace',
                  fontSize: '11px',
                  textTransform: 'uppercase',
                  letterSpacing: '0.06em',
                  color: isLight ? 'rgba(0,0,0,0.36)' : 'rgba(255,255,255,0.36)',
                  mb: 1,
                }}
              >
                <ArrowLeft size={11} />
                Previous
              </Box>
              <Typography sx={{fontSize: '14px', fontWeight: 600, color: 'text.primary', lineHeight: 1.35, textWrap: 'pretty'}}>
                {prevItem.title}
              </Typography>
            </Box>
          ) : (
            <Box />
          )}
          {nextItem && (
            <Box
              component={Link}
              to={nextItem.permalink}
              sx={{
                p: 2.25,
                border: '1px solid',
                borderColor: isLight ? 'rgba(0,0,0,0.07)' : 'rgba(255,255,255,0.07)',
                borderRadius: '12px',
                bgcolor: isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.02)',
                textAlign: {sm: 'right'},
                transition: 'all 0.2s ease',
                '&:hover': {borderColor: 'rgba(54,136,255,0.35)', bgcolor: 'rgba(54,136,255,0.04)'},
              }}
            >
              <Box
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: {sm: 'flex-end'},
                  gap: 0.75,
                  fontFamily: 'monospace',
                  fontSize: '11px',
                  textTransform: 'uppercase',
                  letterSpacing: '0.06em',
                  color: isLight ? 'rgba(0,0,0,0.36)' : 'rgba(255,255,255,0.36)',
                  mb: 1,
                }}
              >
                Next
                <ArrowRight size={11} />
              </Box>
              <Typography sx={{fontSize: '14px', fontWeight: 600, color: 'text.primary', lineHeight: 1.35, textWrap: 'pretty'}}>
                {nextItem.title}
              </Typography>
            </Box>
          )}
        </Box>
      )}
    </Box>
  );
}
