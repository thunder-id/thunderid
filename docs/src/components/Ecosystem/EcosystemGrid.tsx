// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {Search} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import EcosystemCard from './EcosystemCard';
import {isAvailable} from './presentation';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import type {EcosystemEntry} from '@site/src/types/ecosystem';

function SectionHeader({label, count, isLight}: {label: string; count: number; isLight: boolean}): JSX.Element {
  return (
    <Box sx={{display: 'flex', alignItems: 'center', gap: 1, mb: 2}}>
      <Typography component="h2" sx={{fontSize: '14px', fontWeight: 600, color: 'text.primary'}}>
        {label}
      </Typography>
      <Typography
        component="span"
        sx={{fontFamily: 'monospace', fontSize: '11px', color: isLight ? 'rgba(0,0,0,0.35)' : 'rgba(255,255,255,0.35)'}}
      >
        {String(count).padStart(2, '0')}
      </Typography>
    </Box>
  );
}

interface EcosystemGridProps {
  query: string;
  items: EcosystemEntry[];
}

export default function EcosystemGrid({query, items}: EcosystemGridProps): JSX.Element {
  const isLight = !useIsDarkMode();
  const availableItems = items.filter(isAvailable);
  const comingItems = items.filter((item) => !isAvailable(item));
  const noResults = items.length === 0;

  const gridSx = {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
    gap: 2,
  };

  return (
    <Box>
      {noResults ? (
        <Box sx={{textAlign: 'center', py: 10}}>
          <Box
            sx={{
              width: 56,
              height: 56,
              mx: 'auto',
              mb: 2,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              borderRadius: '14px',
              bgcolor: isLight ? 'rgba(0,0,0,0.04)' : 'rgba(255,255,255,0.04)',
            }}
          >
            <Search size={22} color={isLight ? 'rgba(0,0,0,0.3)' : 'rgba(255,255,255,0.3)'} />
          </Box>
          <Typography sx={{fontSize: '15px', fontWeight: 600, color: 'text.primary', mb: 0.5}}>
            {query.trim() ? <>No results for &ldquo;{query}&rdquo;</> : 'Nothing matches these filters'}
          </Typography>
          <Typography sx={{fontSize: '13.5px', color: 'text.secondary'}}>
            {query.trim() ? 'Try a different term, or request one below.' : 'Try clearing a filter, or request one below.'}
          </Typography>
        </Box>
      ) : (
        <>
          {availableItems.length > 0 && (
            <Box sx={{mb: comingItems.length > 0 ? 5 : 0}}>
              <SectionHeader label="Available now" count={availableItems.length} isLight={isLight} />
              <Box sx={gridSx}>
                {availableItems.map((item) => (
                  <EcosystemCard key={item.id} entry={item} />
                ))}
              </Box>
            </Box>
          )}
          {comingItems.length > 0 && (
            <Box>
              <SectionHeader label="Coming soon" count={comingItems.length} isLight={isLight} />
              <Box sx={gridSx}>
                {comingItems.map((item) => (
                  <EcosystemCard key={item.id} entry={item} />
                ))}
              </Box>
            </Box>
          )}
        </>
      )}
    </Box>
  );
}
