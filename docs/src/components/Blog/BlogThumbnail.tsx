// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import {BLOG_HERO_ICONS, BlogHeroIconKey} from './icons';

interface BlogThumbnailProps {
  gradient: string;
  icon: BlogHeroIconKey;
  category: string;
  image?: string;
  iconSize?: number;
  minHeight?: number | {xs: number; md?: number};
}

export default function BlogThumbnail({
  gradient,
  icon,
  category,
  image = undefined,
  iconSize = 44,
  minHeight = 168,
}: BlogThumbnailProps): JSX.Element {
  const Icon = BLOG_HERO_ICONS[icon] ?? BLOG_HERO_ICONS.default;

  if (image) {
    return (
      <Box sx={{position: 'relative', height: minHeight, overflow: 'hidden'}}>
        <Box component="img" src={image} alt="" sx={{width: '100%', height: '100%', objectFit: 'cover'}} />
      </Box>
    );
  }

  return (
    <Box
      sx={{
        position: 'relative',
        minHeight,
        height: '100%',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        overflow: 'hidden',
        backgroundImage: `radial-gradient(circle at 75% 22%, rgba(139,249,250,0.2), transparent 55%), ${gradient}`,
      }}
    >
      <Box
        sx={{
          position: 'absolute',
          inset: 0,
          backgroundSize: '28px 28px',
          backgroundImage:
            'linear-gradient(rgba(255,255,255,0.06) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.06) 1px, transparent 1px)',
          maskImage: 'radial-gradient(ellipse 80% 80% at 50% 50%, black 30%, transparent 100%)',
          WebkitMaskImage: 'radial-gradient(ellipse 80% 80% at 50% 50%, black 30%, transparent 100%)',
        }}
      />
      <Box sx={{position: 'relative', color: 'rgba(255,255,255,0.92)'}}>
        <Icon size={iconSize} />
      </Box>
      <Typography
        component="span"
        sx={{
          position: 'absolute',
          top: 12,
          left: 12,
          fontFamily: 'monospace',
          fontSize: '9.5px',
          fontWeight: 600,
          textTransform: 'uppercase',
          letterSpacing: '0.06em',
          color: '#fff',
          bgcolor: 'rgba(6,13,26,0.55)',
          border: '1px solid rgba(255,255,255,0.18)',
          borderRadius: '6px',
          px: 1,
          py: 0.5,
          backdropFilter: 'blur(4px)',
        }}
      >
        {category}
      </Typography>
    </Box>
  );
}
