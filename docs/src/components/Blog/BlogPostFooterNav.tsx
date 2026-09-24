// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {BlogPostContextValue} from '@docusaurus/plugin-content-blog/client';
import {Box} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import BlogAuthorGroup from './BlogAuthorGroup';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import PrevNextNav from '../PrevNextNav';

export default function BlogPostFooterNav({content}: {content: BlogPostContextValue}): JSX.Element {
  const isLight = !useIsDarkMode();
  const {metadata} = content;
  const {prevItem, nextItem} = metadata;

  return (
    <Box sx={{mt: 6}}>
      {metadata.authors.length > 0 && (
        <Box
          sx={{
            p: 3,
            border: '1px solid',
            borderColor: isLight ? 'rgba(0,0,0,0.08)' : 'rgba(255,255,255,0.08)',
            borderRadius: '16px',
            bgcolor: isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.02)',
          }}
        >
          <BlogAuthorGroup
            authors={metadata.authors}
            avatarSize={52}
            isLight={isLight}
            nameFontSize="15px"
            subtitleFontSize="12.5px"
          />
        </Box>
      )}

      {(prevItem ?? nextItem) && (
        <Box sx={{mt: 3}}>
          <PrevNextNav prev={prevItem ?? undefined} next={nextItem ?? undefined} />
        </Box>
      )}
    </Box>
  );
}
