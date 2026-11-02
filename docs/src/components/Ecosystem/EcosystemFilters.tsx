// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Typography} from '@wso2/oxygen-ui';
import {Check, SlidersHorizontal} from '@wso2/oxygen-ui-icons-react';
import {JSX} from 'react';
import {buildFilterPanel, EcosystemFacetGroup, EcosystemFilters as Filters} from './presentation';
import useIsDarkMode from '../../hooks/useIsDarkMode';
import type {EcosystemEntry} from '@site/src/types/ecosystem';

interface FacetRowProps {
  label: string;
  count: number;
  selected: boolean;
  onToggle: () => void;
  isLight: boolean;
}

function FacetRow({label, count, selected, onToggle, isLight}: FacetRowProps): JSX.Element {
  const ink = (light: number, dark: number): string => (isLight ? `rgba(0,0,0,${light})` : `rgba(255,255,255,${dark})`);

  return (
    <Box
      component="button"
      type="button"
      role="checkbox"
      aria-checked={selected}
      onClick={onToggle}
      sx={{
        width: '100%',
        display: 'flex',
        alignItems: 'center',
        gap: 1.25,
        px: 1,
        py: 0.75,
        border: 'none',
        borderRadius: '7px',
        cursor: 'pointer',
        textAlign: 'left',
        fontFamily: 'inherit',
        transition: 'background-color 0.15s, color 0.15s',
        color: selected ? 'text.primary' : ink(0.65, 0.65),
        bgcolor: selected ? 'rgba(54,136,255,0.1)' : 'transparent',
        '&:hover': {bgcolor: selected ? 'rgba(54,136,255,0.14)' : ink(0.03, 0.04)},
      }}
    >
      <Box
        component="span"
        aria-hidden
        sx={{
          width: 15,
          height: 15,
          flexShrink: 0,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderRadius: '4px',
          border: '1px solid',
          borderColor: selected ? '#8bf9fa' : ink(0.2, 0.2),
          background: selected ? 'linear-gradient(135deg, #8bf9fa 0%, #4b9bff 100%)' : 'transparent',
          transition: 'all 0.15s',
        }}
      >
        {selected && <Check size={10} strokeWidth={3.6} color="#08121f" />}
      </Box>
      <Typography component="span" sx={{flex: 1, minWidth: 0, fontSize: '13px'}}>
        {label}
      </Typography>
      <Typography
        component="span"
        sx={{fontFamily: 'monospace', fontSize: '11px', color: ink(0.35, 0.35), flexShrink: 0}}
      >
        {count}
      </Typography>
    </Box>
  );
}

function FacetGroup({title, children, isLight}: {title: string; children: JSX.Element[]; isLight: boolean}): JSX.Element {
  return (
    <Box component="fieldset" sx={{border: 0, p: 0, m: 0, mb: 3}}>
      <Typography
        component="div"
        sx={{
          fontFamily: 'monospace',
          fontSize: '9.5px',
          letterSpacing: '0.14em',
          textTransform: 'uppercase',
          color: isLight ? 'rgba(0,0,0,0.4)' : 'rgba(255,255,255,0.3)',
          px: 1,
          mb: 1,
        }}
      >
        {title}
      </Typography>
      {children}
    </Box>
  );
}

interface EcosystemFiltersProps {
  entries: EcosystemEntry[];
  filters: Filters;
  onToggle: (group: EcosystemFacetGroup, key: string) => void;
  onClearAll: () => void;
  hasActive: boolean;
}

/**
 * Faceted filter panel.
 *
 * Each group's counts are computed with that group's own selections ignored, so
 * ticking one box does not drive its siblings to zero and the reader can always
 * see what adding a second option would bring in.
 */
export default function EcosystemFilters({
  entries,
  filters,
  onToggle,
  onClearAll,
  hasActive,
}: EcosystemFiltersProps): JSX.Element {
  const isLight = !useIsDarkMode();
  const panel = buildFilterPanel(entries, filters);
  const ink = (light: number, dark: number): string => (isLight ? `rgba(0,0,0,${light})` : `rgba(255,255,255,${dark})`);

  // A group offering a single option can only be a no-op or hide everything, so
  // it is dropped rather than shown as a box that does nothing.
  const sections = (
    [
      {group: 'groups', title: 'Type', facets: panel.groups},
      {group: 'categories', title: 'Platform', facets: panel.categories},
      {group: 'availability', title: 'Status', facets: panel.availability},
    ] satisfies {group: EcosystemFacetGroup; title: string; facets: {key: string; label: string; count: number; selected: boolean}[]}[]
  ).filter((section) => section.facets.length > 1);

  return (
    <Box
      component="aside"
      aria-label="Filters"
      sx={{
        position: {md: 'sticky'},
        top: {md: 88},
        alignSelf: 'start',
        minWidth: 0,
      }}
    >
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 1,
          px: 1,
          pb: 1.5,
          mb: 2,
          borderBottom: '1px solid',
          borderColor: ink(0.08, 0.07),
        }}
      >
        <Box sx={{display: 'inline-flex', alignItems: 'center', gap: 1, color: 'text.primary'}}>
          <SlidersHorizontal size={13} strokeWidth={2} />
          <Typography component="span" sx={{fontSize: '13px', fontWeight: 600}}>
            Filters
          </Typography>
        </Box>
        {hasActive && (
          <Box
            component="button"
            type="button"
            onClick={onClearAll}
            sx={{
              border: 'none',
              bgcolor: 'transparent',
              p: 0,
              cursor: 'pointer',
              fontFamily: 'inherit',
              fontSize: '11.5px',
              color: ink(0.45, 0.45),
              transition: 'color 0.15s',
              '&:hover': {color: 'text.primary'},
            }}
          >
            Clear all
          </Box>
        )}
      </Box>

      {sections.map((section) => (
        <FacetGroup key={section.group} title={section.title} isLight={isLight}>
          {section.facets.map((facet) => (
            <FacetRow
              key={facet.key}
              label={facet.label}
              count={facet.count}
              selected={facet.selected}
              onToggle={() => onToggle(section.group, facet.key)}
              isLight={isLight}
            />
          ))}
        </FacetGroup>
      ))}
    </Box>
  );
}
