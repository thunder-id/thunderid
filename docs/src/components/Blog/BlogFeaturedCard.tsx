// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import type {Content} from '@theme/BlogPostPage';
import {Box, Typography, useTheme} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import BlogAuthorGroup from './BlogAuthorGroup';
import BlogThumbnail from './BlogThumbnail';
import {formatMetaLine, getCategory, getHeroGradient, getHeroIcon, getThumbnail} from './helpers';
import useIsDarkMode from '../../hooks/useIsDarkMode';

export default function BlogFeaturedCard({content}: {content: Content}): JSX.Element {
  const theme = useTheme();
  const isLight = !useIsDarkMode();
  const {metadata} = content;

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

          <BlogAuthorGroup
            authors={metadata.authors}
            avatarSize={34}
            isLight={isLight}
            nameFontSize="12.5px"
            subtitleFontSize="11px"
            subtitleFontFamily="monospace"
            subtitle={formatMetaLine(metadata.date, metadata.readingTime)}
          />
        </Box>
      </Box>
    </Box>
  );
}
