// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Checkbox, Chip, Collapse, FormControlLabel, Stack, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ChangeLines from './ChangeLines';
import type {Change, ChangeKind} from '../models/deploy';

export interface ChangeListProps {
  changes: Change[];
  /** The keys of the changes that will be applied. Without it, the changes are listed to be read. */
  selected?: Set<string>;
  onToggle?: (key: string) => void;
  onToggleAll?: (keys: string[], checked: boolean) => void;
  /** Where the changes are applied, to say a change held back there is held back on it. */
  target?: string;
  /** Where the changes come from, to say a change held back there is held back on it. */
  source?: string;
}

const COLORS: Readonly<Record<ChangeKind, 'success' | 'info' | 'error' | 'default'>> = {
  added: 'success',
  updated: 'info',
  deleted: 'error',
  unchanged: 'default',
};

/**
 * The resources an apply would write or delete, each with a checkbox to leave it alone this time when
 * the changes are selectable, and each opening to its document diffed line by line against what the
 * gateway runs. Resources the gateway already runs as the version has them are counted, not listed.
 */
export default function ChangeList({
  changes,
  selected = undefined,
  onToggle = undefined,
  onToggleAll = undefined,
  target = undefined,
  source = undefined,
}: ChangeListProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const [open, setOpen] = useState<ReadonlySet<string>>(new Set());
  const toggleOpen = (key: string): void =>
    setOpen((previous) => {
      const next = new Set(previous);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  const kinds: Readonly<Record<ChangeKind, string>> = {
    added: t('changes.added', 'New'),
    updated: t('changes.updated', 'Changed'),
    deleted: t('changes.deleted', 'Removed'),
    unchanged: t('changes.unchanged', 'Unchanged'),
  };
  const active = changes.filter((change: Change) => change.change !== 'unchanged');
  const unchanged = changes.length - active.length;
  // A change the source holds back can never be applied, so it is not one to tick.
  const keys = active.filter((change: Change) => !change.sourceExcluded).map((change: Change) => change.key);
  const allChecked = keys.length > 0 && keys.every((key: string) => selected?.has(key));
  const someChecked = keys.some((key: string) => selected?.has(key));

  if (active.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary">
        {t('changes.none', 'The gateway already runs this version as it is.')}
      </Typography>
    );
  }
  const label = (change: Change): JSX.Element => (
    <Stack direction="row" alignItems="center" spacing={1} sx={{minWidth: 0}}>
      <Typography variant="body2" noWrap>
        {change.name ?? change.id ?? change.key}
      </Typography>
      <Typography variant="caption" color="text.secondary" noWrap>
        {t(`changes.types.${change.resourceType}`, change.resourceType.replace(/_/g, ' '))}
      </Typography>
      <Chip
        size="small"
        variant="outlined"
        color={COLORS[change.change]}
        label={kinds[change.change]}
        sx={{height: 18, fontSize: 11}}
      />
      {change.sourceExcluded && (
        <Chip
          size="small"
          label={
            source
              ? t('changes.heldBackOn', 'Held back on {{target}}', {target: source})
              : t('changes.heldBack', 'Held back')
          }
          data-testid="change-source-held-back"
          title={t('changes.sourceHeldBackHint', '{{source}} does not run this change, so it cannot be promoted.', {
            source: source ?? '',
          })}
          sx={{height: 18, fontSize: 11, bgcolor: 'action.selected', color: 'text.secondary'}}
        />
      )}
      {change.excluded && !change.sourceExcluded && (
        <Chip
          size="small"
          label={
            target ? t('changes.heldBackOn', 'Held back on {{target}}', {target}) : t('changes.heldBack', 'Held back')
          }
          data-testid="change-held-back"
          sx={{height: 18, fontSize: 11, bgcolor: 'action.selected', color: 'text.secondary'}}
        />
      )}
    </Stack>
  );
  return (
    <Box>
      {selected && (
        <FormControlLabel
          control={
            <Checkbox
              size="small"
              checked={allChecked}
              indeterminate={!allChecked && someChecked}
              onChange={(_event, checked: boolean) => onToggleAll?.(keys, checked)}
            />
          }
          label={
            <Typography variant="body2" fontWeight={600}>
              {t('changes.selectAll', '{{count}} resources to write', {count: keys.length})}
            </Typography>
          }
        />
      )}
      <Stack sx={{pl: 0.5}}>
        {active.map((change: Change) => {
          const name = change.name ?? change.id ?? change.key;
          const hasLines = (change.lines ?? []).some((line) => line.kind !== ' ');
          const isOpen = open.has(change.key);
          return (
            <Box key={change.key} data-testid={`change-${change.key}`}>
              {/* The whole row opens the diff; only the checkbox selects. The button stays for the
                  keyboard and for what it tells a screen reader. */}
              <Stack
                direction="row"
                alignItems="center"
                spacing={1}
                data-testid={`change-row-${change.key}`}
                onClick={hasLines ? () => toggleOpen(change.key) : undefined}
                sx={{
                  minHeight: 38,
                  borderRadius: 1,
                  ...(hasLines ? {cursor: 'pointer', '&:hover': {bgcolor: 'action.hover'}} : {}),
                }}
              >
                {selected && (
                  <Checkbox
                    size="small"
                    checked={!change.sourceExcluded && selected.has(change.key)}
                    disabled={change.sourceExcluded}
                    onClick={(event) => event.stopPropagation()}
                    onChange={() => onToggle?.(change.key)}
                    inputProps={{'aria-label': t('changes.selectOne', 'Write {{name}}', {name})}}
                  />
                )}
                <Box sx={{minWidth: 0, py: 0.5}}>{label(change)}</Box>
                {hasLines && (
                  <Button
                    size="small"
                    variant="text"
                    sx={{ml: 'auto', flexShrink: 0, textTransform: 'none'}}
                    endIcon={isOpen ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                    aria-expanded={isOpen}
                    aria-label={
                      isOpen
                        ? t('diff.hideLabel', 'Hide the changes to {{name}}', {name})
                        : t('diff.showLabel', 'Show the changes to {{name}}', {name})
                    }
                    onClick={(event) => {
                      event.stopPropagation();
                      toggleOpen(change.key);
                    }}
                  >
                    {isOpen ? t('diff.hide', 'Hide changes') : t('diff.show', 'Show changes')}
                  </Button>
                )}
              </Stack>
              {hasLines && (
                <Collapse in={isOpen} unmountOnExit>
                  <Box sx={{pb: 1, pl: selected ? 3.5 : 0}}>
                    <ChangeLines lines={change.lines ?? []} />
                  </Box>
                </Collapse>
              )}
            </Box>
          );
        })}
      </Stack>
      {unchanged > 0 && (
        <Typography variant="caption" color="text.secondary" component="div" sx={{mt: 1}}>
          {t('changes.unchangedCount', '{{count}} resources are already as the version has them.', {count: unchanged})}
        </Typography>
      )}
    </Box>
  );
}
