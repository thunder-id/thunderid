// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Accordion, AccordionDetails, AccordionSummary, Box, Checkbox, Chip, Typography} from '@wso2/oxygen-ui';
import {ChevronDown} from '@wso2/oxygen-ui-icons-react';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ChangeType, Diff, LineOp, ResourceChange} from '../models/promotion';

/** Colour per change type, used for both the chip and the left border. */
const CHANGE_COLORS: Record<ChangeType, 'success' | 'warning' | 'error' | 'default'> = {
  added: 'success',
  updated: 'warning',
  deleted: 'error',
  unchanged: 'default',
};

interface ResourceDiffListProps {
  diff: Diff;
  /** When set, each changed resource gets a checkbox so the user can reject individual changes. */
  selectable?: boolean;
  /** Keys of the changes currently selected. Only meaningful when selectable. */
  selectedKeys?: Set<string>;
  onToggle?: (key: string) => void;
  /** Unchanged resources are hidden by default; they add noise to a promotion review. */
  showUnchanged?: boolean;
}

/**
 * Renders a configuration diff as a list of resources, each expandable to a line-level diff.
 *
 * In selectable mode every changed resource carries a checkbox, which is what lets a user reject
 * part of a promotion and take only the rest.
 */
export default function ResourceDiffList({
  diff,
  selectable = false,
  selectedKeys = undefined,
  onToggle = undefined,
  showUnchanged = false,
}: ResourceDiffListProps): JSX.Element {
  const {t} = useTranslation();

  const changes: ResourceChange[] = showUnchanged
    ? diff.changes
    : diff.changes.filter((change: ResourceChange) => change.change !== 'unchanged');

  if (changes.length === 0) {
    return (
      <Box sx={{textAlign: 'center', py: 6}}>
        <Typography variant="body1" color="text.secondary">
          {t('promotions:diff.noChanges', 'No differences. The two versions are identical.')}
        </Typography>
      </Box>
    );
  }

  return (
    <Box>
      {changes.map((change: ResourceChange) => {
        const isSelected: boolean = selectedKeys?.has(change.key) ?? false;

        return (
          <Accordion
            key={change.key}
            disableGutters
            sx={{
              mb: 1,
              borderLeft: 3,
              borderLeftColor: `${CHANGE_COLORS[change.change]}.main`,
              '&:before': {display: 'none'},
            }}
          >
            <AccordionSummary expandIcon={<ChevronDown size={16} />}>
              <Box sx={{alignItems: 'center', display: 'flex', flexGrow: 1, gap: 1.5, minWidth: 0}}>
                {selectable && change.change !== 'unchanged' && (
                  <Checkbox
                    checked={isSelected}
                    onChange={() => onToggle?.(change.key)}
                    onClick={(event) => {
                      // Keep the checkbox from toggling the accordion.
                      event.stopPropagation();
                    }}
                    inputProps={{
                      'aria-label': t('promotions:diff.selectChange', 'Include this change'),
                    }}
                  />
                )}
                <Chip size="small" color={CHANGE_COLORS[change.change]} label={changeLabel(t, change.change)} />
                <Box sx={{minWidth: 0}}>
                  <Typography variant="body2" noWrap sx={{fontWeight: 500}}>
                    {change.name ?? change.id ?? change.key}
                  </Typography>
                  <Typography variant="caption" color="text.secondary">
                    {change.type}
                    {change.id ? ` · ${change.id}` : ''}
                  </Typography>
                </Box>
              </Box>
            </AccordionSummary>
            <AccordionDetails>
              <DiffLines lines={change.lines} />
            </AccordionDetails>
          </Accordion>
        );
      })}
    </Box>
  );
}

/** Translates a change type into its display label. */
function changeLabel(t: (key: string, fallback: string) => string, change: ChangeType): string {
  switch (change) {
    case 'added':
      return t('promotions:diff.added', 'Added');
    case 'updated':
      return t('promotions:diff.updated', 'Updated');
    case 'deleted':
      return t('promotions:diff.deleted', 'Deleted');
    default:
      return t('promotions:diff.unchanged', 'Unchanged');
  }
}

/** Renders the unified line diff for one resource. */
function DiffLines({lines = undefined}: {lines?: LineOp[]}): JSX.Element {
  const {t} = useTranslation();

  if (!lines || lines.length === 0) {
    return (
      <Typography variant="caption" color="text.secondary">
        {t('promotions:diff.noLineDetail', 'No line-level detail available.')}
      </Typography>
    );
  }

  return (
    <Box
      component="pre"
      sx={{
        borderRadius: 1,
        fontFamily: 'monospace',
        fontSize: '0.75rem',
        m: 0,
        maxHeight: 400,
        overflow: 'auto',
        p: 1.5,
      }}
    >
      {lines.map((line: LineOp, index: number) => (
        <Box
          // Diff lines have no stable identity of their own; position is the identity here.
          key={`${String(index)}-${line.kind}`}
          component="div"
          sx={{
            backgroundColor: lineBackground(line.kind),
            color: line.kind === ' ' ? 'text.secondary' : 'text.primary',
            px: 0.5,
            whiteSpace: 'pre-wrap',
            wordBreak: 'break-all',
          }}
        >
          {line.kind}
          {line.text}
        </Box>
      ))}
    </Box>
  );
}

function lineBackground(kind: string): string {
  if (kind === '+') {
    return 'success.light';
  }
  if (kind === '-') {
    return 'error.light';
  }
  return 'transparent';
}
