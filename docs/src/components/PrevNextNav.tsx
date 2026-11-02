// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {Box, Typography} from '@wso2/oxygen-ui';
import {ArrowLeft, ArrowRight} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import useIsDarkMode from '../hooks/useIsDarkMode';

export interface PrevNextLink {
  title: string;
  permalink: string;
}

/**
 * Shared "Previous / Next" footer card pair, originally the blog post footer's own look
 * (Blog/BlogPostFooterNav.tsx) — reused here so a docs page's pagination reads as the same
 * component as a blog post's, instead of the docs side falling back to Infima's plain
 * default `.pagination-nav` styling while the blog side got a bespoke treatment.
 */
export default function PrevNextNav({
  prev = undefined,
  next = undefined,
}: {
  prev?: PrevNextLink;
  next?: PrevNextLink;
}): JSX.Element | null {
  const isLight = !useIsDarkMode();

  if (!prev && !next) return null;

  return (
    <Box
      component="nav"
      aria-label="Previous and next pages"
      sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', sm: '1fr 1fr'}, gap: 2}}
    >
      {prev ? (
        <Box
          component={Link}
          to={prev.permalink}
          sx={{
            p: 2.25,
            border: '1px solid',
            borderColor: isLight ? 'rgba(0,0,0,0.07)' : 'rgba(255,255,255,0.07)',
            borderRadius: '12px',
            bgcolor: isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.02)',
            transition: 'all 0.2s ease',
            textDecoration: 'none',
            '&:hover': {
              borderColor: 'rgba(54,136,255,0.35)',
              bgcolor: 'rgba(54,136,255,0.04)',
              textDecoration: 'none',
            },
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
              color: isLight ? 'rgba(0,0,0,0.6)' : 'rgba(255,255,255,0.65)',
              mb: 1,
            }}
          >
            <ArrowLeft size={11} />
            Previous
          </Box>
          <Typography sx={{fontSize: '14px', fontWeight: 600, color: 'text.primary', lineHeight: 1.35, textWrap: 'pretty'}}>
            {prev.title}
          </Typography>
        </Box>
      ) : (
        <Box />
      )}
      {next && (
        <Box
          component={Link}
          to={next.permalink}
          sx={{
            p: 2.25,
            border: '1px solid',
            borderColor: isLight ? 'rgba(0,0,0,0.07)' : 'rgba(255,255,255,0.07)',
            borderRadius: '12px',
            bgcolor: isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.02)',
            textAlign: {sm: 'right'},
            transition: 'all 0.2s ease',
            textDecoration: 'none',
            '&:hover': {
              borderColor: 'rgba(54,136,255,0.35)',
              bgcolor: 'rgba(54,136,255,0.04)',
              textDecoration: 'none',
            },
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
              color: isLight ? 'rgba(0,0,0,0.6)' : 'rgba(255,255,255,0.65)',
              mb: 1,
            }}
          >
            Next
            <ArrowRight size={11} />
          </Box>
          <Typography sx={{fontSize: '14px', fontWeight: 600, color: 'text.primary', lineHeight: 1.35, textWrap: 'pretty'}}>
            {next.title}
          </Typography>
        </Box>
      )}
    </Box>
  );
}
