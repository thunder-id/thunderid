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

import {Box, Typography, useTheme} from '@wso2/oxygen-ui';
import {Search} from '@wso2/oxygen-ui-icons-react';
import {ChangeEvent, JSX, KeyboardEvent} from 'react';
import {EcosystemCategory, FILTER_TABS} from './data';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import FloatingLogosBackground from '../FloatingLogosBackground';
import ProductName from '@site/src/components/ProductName';

interface EcosystemHeroProps {
  query: string;
  onQueryChange: (value: string) => void;
  category: 'all' | EcosystemCategory;
  onCategoryChange: (value: 'all' | EcosystemCategory) => void;
}

export default function EcosystemHero({query, onQueryChange, category, onCategoryChange}: EcosystemHeroProps): JSX.Element {
  const theme = useTheme();
  const isLight = !useIsDarkMode();

  return (
    <Box sx={{position: 'relative', pt: {xs: 6, md: 8}, pb: 2, textAlign: 'center', overflow: 'hidden'}}>
      <Box sx={{position: 'absolute', inset: 0, height: 280, zIndex: 0}}>
        <FloatingLogosBackground />
      </Box>

      <Box sx={{position: 'relative', zIndex: 1, maxWidth: 780, mx: 'auto', px: 2}}>
        <Box
          sx={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: 1,
            mb: 2.5,
            fontFamily: 'monospace',
            fontSize: '10.5px',
            fontWeight: 600,
            letterSpacing: '0.18em',
            textTransform: 'uppercase',
            color: '#8bf9fa',
          }}
        >
          <Box component="span" sx={{width: 5, height: 5, borderRadius: '50%', bgcolor: '#8bf9fa', boxShadow: '0 0 10px #8bf9fa'}} />
          SDKs &amp; Tools
        </Box>

        <Typography
          variant="h1"
          sx={{
            fontSize: {xs: '2.25rem', sm: '2.75rem', md: '3.5rem'},
            fontWeight: 700,
            letterSpacing: '-0.04em',
            lineHeight: 1.04,
            color: 'text.primary',
            mb: 2.5,
          }}
        >
          Build <ProductName /> into
          <br />
          any stack
        </Typography>

        <Typography
          sx={{
            fontSize: '16.5px',
            lineHeight: 1.65,
            color: 'text.secondary',
            maxWidth: 560,
            mx: 'auto',
            mb: 4.5,
          }}
        >
          Official SDKs, framework integrations, and agent tooling for seamless authentication from the browser to
          the edge to the server.
        </Typography>

        <Box sx={{position: 'relative', maxWidth: 560, mx: 'auto', mb: 3}}>
          <Box
            sx={{
              position: 'absolute',
              left: 18,
              top: '50%',
              display: 'inline-flex',
              transform: 'translateY(-50%)',
              color: isLight ? 'rgba(0,0,0,0.35)' : 'rgba(255,255,255,0.35)',
              pointerEvents: 'none',
            }}
          >
            <Search size={18} />
          </Box>
          <Box
            component="input"
            value={query}
            aria-label="Search SDKs, packages, and frameworks"
            onChange={(e: ChangeEvent<HTMLInputElement>) => onQueryChange(e.target.value)}
            placeholder="Search SDKs, packages, frameworks…"
            sx={{
              width: '100%',
              height: 50,
              pl: '48px',
              pr: '18px',
              fontSize: '14.5px',
              fontFamily: 'inherit',
              color: 'text.primary',
              bgcolor: isLight ? 'rgba(0,0,0,0.03)' : 'rgba(255,255,255,0.04)',
              border: '1px solid',
              borderColor: isLight ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.1)',
              borderRadius: '12px',
              outline: 'none',
              transition: 'all 0.15s ease',
              '&:focus': {
                borderColor: 'rgba(54,136,255,0.5)',
                bgcolor: 'rgba(54,136,255,0.05)',
              },
              '&::placeholder': {
                color: isLight ? 'rgba(0,0,0,0.35)' : 'rgba(255,255,255,0.35)',
              },
            }}
          />
        </Box>

        <Box role="tablist" sx={{display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: 0.75}}>
          {FILTER_TABS.map((tab) => {
            const isActive = category === tab.key;
            return (
              <Box
                key={tab.key}
                role="tab"
                aria-selected={isActive}
                tabIndex={0}
                onClick={() => onCategoryChange(tab.key)}
                onKeyDown={(e: KeyboardEvent<HTMLDivElement>) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    onCategoryChange(tab.key);
                  }
                }}
                sx={{
                  px: 1.75,
                  py: 0.625,
                  borderRadius: '999px',
                  fontSize: '12.5px',
                  fontWeight: 500,
                  border: '1px solid',
                  cursor: 'pointer',
                  userSelect: 'none',
                  transition: 'all 0.15s ease',
                  borderColor: isActive ? 'rgba(54,136,255,0.45)' : isLight ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.1)',
                  bgcolor: isActive ? 'rgba(54,136,255,0.1)' : 'transparent',
                  color: isActive ? theme.vars?.palette.primary.main : isLight ? 'rgba(0,0,0,0.5)' : 'rgba(255,255,255,0.5)',
                  '&:hover': {
                    borderColor: isActive ? 'rgba(54,136,255,0.55)' : isLight ? 'rgba(0,0,0,0.2)' : 'rgba(255,255,255,0.2)',
                  },
                }}
              >
                {tab.label}
              </Box>
            );
          })}
        </Box>
      </Box>
    </Box>
  );
}
