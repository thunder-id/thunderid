// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {Box, Typography} from '@wso2/oxygen-ui';
import {Check, Users} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import EntryIcon from './Detail/EntryIcon';
import {
  artifactOf,
  CATEGORY_LABELS,
  entryHref,
  isAvailable,
  originOf,
  packageLabel,
  showsCategory,
  versionLabel,
} from './presentation';
import useEntryVersion from './useEntryVersion';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import {useDocsUrl} from '@site/src/hooks/useDocsUrl';
import type {EcosystemEntry} from '@site/src/types/ecosystem';

function OriginBadge({entry, isLight}: {entry: EcosystemEntry; isLight: boolean}): JSX.Element {
  const official = originOf(entry) === 'official';

  return (
    <Box
      component="span"
      sx={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 0.625,
        flexShrink: 0,
        fontSize: '9px',
        fontWeight: 700,
        letterSpacing: '0.1em',
        textTransform: 'uppercase',
        borderRadius: '6px',
        whiteSpace: 'nowrap',
        ...(official
          ? {
              px: '9px',
              py: '4px',
              color: '#08121f',
              background: 'linear-gradient(135deg, #8bf9fa 0%, #4b9bff 100%)',
            }
          : {
              px: '8px',
              py: '3px',
              color: isLight ? '#b45309' : '#fbbf24',
              bgcolor: 'rgba(249,115,22,0.1)',
              border: '1px solid rgba(251,191,36,0.32)',
            }),
      }}
    >
      {official ? <Check size={10} strokeWidth={3.2} /> : <Users size={10} strokeWidth={2.4} />}
      {official ? 'Official' : 'Community'}
    </Box>
  );
}

export default function EcosystemCard({entry}: {entry: EcosystemEntry}): JSX.Element {
  const isLight = !useIsDarkMode();
  const docsUrl = useDocsUrl();
  const version = useEntryVersion(entry);
  const available = isAvailable(entry);
  const versionText = versionLabel(entry, version);
  const href = entryHref(entry);

  const muted = (light: number, dark: number): string =>
    isLight ? `rgba(0,0,0,${light})` : `rgba(255,255,255,${dark})`;

  const content = (
    <Box
      sx={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        gap: 1.75,
        borderRadius: '14px',
        border: '1px solid',
        borderColor: available ? muted(0.08, 0.07) : muted(0.05, 0.05),
        bgcolor: available ? muted(0.02, 0.02) : muted(0.012, 0.012),
        filter: available ? 'none' : 'saturate(0)',
        p: '22px',
        transition: 'border-color 0.18s, background-color 0.18s, transform 0.18s',
        ...(available && href
          ? {
              cursor: 'pointer',
              '&:hover': {
                borderColor: 'rgba(54,136,255,0.4)',
                bgcolor: 'rgba(54,136,255,0.04)',
                transform: 'translateY(-2px)',
              },
            }
          : {'&:hover': {borderColor: muted(0.12, 0.12)}}),
      }}
    >
      <Box sx={{display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 1.5}}>
        <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5, minWidth: 0}}>
          <Box
            sx={{
              width: 46,
              height: 46,
              flexShrink: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              borderRadius: '12px',
              bgcolor: muted(0.03, 0.04),
              border: '1px solid',
              borderColor: muted(0.06, 0.07),
              opacity: available ? 1 : 0.75,
            }}
          >
            <EntryIcon name={entry.icon} size={24} />
          </Box>
          <Box sx={{minWidth: 0}}>
            <Typography
              sx={{
                fontSize: '14.5px',
                fontWeight: 600,
                letterSpacing: '-0.01em',
                color: available ? 'text.primary' : muted(0.6, 0.82),
              }}
            >
              {entry.name}
            </Typography>
            <Typography
              sx={{
                fontFamily: 'monospace',
                fontSize: '11px',
                mt: '3px',
                color: muted(0.4, 0.4),
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {packageLabel(entry)}
            </Typography>
          </Box>
        </Box>

        <Box sx={{display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: 0.875, flexShrink: 0}}>
          <OriginBadge entry={entry} isLight={isLight} />
          {versionText && (
            <Typography
              component="span"
              sx={{fontFamily: 'monospace', fontSize: '10px', color: muted(0.5, 0.55), whiteSpace: 'nowrap'}}
            >
              {versionText}
            </Typography>
          )}
        </Box>
      </Box>

      <Typography sx={{fontSize: '13px', lineHeight: 1.62, color: muted(0.5, 0.55), flex: 1}}>
        {entry.description}
      </Typography>

      <Box sx={{display: 'flex', alignItems: 'center', gap: 1, minWidth: 0}}>
        <Typography
          component="span"
          sx={{
            fontFamily: 'monospace',
            fontSize: '9.5px',
            letterSpacing: '0.1em',
            textTransform: 'uppercase',
            color: muted(0.5, 0.5),
            whiteSpace: 'nowrap',
          }}
        >
          {artifactOf(entry)}
        </Typography>
        {showsCategory(entry) && (
          <Typography
            component="span"
            sx={{
              fontFamily: 'monospace',
              fontSize: '9.5px',
              letterSpacing: '0.1em',
              textTransform: 'uppercase',
              color: muted(0.4, 0.4),
              whiteSpace: 'nowrap',
            }}
          >
            {CATEGORY_LABELS[entry.category]}
          </Typography>
        )}
        {entry.author && (
          <Typography
            component="span"
            sx={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 0.5,
              fontSize: '10.5px',
              color: muted(0.32, 0.32),
              minWidth: 0,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            <Users size={10} strokeWidth={2.2} />@{entry.author}
          </Typography>
        )}
      </Box>
    </Box>
  );

  // A `soon` entry has nowhere to go, and a released one without a docs page
  // yet renders as a plain card rather than a dead link.
  if (!available || !href) return content;

  return (
    <Box component={Link} to={docsUrl(href)} sx={{textDecoration: 'none', display: 'block', height: '100%'}}>
      {content}
    </Box>
  );
}
