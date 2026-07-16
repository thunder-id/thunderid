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
import {Search} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import {EcosystemItem} from './data';
import EcosystemCard from './EcosystemCard';
import useIsDarkMode from '../../hooks/useIsDarkMode';

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
  items: EcosystemItem[];
}

export default function EcosystemGrid({query, items}: EcosystemGridProps): JSX.Element {
  const isLight = !useIsDarkMode();
  const availableItems = items.filter((item) => !item.soon);
  const comingItems = items.filter((item) => item.soon);
  const noResults = items.length === 0;

  const gridSx = {
    display: 'grid',
    gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))',
    gap: 2,
  };

  return (
    <Box sx={{maxWidth: 1200, mx: 'auto', px: {xs: 2, sm: 4}, py: {xs: 5, md: 7}}}>
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
            No results for &ldquo;{query}&rdquo;
          </Typography>
          <Typography sx={{fontSize: '13.5px', color: 'text.secondary'}}>
            Try a different term, or request one below.
          </Typography>
        </Box>
      ) : (
        <>
          {availableItems.length > 0 && (
            <Box sx={{mb: comingItems.length > 0 ? 5 : 0}}>
              <SectionHeader label="Available now" count={availableItems.length} isLight={isLight} />
              <Box sx={gridSx}>
                {availableItems.map((item) => (
                  <EcosystemCard key={item.id} item={item} />
                ))}
              </Box>
            </Box>
          )}
          {comingItems.length > 0 && (
            <Box>
              <SectionHeader label="Coming soon" count={comingItems.length} isLight={isLight} />
              <Box sx={gridSx}>
                {comingItems.map((item) => (
                  <EcosystemCard key={item.id} item={item} />
                ))}
              </Box>
            </Box>
          )}
        </>
      )}
    </Box>
  );
}
