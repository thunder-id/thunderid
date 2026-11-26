// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CopyableField, SettingsCard} from '@thunderid/components';
import {Box, Button, Divider, Stack} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import CspDirectiveRow, {type CspDirectiveRowAction} from './CspDirectiveRow';
import {CSP_DEFAULT_SCOPE, type CspScopeRow} from '../../models/cspDirectiveRow';
import {rowSources} from '../../utils/cspDirectiveRows';
import buildCspHeader from '../../utils/cspHeader';

/**
 * Props for {@link CspScopeCard}.
 *
 * @public
 */
export interface CspScopeCardProps {
  /** {@link CSP_DEFAULT_SCOPE} or a path location. */
  scope: string;
  /** The directives that apply to the scope. */
  rows: CspScopeRow[];
  /** Directives the scope does not have yet. */
  availableDirectives: string[];
  /** Error messages, keyed by draft row id. */
  errors: Record<string, string>;
  /** Whether the policy is report-only, for the header preview. */
  reportOnly: boolean;
  /** The effective report URI, for the header preview. */
  reportUri?: string;
  onSourcesChange: (directive: string, sources: string[]) => void;
  onDirectiveChange: (draftId: string, directive: string) => void;
  onRemove: (draftId: string) => void;
  onAdd: () => void;
}

/**
 * Picks the trailing action for a row: a directive with no default value can be removed, and an
 * overridden one can be reset.
 */
function rowAction(row: CspScopeRow): CspDirectiveRowAction | undefined {
  if (row.draftId === undefined) {
    return undefined;
  }
  return row.base.length === 0 ? 'remove' : 'reset';
}

/**
 * One scope of the CSP section: the top-level directives or a path's, with the header that scope sends.
 *
 * @public
 */
export default function CspScopeCard({
  scope,
  rows,
  availableDirectives,
  errors,
  reportOnly,
  reportUri = undefined,
  onSourcesChange,
  onDirectiveChange,
  onRemove,
  onAdd,
}: CspScopeCardProps): JSX.Element {
  const {t} = useTranslation();

  let title: string;
  let description: string;
  if (scope === CSP_DEFAULT_SCOPE) {
    title = t('settings:csp.scope.shared', 'Shared');
    description = t(
      'settings:csp.scope.sharedDescription',
      'Applies to the Console and Gate. A directive set in the Console or Gate section replaces the one here for that section.',
    );
  } else {
    const names: Record<string, string> = {
      '/console/': t('settings:csp.scope.console', 'Console'),
      '/gate/': t('settings:csp.scope.gate', 'Gate'),
    };
    title = names[scope] ?? scope;
    description = t('settings:csp.scope.pathDescription', 'Applies to pages under {{location}}.', {location: scope});
  }

  return (
    <Box component="section" aria-label={title}>
      <SettingsCard title={title} description={description}>
        <Stack spacing={1}>
          {rows.map((row) => (
            <CspDirectiveRow
              // A default row keeps its key when it gains an override, so the field keeps focus.
              key={row.base.length > 0 ? row.directive : row.draftId}
              testId="csp-directive-row"
              directive={row.directive}
              sources={rowSources(row)}
              directiveOptions={row.base.length === 0 ? availableDirectives : undefined}
              error={row.draftId ? errors[row.draftId] : undefined}
              action={rowAction(row)}
              onDirectiveChange={(directive) => {
                if (row.draftId) {
                  onDirectiveChange(row.draftId, directive);
                }
              }}
              onSourcesChange={(sources) => onSourcesChange(row.directive, sources)}
              onAction={() => {
                if (row.draftId) {
                  onRemove(row.draftId);
                }
              }}
            />
          ))}
        </Stack>

        {availableDirectives.length > 0 && (
          <Button variant="text" color="primary" startIcon={<Plus size={18} />} onClick={onAdd} sx={{mt: 2}}>
            {t('settings:csp.addDirective', 'Add directive')}
          </Button>
        )}

        <Divider sx={{my: 2}} />
        <CopyableField
          label={
            scope === CSP_DEFAULT_SCOPE
              ? t('settings:csp.header.sharedLabel', 'Header sent when the Console or Gate has no section of its own')
              : t('settings:csp.header.label', 'Header sent for {{scope}}', {scope: title})
          }
          value={buildCspHeader(rows, reportOnly, reportUri)}
        />
      </SettingsCard>
    </Box>
  );
}
