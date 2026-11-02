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

export default function BlogPostCard({content}: {content: Content}): JSX.Element {
  const theme = useTheme();
  const isLight = !useIsDarkMode();
  const {metadata} = content;

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
        <BlogAuthorGroup
          authors={metadata.authors}
          avatarSize={30}
          isLight={isLight}
          nameFontSize="12px"
          subtitleFontSize="10.5px"
          subtitleFontFamily="monospace"
          subtitle={formatMetaLine(metadata.date, metadata.readingTime)}
        />
      </Box>
    </Box>
  );
}
