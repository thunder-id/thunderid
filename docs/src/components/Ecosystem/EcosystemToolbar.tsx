// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {Search, X} from '@wso2/oxygen-ui-icons-react';
import {ChangeEvent, JSX} from 'react';
import {activeChips, EcosystemFacetGroup, EcosystemFilters as Filters} from './presentation';
import useIsDarkMode from '../../hooks/useIsDarkMode';

interface EcosystemToolbarProps {
  filters: Filters;
  onQueryChange: (value: string) => void;
  onToggle: (group: EcosystemFacetGroup, key: string) => void;
  resultCount: number;
  totalCount: number;
}

/**
 * Search box, result count, and the removable chips for whatever is selected in
 * the filter panel. Sits directly above the grids it describes.
 */
export default function EcosystemToolbar({
  filters,
  onQueryChange,
  onToggle,
  resultCount,
  totalCount,
}: EcosystemToolbarProps): JSX.Element {
  const isLight = !useIsDarkMode();
  const ink = (light: number, dark: number): string => (isLight ? `rgba(0,0,0,${light})` : `rgba(255,255,255,${dark})`);
  const chips = activeChips(filters);

  return (
    <Box sx={{mb: 3.5}}>
      <Box sx={{display: 'flex', alignItems: 'center', gap: 2, flexWrap: 'wrap'}}>
        <Box sx={{position: 'relative', flex: 1, minWidth: 240}}>
          <Box
            sx={{
              position: 'absolute',
              left: 16,
              top: '50%',
              transform: 'translateY(-50%)',
              display: 'inline-flex',
              color: ink(0.35, 0.35),
              pointerEvents: 'none',
            }}
          >
            <Search size={16} />
          </Box>
          <Box
            component="input"
            type="text"
            value={filters.query}
            aria-label="Search SDKs, packages, and frameworks"
            placeholder="Search SDKs, packages, frameworks…"
            onChange={(e: ChangeEvent<HTMLInputElement>) => onQueryChange(e.target.value)}
            sx={{
              width: '100%',
              height: 44,
              pl: '44px',
              pr: '16px',
              fontSize: '14px',
              fontFamily: 'inherit',
              color: 'text.primary',
              bgcolor: ink(0.03, 0.04),
              border: '1px solid',
              borderColor: ink(0.1, 0.1),
              borderRadius: '10px',
              outline: 'none',
              transition: 'all 0.18s',
              '&:focus': {borderColor: 'rgba(54,136,255,0.5)', bgcolor: 'rgba(54,136,255,0.05)'},
              '&::placeholder': {color: ink(0.35, 0.32)},
            }}
          />
        </Box>
        <Typography
          component="span"
          sx={{fontFamily: 'monospace', fontSize: '12px', color: ink(0.4, 0.4), whiteSpace: 'nowrap'}}
        >
          {resultCount} of {totalCount}
        </Typography>
      </Box>

      {chips.length > 0 && (
        <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 0.875, mt: 1.75}}>
          {chips.map((chip) => (
            <Box
              key={`${chip.group}:${chip.key}`}
              component="button"
              type="button"
              aria-label={`Remove filter ${chip.label}`}
              onClick={() => onToggle(chip.group, chip.key)}
              sx={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: 0.75,
                px: 1.5,
                py: 0.625,
                borderRadius: '999px',
                fontFamily: 'inherit',
                fontSize: '12px',
                cursor: 'pointer',
                border: '1px solid rgba(54,136,255,0.4)',
                bgcolor: 'rgba(54,136,255,0.1)',
                color: 'text.primary',
                transition: 'all 0.15s',
                '&:hover': {borderColor: 'rgba(54,136,255,0.65)'},
              }}
            >
              {chip.label}
              <X size={11} strokeWidth={2.6} />
            </Box>
          ))}
        </Box>
      )}
    </Box>
  );
}
