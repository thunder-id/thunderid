// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {usePagedOrganizationUnits} from '@thunderid/configure-organization-units';
import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {
  alpha,
  Box,
  Button,
  Checkbox,
  Chip,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Typography,
} from '@wso2/oxygen-ui';
import {ChevronRight, Search, X} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ChildUnitReach} from '../models/organization-access';

/**
 * Where the dialog lists organization units from.
 *
 * - `children`: the direct children of one unit, the only units an owner's policy may name.
 * - `roots`: the top unit of every tree.
 * - `allTrees`: every unit of every tree, browsed level by level.
 */
export type OrganizationUnitSource = {type: 'children'; parentId: string} | {type: 'roots'} | {type: 'allTrees'};

export interface OrganizationUnitSelection {
  ouId: string;
  /**
   * Set only when the dialog asks for each unit's reach.
   */
  reach?: ChildUnitReach;
}

export interface OrganizationUnitSelectDialogProps {
  open: boolean;
  title: string;
  description: string;
  confirmLabel: string;
  source: OrganizationUnitSource;
  /**
   * Units already chosen, shown checked and locked with `lockedLabel`.
   */
  lockedIds?: string[];
  lockedLabel?: string;
  /**
   * Units not listed at all, such as the owner among the root units.
   */
  hiddenIds?: string[];
  /**
   * Asks how far below each picked unit access reaches.
   */
  withReach?: boolean;
  /**
   * `exclude` marks picked units as carved out rather than granted.
   */
  tone?: 'grant' | 'exclude';
  onClose: () => void;
  onConfirm: (selection: OrganizationUnitSelection[]) => void;
}

interface UnitLevelProps {
  parentId: string | null;
  depth: number;
  expandable: boolean;
  query: string;
  selected: Map<string, OrganizationUnitSelection>;
  lockedIds: string[];
  lockedLabel: string;
  hiddenIds: string[];
  includedBy: string | null;
  tone: 'grant' | 'exclude';
  onToggle: (unit: OrganizationUnit) => void;
}

function UnitLevel({
  parentId,
  depth,
  expandable,
  query,
  selected,
  lockedIds,
  lockedLabel,
  hiddenIds,
  includedBy,
  tone,
  onToggle,
}: UnitLevelProps): JSX.Element {
  const {t} = useTranslation();
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const {data, error, isLoading, refetch, hasNextPage, fetchNextPage, isFetchingNextPage} =
    usePagedOrganizationUnits(parentId);

  const tForErrors = (key: string, options?: Record<string, unknown>): string =>
    t(key.includes(':') ? key : `resourceSharing:${key}`, options);

  const units = (data?.pages ?? [])
    .flatMap((page) => page.organizationUnits)
    .filter((unit) => !hiddenIds.includes(unit.id));
  const needle = query.trim().toLowerCase();
  const visible = needle
    ? units.filter((unit) => unit.name.toLowerCase().includes(needle) || unit.handle.toLowerCase().includes(needle))
    : units;

  if (isLoading) {
    return (
      <Box sx={{py: 2, pl: depth * 3, display: 'flex', justifyContent: depth ? 'flex-start' : 'center'}}>
        <CircularProgress size={20} />
      </Box>
    );
  }

  if (error) {
    return (
      <Box sx={{pl: depth * 3, py: 1}}>
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="inline"
          fallbackKey="picker.loadError"
          fallbackDefaultValue="Failed to load organization units."
          onRetry={() => void refetch()}
        />
      </Box>
    );
  }

  if (visible.length === 0 && depth === 0) {
    return (
      <Typography variant="body2" color="text.secondary" sx={{py: 2, textAlign: 'center'}}>
        {t('resourceSharing:picker.empty', 'No organization units to show.')}
      </Typography>
    );
  }

  return (
    <Box component="ul" sx={{listStyle: 'none', m: 0, p: 0}}>
      {visible.map((unit) => {
        const locked = lockedIds.includes(unit.id);
        const isSelected = selected.has(unit.id);
        const isOpen = expanded.has(unit.id);
        let tag: string | null = null;
        if (locked) tag = lockedLabel;
        else if (includedBy) {
          tag = t('resourceSharing:picker.includedWith', 'Included with {{name}}', {name: includedBy});
        }

        return (
          <Box component="li" key={unit.id}>
            <Stack
              direction="row"
              alignItems="center"
              spacing={0.5}
              sx={{
                pl: depth * 3,
                pr: 1,
                borderRadius: '8px',
                bgcolor: (theme) => {
                  if (!isSelected) return 'transparent';
                  return tone === 'exclude' ? alpha(theme.palette.error.main, 0.08) : theme.palette.action.selected;
                },
              }}
            >
              {expandable ? (
                <IconButton
                  size="small"
                  aria-expanded={isOpen}
                  aria-label={
                    isOpen
                      ? t('resourceSharing:picker.collapse', 'Hide units under {{name}}', {name: unit.name})
                      : t('resourceSharing:picker.expand', 'Show units under {{name}}', {name: unit.name})
                  }
                  onClick={() =>
                    setExpanded((current) => {
                      const next = new Set(current);
                      if (next.has(unit.id)) next.delete(unit.id);
                      else next.add(unit.id);
                      return next;
                    })
                  }
                >
                  <ChevronRight size={14} style={{transform: isOpen ? 'rotate(90deg)' : 'none'}} />
                </IconButton>
              ) : null}
              <Box
                component="label"
                sx={{display: 'flex', alignItems: 'center', gap: 1, flex: 1, minWidth: 0, py: 0.5, cursor: 'pointer'}}
              >
                <Checkbox
                  size="small"
                  color={tone === 'exclude' ? 'error' : 'primary'}
                  checked={locked || isSelected || Boolean(includedBy)}
                  disabled={locked || Boolean(includedBy)}
                  onChange={() => onToggle(unit)}
                />
                <Box sx={{minWidth: 0}}>
                  <Typography variant="body2" fontWeight={600} noWrap>
                    {unit.name}
                  </Typography>
                  <Typography variant="caption" color="text.secondary" sx={{fontFamily: 'monospace'}} noWrap>
                    {unit.handle}
                  </Typography>
                </Box>
              </Box>
              {tag && <Chip size="small" label={tag} />}
            </Stack>
            {expandable && isOpen && (
              <UnitLevel
                parentId={unit.id}
                depth={depth + 1}
                expandable
                query=""
                selected={selected}
                lockedIds={lockedIds}
                lockedLabel={lockedLabel}
                hiddenIds={hiddenIds}
                includedBy={includedBy ?? (isSelected || locked ? unit.name : null)}
                tone={tone}
                onToggle={onToggle}
              />
            )}
          </Box>
        );
      })}
      {hasNextPage && (
        <Box component="li" sx={{pl: depth * 3, py: 0.5}}>
          <Button size="small" disabled={isFetchingNextPage} onClick={() => void fetchNextPage()}>
            {t('resourceSharing:picker.loadMore', 'Load more')}
          </Button>
        </Box>
      )}
    </Box>
  );
}

/**
 * Picks several organization units from one source, for granting access or for carving units out
 * of a broader grant. Picked units, and their reach where asked, are listed beside the source.
 */
export default function OrganizationUnitSelectDialog({
  open,
  title,
  description,
  confirmLabel,
  source,
  lockedIds = [],
  lockedLabel = '',
  hiddenIds = [],
  withReach = false,
  tone = 'grant',
  onClose,
  onConfirm,
}: OrganizationUnitSelectDialogProps): JSX.Element {
  const {t} = useTranslation();
  const [query, setQuery] = useState('');
  const [selected, setSelected] = useState<Map<string, OrganizationUnitSelection>>(new Map());
  const [names, setNames] = useState<Map<string, string>>(new Map());

  const toggle = (unit: OrganizationUnit): void => {
    setSelected((current) => {
      const next = new Map(current);
      if (next.has(unit.id)) next.delete(unit.id);
      else next.set(unit.id, withReach ? {ouId: unit.id, reach: 'subtree'} : {ouId: unit.id});
      return next;
    });
    setNames((current) => new Map(current).set(unit.id, unit.name));
  };

  const setReach = (ouId: string, reach: ChildUnitReach): void => {
    setSelected((current) => new Map(current).set(ouId, {ouId, reach}));
  };

  const picked = useMemo(() => [...selected.values()], [selected]);
  const parentId = source.type === 'children' ? source.parentId : null;

  return (
    <Dialog open={open} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>{description}</DialogContentText>
        <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', md: '3fr 2fr'}, gap: 2}}>
          <Stack spacing={1.5} sx={{minWidth: 0}}>
            <TextField
              size="small"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder={t('resourceSharing:picker.search', 'Search organization units')}
              slotProps={{
                htmlInput: {'aria-label': t('resourceSharing:picker.search', 'Search organization units')},
                input: {
                  startAdornment: (
                    <InputAdornment position="start">
                      <Search size={16} />
                    </InputAdornment>
                  ),
                },
              }}
            />
            <Box sx={{maxHeight: 420, overflowY: 'auto'}}>
              <UnitLevel
                parentId={parentId}
                depth={0}
                expandable={source.type === 'allTrees'}
                query={query}
                selected={selected}
                lockedIds={lockedIds}
                lockedLabel={lockedLabel}
                hiddenIds={hiddenIds}
                includedBy={null}
                tone={tone}
                onToggle={toggle}
              />
            </Box>
          </Stack>
          <Stack spacing={1} sx={{minWidth: 0, bgcolor: 'action.hover', borderRadius: '10px', p: 2}}>
            <Typography variant="subtitle2" color="text.secondary">
              {tone === 'exclude'
                ? t('resourceSharing:picker.toExclude', 'To exclude')
                : t('resourceSharing:picker.toGrant', 'To grant')}
            </Typography>
            {picked.length === 0 && (
              <Typography variant="body2" color="text.secondary">
                {t('resourceSharing:picker.noneSelected', 'Nothing selected yet.')}
              </Typography>
            )}
            {picked.map((item) => {
              const name = names.get(item.ouId) ?? item.ouId;

              return (
                <Stack
                  key={item.ouId}
                  spacing={1}
                  sx={{
                    bgcolor: 'background.paper',
                    border: '1px solid',
                    borderColor: 'divider',
                    borderRadius: '8px',
                    p: 1.5,
                  }}
                >
                  <Stack direction="row" alignItems="center" justifyContent="space-between" spacing={1}>
                    <Typography variant="body2" fontWeight={600} noWrap>
                      {name}
                    </Typography>
                    <IconButton
                      size="small"
                      aria-label={t('resourceSharing:picker.unselect', 'Remove {{name}}', {name})}
                      onClick={() =>
                        setSelected((current) => {
                          const next = new Map(current);
                          next.delete(item.ouId);
                          return next;
                        })
                      }
                    >
                      <X size={14} />
                    </IconButton>
                  </Stack>
                  {item.reach && (
                    <ToggleButtonGroup
                      exclusive
                      size="small"
                      fullWidth
                      value={item.reach}
                      aria-label={t('resourceSharing:reach.label', 'Reach for {{name}}', {name})}
                      onChange={(_event, reach: ChildUnitReach | null) => {
                        if (reach) setReach(item.ouId, reach);
                      }}
                    >
                      <ToggleButton value="unit" sx={{textTransform: 'none'}}>
                        {t('resourceSharing:reach.unit', 'This unit only')}
                      </ToggleButton>
                      <ToggleButton value="subtree" sx={{textTransform: 'none'}}>
                        {t('resourceSharing:reach.subtree', 'And below')}
                      </ToggleButton>
                    </ToggleButtonGroup>
                  )}
                </Stack>
              );
            })}
          </Stack>
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('common:actions.cancel', 'Cancel')}</Button>
        <Button variant="contained" disabled={picked.length === 0} onClick={() => onConfirm(picked)}>
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
