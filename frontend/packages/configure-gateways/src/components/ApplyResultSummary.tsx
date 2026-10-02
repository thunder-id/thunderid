// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import MissingValuesNotice from './MissingValuesNotice';
import {GatewayDetailTabs, type GatewayValuesTab} from '../constants/gateway-values';
import type {ApplyResult, ImportResourceResult} from '../models/gateway';
import {hasMissingValues} from '../utils/missingValues';

export interface ApplyResultSummaryProps {
  result: ApplyResult;
  /** Opens the gateway's variables or secrets tab, offered when the result lists missing values. */
  onManage?: (tab: GatewayValuesTab) => void;
}

/**
 * The gateway's account of an apply or a revert: how many resources it took, and each one it
 * refused.
 */
export default function ApplyResultSummary({result, onManage = undefined}: ApplyResultSummaryProps): JSX.Element {
  const {t} = useTranslation();
  const summary = result.import?.summary;
  const failures: ImportResourceResult[] = (result.import?.results ?? []).filter(
    (entry: ImportResourceResult) => entry.status === 'failed',
  );
  const failed = summary?.failed ?? failures.length;

  let severity: 'success' | 'info' | 'warning' = 'success';
  let message: string;
  if (failed > 0) {
    severity = 'warning';
    message = result.dryRun
      ? t('gateways:result.dryRunWithFailures', 'Dry run finished. The gateway would refuse some resources.')
      : t('gateways:result.appliedWithFailures', 'Applied, but the gateway refused some resources.');
  } else if (result.dryRun) {
    severity = 'info';
    message = t('gateways:result.dryRun', 'Dry run finished. Nothing was changed on the gateway.');
  } else {
    message = t('gateways:result.applied', 'The gateway now holds this configuration.');
  }

  return (
    <Stack spacing={2}>
      <Alert severity={severity}>{message}</Alert>

      {hasMissingValues(result.missing) && (
        <MissingValuesNotice
          missing={result.missing}
          onManageVariables={onManage ? () => onManage(GatewayDetailTabs.VARIABLES) : undefined}
          onManageSecrets={onManage ? () => onManage(GatewayDetailTabs.SECRETS) : undefined}
        />
      )}

      {summary && (
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          <Chip
            size="small"
            variant="outlined"
            label={t('gateways:result.total', '{{count}} documents', {count: summary.totalDocuments})}
          />
          <Chip
            size="small"
            color="success"
            variant="outlined"
            label={t('gateways:result.imported', '{{count}} imported', {count: summary.imported})}
          />
          {summary.deleted !== undefined && (
            <Chip
              size="small"
              color="warning"
              variant="outlined"
              label={t('gateways:result.deleted', '{{count}} deleted', {count: summary.deleted})}
            />
          )}
          <Chip
            size="small"
            color={summary.failed > 0 ? 'error' : 'default'}
            variant="outlined"
            label={t('gateways:result.failed', '{{count}} failed', {count: summary.failed})}
          />
        </Stack>
      )}

      {failures.length > 0 && (
        <Box>
          <Typography variant="subtitle2" sx={{mb: 1}}>
            {t('gateways:result.failuresTitle', 'Resources the gateway refused')}
          </Typography>
          <Stack spacing={0.5}>
            {failures.map((entry: ImportResourceResult, index: number) => (
              <Box
                // A refused resource may carry no id, so its position is part of its identity.
                key={`${entry.resourceType}-${entry.resourceId ?? ''}-${String(index)}`}
                sx={{px: 1.5, py: 0.75, borderLeft: 3, borderLeftColor: 'error.main', borderRadius: 1}}
              >
                <Typography variant="body2" sx={{fontWeight: 500}}>
                  {entry.resourceName ?? entry.resourceId ?? '-'}
                </Typography>
                <Typography variant="caption" color="text.secondary" component="div">
                  {entry.resourceType}
                  {entry.code ? ` (${entry.code})` : ''}
                </Typography>
                {entry.message && (
                  <Typography variant="caption" color="error" component="div">
                    {entry.message}
                  </Typography>
                )}
              </Box>
            ))}
          </Stack>
        </Box>
      )}
    </Stack>
  );
}
