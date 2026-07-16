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
import {Box, Typography, useTheme} from '@wso2/oxygen-ui';
import {ArrowRight} from '@wso2/oxygen-ui-icons-react';
import {JSX, useEffect, useState} from 'react';
import {CATEGORY_LABELS, EcosystemItem} from './data';
import useIsDarkMode from '../../hooks/useIsDarkMode';

interface VersionChipProps {
  item: EcosystemItem;
  isLight: boolean;
}

function VersionChip({item, isLight}: VersionChipProps): JSX.Element | null {
  const [fetchedVersion, setFetchedVersion] = useState('');

  useEffect(() => {
    if (item.packageManager === 'npm' && !item.soon) {
      fetch(`https://registry.npmjs.org/${item.packageName}/latest`)
        .then((res) => res.json())
        .then((data: {version?: string}) => {
          if (data.version) setFetchedVersion(`v${data.version}`);
        })
        .catch(() => {
          // Silently fail if version fetch fails.
        });
    }
  }, [item.packageManager, item.packageName, item.soon]);

  const baseSx = {
    fontFamily: 'monospace',
    fontSize: '9.5px',
    fontWeight: 600,
    textTransform: 'uppercase' as const,
    letterSpacing: '0.04em',
    borderRadius: '6px',
    px: 1,
    py: 0.4,
    border: '1px solid',
    flexShrink: 0,
    whiteSpace: 'nowrap' as const,
  };

  if (item.category === 'integration') {
    return (
      <Box
        component="span"
        sx={{
          ...baseSx,
          color: isLight ? 'rgba(0,0,0,0.55)' : 'rgba(255,255,255,0.55)',
          bgcolor: isLight ? 'rgba(0,0,0,0.04)' : 'rgba(255,255,255,0.05)',
          borderColor: isLight ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.12)',
        }}
      >
        Built-in
      </Box>
    );
  }

  if (item.category === 'agent') {
    return (
      <Box
        component="span"
        sx={{...baseSx, color: '#3688ff', bgcolor: 'rgba(139,249,250,0.1)', borderColor: 'rgba(139,249,250,0.3)'}}
      >
        Beta
      </Box>
    );
  }

  if (!fetchedVersion) return null;

  return (
    <Box
      component="span"
      sx={{...baseSx, textTransform: 'none', color: '#4ade80', bgcolor: 'rgba(74,222,128,0.1)', borderColor: 'rgba(74,222,128,0.22)'}}
    >
      {fetchedVersion}
    </Box>
  );
}

export default function EcosystemCard({item}: {item: EcosystemItem}): JSX.Element {
  const theme = useTheme();
  const isLight = !useIsDarkMode();
  const Icon = item.icon;

  const content = (
    <Box
      sx={{
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        gap: 1.5,
        borderRadius: '14px',
        border: '1px solid',
        borderColor: item.soon
          ? isLight
            ? 'rgba(0,0,0,0.06)'
            : 'rgba(255,255,255,0.05)'
          : isLight
            ? 'rgba(0,0,0,0.08)'
            : 'rgba(255,255,255,0.07)',
        bgcolor: item.soon ? (isLight ? 'rgba(0,0,0,0.012)' : 'rgba(255,255,255,0.012)') : isLight ? 'rgba(0,0,0,0.02)' : 'rgba(255,255,255,0.02)',
        filter: item.soon ? 'saturate(0)' : 'none',
        opacity: item.soon ? 0.55 : 1,
        p: '22px',
        transition: 'all 0.2s ease',
        cursor: item.soon ? 'default' : 'pointer',
        '&:hover': item.soon
          ? {opacity: 0.9}
          : {
              borderColor: theme.vars?.palette.primary.main,
              bgcolor: 'rgba(54,136,255,0.04)',
              transform: 'translateY(-2px)',
            },
      }}
    >
      <Box sx={{display: 'flex', alignItems: 'flex-start', gap: 1.5}}>
        <Box
          sx={{
            width: 46,
            height: 46,
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            borderRadius: '12px',
            bgcolor: isLight ? 'rgba(0,0,0,0.03)' : 'rgba(255,255,255,0.04)',
            border: '1px solid',
            borderColor: isLight ? 'rgba(0,0,0,0.06)' : 'rgba(255,255,255,0.07)',
          }}
        >
          <Icon size={24} />
        </Box>
        <Box sx={{flex: 1, minWidth: 0}}>
          <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1}}>
            <Typography sx={{fontSize: '14.5px', fontWeight: 600, color: 'text.primary'}}>{item.name}</Typography>
            {item.soon ? (
              <Box
                component="span"
                sx={{
                  fontFamily: 'monospace',
                  fontSize: '9px',
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.04em',
                  borderRadius: '6px',
                  px: 1,
                  py: 0.4,
                  border: '1px solid',
                  borderColor: isLight ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.1)',
                  color: isLight ? 'rgba(0,0,0,0.45)' : 'rgba(255,255,255,0.45)',
                  bgcolor: isLight ? 'rgba(0,0,0,0.04)' : 'rgba(255,255,255,0.05)',
                  flexShrink: 0,
                }}
              >
                Soon
              </Box>
            ) : (
              <VersionChip item={item} isLight={isLight} />
            )}
          </Box>
          <Typography
            sx={{
              fontFamily: 'monospace',
              fontSize: '11px',
              color: isLight ? 'rgba(0,0,0,0.4)' : 'rgba(255,255,255,0.4)',
            }}
          >
            {item.packageName}
          </Typography>
        </Box>
      </Box>

      <Typography sx={{fontSize: '13px', lineHeight: 1.62, color: isLight ? 'rgba(0,0,0,0.5)' : 'rgba(255,255,255,0.5)', flex: 1}}>
        {item.description}
      </Typography>

      <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', mt: 0.5}}>
        <Typography
          component="span"
          sx={{
            fontFamily: 'monospace',
            fontSize: '9.5px',
            textTransform: 'uppercase',
            letterSpacing: '0.1em',
            color: isLight ? 'rgba(0,0,0,0.4)' : 'rgba(255,255,255,0.4)',
          }}
        >
          {CATEGORY_LABELS[item.category]}
        </Typography>
        {!item.soon && (
          <Box
            component="span"
            sx={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: 0.5,
              fontSize: '12px',
              fontWeight: 500,
              color: theme.vars?.palette.primary.main,
            }}
          >
            {item.ctaLabel}
            <ArrowRight size={12} strokeWidth={2.4} />
          </Box>
        )}
      </Box>
    </Box>
  );

  if (item.soon || !item.href) {
    return content;
  }

  const isExternal = item.href.startsWith('http');

  return (
    <Box
      component={Link}
      to={item.href}
      target={isExternal ? '_blank' : undefined}
      rel={isExternal ? 'noopener noreferrer' : undefined}
      sx={{textDecoration: 'none', display: 'block', height: '100%'}}
    >
      {content}
    </Box>
  );
}
