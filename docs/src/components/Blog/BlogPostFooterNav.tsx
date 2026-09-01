// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import type {BlogPostContextValue} from '@docusaurus/plugin-content-blog/client';
import {Box, Typography} from '@wso2/oxygen-ui';
import {ArrowLeft, ArrowRight} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import BlogAuthorGroup from './BlogAuthorGroup';
import useIsDarkMode from '../../hooks/useIsDarkMode';

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
