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

export default function BlogFeaturedCard({content}: {content: Content}): JSX.Element {
  const theme = useTheme();
  const isLight = !useIsDarkMode();
  const {metadata} = content;
  const author = metadata.authors[0];

  return (
    <Box sx={{maxWidth: 1200, width: '100%', mx: 'auto', px: {xs: 2, sm: 4}, pt: {xs: 4, md: 5}, pb: {xs: 4, md: 5}}}>
      <Box
        component={Link}
        to={metadata.permalink}
        sx={{
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', md: '1.15fr 1fr'},
          borderRadius: '20px',
          overflow: 'hidden',
          border: '1px solid',
          borderColor: isLight ? 'rgba(0,0,0,0.08)' : 'rgba(255,255,255,0.08)',
          bgcolor: isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.02)',
          textDecoration: 'none',
          transition: 'all 0.2s ease',
          '&:hover': {
            borderColor: theme.vars?.palette.primary.main,
            boxShadow: '0 22px 56px rgba(0,0,0,0.4)',
            transform: 'translateY(-3px)',
          },
        }}
      >
        <BlogThumbnail
          gradient={getHeroGradient(content)}
          icon={getHeroIcon(content)}
          category={getCategory(content)}
          image={getThumbnail(content)}
          iconSize={72}
          minHeight={{xs: 220, md: 340}}
        />

        <Box sx={{p: {xs: 3, md: 5}, display: 'flex', flexDirection: 'column', justifyContent: 'center'}}>
          <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5, mb: 2}}>
            <Box
              component="span"
              sx={{
                fontFamily: 'monospace',
                fontSize: '10px',
                fontWeight: 600,
                textTransform: 'uppercase',
                color: isLight ? '#1856b3' : '#8bf9fa',
                bgcolor: 'rgba(54,136,255,0.12)',
                border: '1px solid rgba(54,136,255,0.28)',
                borderRadius: '6px',
                px: 1.1,
                py: 0.4,
              }}
            >
              Featured
            </Box>
            <Typography
              component="span"
              sx={{
                fontFamily: 'monospace',
                fontSize: '11.5px',
                color: isLight ? 'rgba(0,0,0,0.4)' : 'rgba(255,255,255,0.4)',
              }}
            >
              {getCategory(content)}
            </Typography>
          </Box>

          <Typography
            component="h2"
            sx={{
              fontSize: {xs: '24px', md: '32px'},
              fontWeight: 700,
              letterSpacing: '-0.025em',
              color: 'text.primary',
              mb: 1.5,
            }}
          >
            {metadata.title}
          </Typography>

          <Typography sx={{fontSize: '14.5px', lineHeight: 1.7, color: 'text.secondary', mb: 3}}>
            {metadata.description}
          </Typography>

          {author && (
            <Box sx={{display: 'flex', alignItems: 'center', gap: 1.25}}>
              <BlogAvatar name={author.name ?? ''} imageURL={author.imageURL} size={34} />
              <Box>
                <Typography sx={{fontSize: '12.5px', fontWeight: 600, color: isLight ? 'rgba(0,0,0,0.82)' : 'rgba(255,255,255,0.82)'}}>
                  {author.name}
                </Typography>
                <Typography
                  sx={{
                    fontFamily: 'monospace',
                    fontSize: '11px',
                    color: isLight ? 'rgba(0,0,0,0.4)' : 'rgba(255,255,255,0.4)',
                  }}
                >
                  {formatMetaLine(metadata.date, metadata.readingTime)}
                </Typography>
              </Box>
            </Box>
          )}
        </Box>
      </Box>
    </Box>
  );
}
