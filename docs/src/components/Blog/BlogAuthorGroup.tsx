// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {JSX, ReactNode} from 'react';
import BlogAvatar from './BlogAvatar';

interface BlogAuthorGroupAuthor {
  name?: string;
  title?: string;
  description?: string;
  imageURL?: string;
}

interface BlogAuthorGroupProps {
  authors: readonly BlogAuthorGroupAuthor[];
  avatarSize: number;
  isLight: boolean;
  nameFontSize: string;
  subtitleFontSize: string;
  subtitleFontFamily?: string;
  // Overrides the author title/description subtitle, e.g. with a date/reading-time meta line.
  subtitle?: ReactNode;
}

function formatNameList(names: string[]): string {
  if (names.length <= 2) return names.join(' and ');
  return `${names.slice(0, -1).join(', ')}, and ${names[names.length - 1]}`;
}

export default function BlogAuthorGroup({
  authors,
  avatarSize,
  isLight,
  nameFontSize,
  subtitleFontSize,
  subtitleFontFamily,
  subtitle: subtitleOverride,
}: BlogAuthorGroupProps): JSX.Element | null {
  if (authors.length === 0) return null;

  const names = formatNameList(authors.map((author) => author.name ?? ''));
  const titles = authors.map((author) => author.description ?? author.title).filter(Boolean);
  const commonTitle = titles.length === authors.length && new Set(titles).size === 1 ? titles[0] : undefined;
  const subtitle = subtitleOverride ?? commonTitle;

  return (
    <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
      <Box sx={{display: 'flex'}}>
        {authors.map((author, index) => (
          <Box
            key={author.name ?? index}
            sx={{
              ml: index === 0 ? 0 : `-${avatarSize * 0.3}px`,
              border: '2px solid',
              borderColor: isLight ? '#fff' : '#0b0e14',
              borderRadius: '50%',
              lineHeight: 0,
            }}
          >
            <BlogAvatar name={author.name ?? ''} imageURL={author.imageURL} size={avatarSize} />
          </Box>
        ))}
      </Box>
      <Box>
        <Typography sx={{fontSize: nameFontSize, fontWeight: 600, color: 'text.primary'}}>{names}</Typography>
        {subtitle && (
          <Typography
            sx={{
              fontSize: subtitleFontSize,
              fontFamily: subtitleFontFamily,
              color: isLight ? 'rgba(0,0,0,0.4)' : 'rgba(255,255,255,0.4)',
            }}
          >
            {subtitle}
          </Typography>
        )}
      </Box>
    </Box>
  );
}
