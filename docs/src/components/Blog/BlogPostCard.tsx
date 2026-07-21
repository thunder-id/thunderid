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
import type {Content} from '@theme/BlogPostPage';
import {Box, Typography, useTheme} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import BlogAvatar from './BlogAvatar';
import BlogThumbnail from './BlogThumbnail';
import {formatMetaLine, getCategory, getHeroGradient, getHeroIcon, getThumbnail} from './helpers';
import useIsDarkMode from '../../hooks/useIsDarkMode';

export default function BlogPostCard({content}: {content: Content}): JSX.Element {
  const theme = useTheme();
  const isLight = !useIsDarkMode();
  const {metadata} = content;
  const author = metadata.authors[0];

  return (
    <Box
      component={Link}
      to={metadata.permalink}
      sx={{
        display: 'flex',
        flexDirection: 'column',
        height: '100%',
        borderRadius: '16px',
        overflow: 'hidden',
        border: '1px solid',
        borderColor: isLight ? 'rgba(0,0,0,0.07)' : 'rgba(255,255,255,0.07)',
        bgcolor: isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.02)',
        textDecoration: 'none',
        transition: 'all 0.2s ease',
        '&:hover': {
          borderColor: theme.vars?.palette.primary.main,
          boxShadow: '0 18px 44px rgba(0,0,0,0.36)',
          transform: 'translateY(-4px)',
        },
      }}
    >
      <BlogThumbnail
        gradient={getHeroGradient(content)}
        icon={getHeroIcon(content)}
        category={getCategory(content)}
        image={getThumbnail(content)}
      />

      <Box sx={{p: '22px 22px 20px', display: 'flex', flexDirection: 'column', flex: 1}}>
        <Typography
          component="h3"
          sx={{fontSize: '16.5px', fontWeight: 700, letterSpacing: '-0.015em', lineHeight: 1.32, color: 'text.primary', mb: 1}}
        >
          {metadata.title}
        </Typography>
        <Typography sx={{fontSize: '13.5px', lineHeight: 1.65, color: 'text.secondary', flex: 1, mb: 2}}>
          {metadata.description}
        </Typography>
        {author && (
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
            <BlogAvatar name={author.name ?? ''} imageURL={author.imageURL} size={30} />
            <Box>
              <Typography sx={{fontSize: '12px', fontWeight: 600, color: isLight ? 'rgba(0,0,0,0.78)' : 'rgba(255,255,255,0.78)'}}>
                {author.name}
              </Typography>
              <Typography
                sx={{fontFamily: 'monospace', fontSize: '10.5px', color: isLight ? 'rgba(0,0,0,0.4)' : 'rgba(255,255,255,0.4)'}}
              >
                {formatMetaLine(metadata.date, metadata.readingTime)}
              </Typography>
            </Box>
          </Box>
        )}
      </Box>
    </Box>
  );
}
