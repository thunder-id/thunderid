// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Checkbox, Chip, Collapse, FormControlLabel, Link, Stack, Switch, Typography} from '@wso2/oxygen-ui';
import {CircleCheck} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ChangeFlags from './ChangeFlags';
import ChangeLines from './ChangeLines';
import useChangeName from '../hooks/useChangeName';
import type {Change, ChangeKind} from '../models/deploy';
import {changedFields, isShared} from '../utils/changeSummary';
import {humanize} from '../utils/groupByResource';

export interface DeployChangesProps {
  changes: Change[];
  /** The keys of the changes that will be applied. */
  selected: Set<string>;
  onToggle: (key: string) => void;
  onToggleAll: (keys: string[], checked: boolean) => void;
  /** The gateway the changes are applied to. */
  target: string;
  /** The gateway the changes come from, on a promotion. */
  source?: string;
  /** The changes a selection brings along, each with the key of the selected change that needs it. */
  required?: ReadonlyMap<string, string>;
}

const COLORS: Readonly<Record<ChangeKind, 'success' | 'info' | 'error' | 'default'>> = {
  added: 'success',
  updated: 'info',
  deleted: 'error',
  unchanged: 'default',
};

/**
 * What a deploy writes, as the deploy drawer lists it: the project's own changes first, then the
 * resources the whole organization shares, which ship with every deploy. Each change is a row with its
 * kind, what it is and what changed, a checkbox to leave it out this time and its diff to open in place.
 *
 * A project's deploy check places each change in a section: the project's own changes start ticked and
 * can be unticked, and what the ticked ones cannot run without is carried and locked; what they need in
 * another form and what is shared are carried only when ticked; another project's changes are shown and
 * never carried.
 */
export default function DeployChanges({
  changes,
  selected,
  onToggle,
  onToggleAll,
  target,
  source = undefined,
  required = undefined,
}: DeployChangesProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const nameOfChange = useChangeName();
  const isLocked = (change: Change): boolean => Boolean(required?.has(change.key));
  const [open, setOpen] = useState<ReadonlySet<string>>(new Set());
  const active = changes.filter((change: Change) => change.change !== 'unchanged');
  const unchanged = changes.length - active.length;
  const kinds: Readonly<Record<ChangeKind, string>> = {
    added: t('deployChanges.added', 'Added'),
    updated: t('deployChanges.modified', 'Modified'),
    deleted: t('deployChanges.removed', 'Removed'),
    unchanged: t('changes.unchanged', 'Unchanged'),
  };
  const nameOf = (key: string): string => {
    const change = changes.find((entry: Change) => entry.key === key);
    return change ? nameOfChange(change) : key;
  };
  const toggleOpen = (key: string): void =>
    setOpen((previous) => {
      const next = new Set(previous);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  if (active.length === 0) {
    return (
      <Typography variant="body2" color="text.secondary" data-testid="deploy-changes-none">
        {t('changes.noneAt', '{{target}} already runs this version as it is.', {target})}
      </Typography>
    );
  }

  const summary = (change: Change): string => {
    const type = t(`changes.types.${change.resourceType}`, {defaultValue: humanize(change.resourceType)});
    if (change.change === 'added') return t('deployChanges.new', '{{type}} · new', {type});
    if (change.change === 'deleted') return t('deployChanges.gone', '{{type}} · removed', {type});
    const fields = changedFields(change.lines);
    if (fields.length > 0) return `${type} · ${fields.join(', ')}`;
    if (change.valueChanged) return t('deployChanges.valueChanged', '{{type}} · a value it uses changed', {type});
    return type;
  };

  const row = (change: Change): JSX.Element => {
    const name = nameOfChange(change);
    // Under a project, what it always carries is ticked for good, and another project's is never carried.
    const locked = isLocked(change);
    const readOnly = change.section === 'other';
    const hasLines = (change.lines ?? []).some((line) => line.kind !== ' ');
    const isOpen = open.has(change.key);
    return (
      <Box
        key={change.key}
        data-testid={`change-${change.key}`}
        sx={{border: 1, borderColor: 'divider', borderRadius: 2, px: 1.5, py: 1}}
      >
        <Stack direction="row" alignItems="center" spacing={1.5}>
          <Checkbox
            size="small"
            checked={!change.sourceExcluded && !readOnly && (locked || selected.has(change.key))}
            disabled={change.sourceExcluded === true || locked || readOnly}
            onChange={() => onToggle(change.key)}
            inputProps={{'aria-label': t('changes.selectOne', 'Write {{name}}', {name})}}
            sx={{p: 0.5}}
          />
          <Chip
            size="small"
            variant="outlined"
            color={COLORS[change.change]}
            label={kinds[change.change]}
            sx={{height: 20, fontSize: 11, minWidth: 72, flexShrink: 0}}
          />
          <Box sx={{flex: 1, minWidth: 0}}>
            <Typography variant="body2" fontWeight={500} noWrap>
              {name}
            </Typography>
            <Typography variant="caption" color="text.secondary" component="div" noWrap>
              {summary(change)}
            </Typography>
            <Stack direction="row" spacing={0.75} sx={{flexWrap: 'wrap', rowGap: 0.5, '&:empty': {display: 'none'}}}>
              <ChangeFlags
                change={change}
                target={target}
                source={source}
                neededBy={required?.has(change.key) ? nameOf(required.get(change.key) ?? '') : undefined}
              />
              {locked && change.section === 'needed' && (
                <Chip
                  size="small"
                  label={t('deployChanges.lacking', '{{target}} lacks it', {target})}
                  data-testid="change-required"
                  sx={{height: 18, fontSize: 11, bgcolor: 'action.selected', color: 'text.secondary'}}
                />
              )}
              {(change.usedBy ?? []).length > 0 && (
                <Chip
                  size="small"
                  color="warning"
                  variant="outlined"
                  label={t('deployChanges.usedBy', 'Also used by {{projects}}', {
                    projects: (change.usedBy ?? []).join(', '),
                  })}
                  title={t(
                    'deployChanges.usedByHint',
                    'Bringing this changes it on {{target}} for these projects too.',
                    {target},
                  )}
                  data-testid="change-used-by"
                  sx={{height: 18, fontSize: 11}}
                />
              )}
            </Stack>
          </Box>
          {hasLines && (
            <Link
              component="button"
              type="button"
              variant="body2"
              underline="hover"
              aria-expanded={isOpen}
              aria-label={
                isOpen
                  ? t('diff.hideLabel', 'Hide the changes to {{name}}', {name})
                  : t('diff.showLabel', 'Show the changes to {{name}}', {name})
              }
              onClick={() => toggleOpen(change.key)}
              sx={{flexShrink: 0}}
            >
              {t('deployChanges.diff', 'Diff')}
            </Link>
          )}
        </Stack>
        {hasLines && (
          <Collapse in={isOpen} unmountOnExit>
            <Box sx={{pt: 1}}>
              <ChangeLines lines={change.lines ?? []} />
            </Box>
          </Collapse>
        )}
      </Box>
    );
  };

  const section = (id: string, title: string, rows: Change[], note?: string, bringAll = false): JSX.Element | null => {
    if (rows.length === 0) return null;
    const keys = rows
      .filter((change: Change) => !change.sourceExcluded && !isLocked(change) && change.section !== 'other')
      .map((change: Change) => change.key);
    const allChecked = keys.length > 0 && keys.every((key: string) => selected.has(key));
    const someChecked = keys.some((key: string) => selected.has(key));
    return (
      <Stack spacing={1.25} component="section" aria-label={title} data-testid={`deploy-changes-${id}`}>
        <Stack direction="row" alignItems="center" spacing={1}>
          {keys.length > 0 && (
            <Checkbox
              size="small"
              checked={allChecked}
              indeterminate={!allChecked && someChecked}
              onChange={(_event, checked: boolean) => onToggleAll(keys, checked)}
              inputProps={{'aria-label': t('deployChanges.selectAll', 'Write every one of {{title}}', {title})}}
              sx={{p: 0.5}}
            />
          )}
          <Typography variant="subtitle2" sx={{flex: 1}}>
            {t('deployChanges.title', '{{title}} · {{count}}', {title, count: rows.length})}
          </Typography>
          {bringAll && keys.length > 0 && (
            <FormControlLabel
              control={
                <Switch
                  size="small"
                  checked={allChecked}
                  onChange={(_event, checked: boolean) => onToggleAll(keys, checked)}
                  data-testid="deploy-changes-bring-all-shared"
                />
              }
              label={
                <Typography variant="body2">{t('deployChanges.bringAllShared', 'Bring all shared updates')}</Typography>
              }
              sx={{mr: 0}}
            />
          )}
        </Stack>
        {note && (
          <Typography
            variant="body2"
            color="text.secondary"
            sx={{px: 1.5, py: 1, borderRadius: 1.5, bgcolor: 'action.hover'}}
          >
            {note}
          </Typography>
        )}
        {rows.map(row)}
      </Stack>
    );
  };

  if (active.some((change: Change) => change.section !== undefined)) {
    const of = (wanted: Change['section']): Change[] => active.filter((change: Change) => change.section === wanted);
    const own = of('project');
    return (
      <Stack spacing={3}>
        {own.length > 0 ? (
          section(
            'project',
            t('deployChanges.thisProject', 'Project changes'),
            own,
            t(
              'deployChanges.thisProjectFocusedNote',
              'Tick the project changes to bring. What they need goes with them.',
            ),
          )
        ) : (
          // Nothing of the project's own to bring: said in its place, rather than leaving the deploy to
          // read as though another project's changes were what it carries.
          <Stack
            spacing={1.25}
            component="section"
            aria-label={t('deployChanges.thisProject', 'Project changes')}
            data-testid="deploy-changes-project"
          >
            <Typography variant="subtitle2">{t('deployChanges.thisProject', 'Project changes')}</Typography>
            <Stack
              direction="row"
              spacing={1.5}
              alignItems="center"
              data-testid="deploy-changes-project-none"
              sx={{px: 2, py: 1.75, borderRadius: 1.5, border: 1, borderStyle: 'dashed', borderColor: 'divider'}}
            >
              <Box sx={{display: 'flex', color: 'success.main'}}>
                <CircleCheck size={18} />
              </Box>
              <Box>
                <Typography variant="body2">
                  {t('deployChanges.thisProjectNone', '{{target}} is up to date with this project', {target})}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  {t(
                    'deployChanges.thisProjectNoneHint',
                    "The project's applications and agents on {{target}} already match this version.",
                    {target},
                  )}
                </Typography>
              </Box>
            </Stack>
          </Stack>
        )}
        {section(
          'needed',
          t('deployChanges.needed', 'What they need'),
          of('needed'),
          t(
            'deployChanges.neededNote',
            'What {{target}} lacks goes with them. What {{target}} holds in another form already serves them; tick it to replace it.',
            {target},
          ),
        )}
        {section(
          'shared',
          t('deployChanges.sharedProject', 'Shared resources'),
          of('shared'),
          t(
            'deployChanges.sharedProjectNote',
            'These belong to no project, such as translations, branding and other flows. Tick what to bring.',
          ),
          true,
        )}
        {own.length > 0 &&
          section(
            'other',
            t('deployChanges.otherProjects', 'Other projects'),
            of('other'),
            t('deployChanges.otherProjectsNote', 'Each project promotes its own, so these stay as they are.'),
          )}
        {unchanged > 0 && (
          <Typography variant="caption" color="text.secondary">
            {t('changes.unchangedCount', '{{count}} resources are already as the version has them.', {
              count: unchanged,
            })}
          </Typography>
        )}
      </Stack>
    );
  }

  return (
    <Stack spacing={3}>
      {section(
        'project',
        t('deployChanges.changes', 'Changes'),
        active.filter((change: Change) => !isShared(change)),
      )}
      {section(
        'shared',
        t('deployChanges.shared', 'Shared resources'),
        active.filter(isShared),
        t(
          'deployChanges.sharedNote',
          'Organization resources ship with the deploy, so every gateway runs the same flows, connections and branding.',
        ),
      )}
      {unchanged > 0 && (
        <Typography variant="caption" color="text.secondary">
          {t('changes.unchangedCount', '{{count}} resources are already as the version has them.', {count: unchanged})}
        </Typography>
      )}
    </Stack>
  );
}
