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
      <Box sx={{position: 'relative', minHeight, height: '100%', overflow: 'hidden'}}>
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
