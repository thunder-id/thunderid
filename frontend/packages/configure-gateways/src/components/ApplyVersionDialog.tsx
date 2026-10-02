// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormControlLabel,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ApplyResultSummary from './ApplyResultSummary';
import DiffSummaryChips from './DiffSummaryChips';
import MissingValuesCheck from './MissingValuesCheck';
import ResourceDiffList from './ResourceDiffList';
import useApplyDryRun from '../api/useApplyDryRun';
import useApplyVersion from '../api/useApplyVersion';
import useGetConfigurationVersions from '../api/useGetConfigurationVersions';
import useGetGatewayDiff from '../api/useGetGatewayDiff';
import type {GatewayValuesTab} from '../constants/gateway-values';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import useOpenGatewayTab from '../hooks/useOpenGatewayTab';
import type {ApplyResult, ConfigurationVersion} from '../models/gateway';
import getApplyErrorMessage from '../utils/getApplyErrorMessage';
import {hasMissingValues, isMissingValuesError} from '../utils/missingValues';

const LATEST = 'latest';

export interface ApplyVersionDialogProps {
  open: boolean;
  gatewayId: string;
  gatewayName: string;
  onClose: () => void;
}

/**
 * Applies a configuration version to a gateway after showing what it would change there.
 *
 * The comparison is against what the gateway holds now, not against the version before the chosen
 * one, because the two differ whenever versions were skipped. A dry run checks, as soon as a
 * version is chosen, that the gateway holds every variable and secret the version refers to, and
 * the apply waits until it does.
 */
export default function ApplyVersionDialog({
  open,
  gatewayId,
  gatewayName,
  onClose,
}: ApplyVersionDialogProps): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const logger = useLogger('ApplyVersionDialog');
  const [version, setVersion] = useState<string>(LATEST);
  const [dryRun, setDryRun] = useState(false);
  const [result, setResult] = useState<ApplyResult | null>(null);
  // The version number the reviewed diff led to. "Latest" moves when another capture lands, so an
  // apply names this number rather than the alias, and the follow-up "Apply now" reuses it.
  const [appliedVersion, setAppliedVersion] = useState<string | null>(null);
  const {data: versions} = useGetConfigurationVersions();
  const diff = useGetGatewayDiff(gatewayId, version, open && result === null);
  const applyVersion = useApplyVersion();
  const openGatewayTab = useOpenGatewayTab();

  const latest: ConfigurationVersion | undefined = versions?.[0];
  const hasVersions = (versions?.length ?? 0) > 0;
  const check = useApplyDryRun(gatewayId, version, open && result === null && hasVersions);
  const isMissingValues = hasMissingValues(check.data?.missing);

  const reset = (): void => {
    setVersion(LATEST);
    setDryRun(false);
    setResult(null);
    setAppliedVersion(null);
    applyVersion.reset();
  };

  const handleClose = (): void => {
    if (applyVersion.isPending) return;
    reset();
    onClose();
  };

  const handleManage = (tab: GatewayValuesTab): void => {
    if (applyVersion.isPending) return;
    handleClose();
    openGatewayTab(gatewayId, tab);
  };

  const runApply = (asDryRun: boolean, target: string): void => {
    setAppliedVersion(target);
    applyVersion.mutate(
      {gatewayId, data: {version: target, dryRun: asDryRun}},
      {
        onSuccess: (applied: ApplyResult) => {
          setResult(applied);
        },
        onError: (err: Error) => {
          logger.error('Failed to apply configuration version', {error: err});
          // The gateway lost a value since the check ran, so check again to name it.
          if (isMissingValuesError(err)) void check.refetch();
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="md" fullWidth>
      <DialogTitle>{t('gateways:apply.title', 'Apply a configuration version')}</DialogTitle>
      <DialogContent dividers>
        {result ? (
          <ApplyResultSummary result={result} onManage={handleManage} />
        ) : (
          <Stack spacing={3}>
            <Typography variant="body2" color="text.secondary">
              {t(
                'gateways:apply.description',
                'Review what the version would change on {{name}} before applying it. Resources the version no longer has are removed from the gateway.',
                {name: gatewayName},
              )}
            </Typography>

            <FormControl fullWidth>
              <FormLabel id="apply-version-label">{t('gateways:apply.version.label', 'Version')}</FormLabel>
              <Select
                labelId="apply-version-label"
                value={version}
                disabled={!hasVersions}
                onChange={(e) => {
                  applyVersion.reset();
                  setVersion(String(e.target.value));
                }}
              >
                <MenuItem value={LATEST}>
                  {latest
                    ? t('gateways:apply.version.latestNumbered', 'Latest (version {{version}})', {
                        version: latest.version,
                      })
                    : t('gateways:apply.version.latest', 'Latest')}
                </MenuItem>
                {(versions ?? []).map((entry: ConfigurationVersion) => (
                  <MenuItem key={entry.version} value={String(entry.version)}>
                    {entry.note
                      ? t('gateways:apply.version.optionWithNote', 'Version {{version}}: {{note}}', {
                          version: entry.version,
                          note: entry.note,
                        })
                      : t('gateways:versions.versionNumber', 'Version {{version}}', {version: entry.version})}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>

            {versions && !hasVersions && (
              <Alert severity="info">
                {t(
                  'gateways:apply.noVersions',
                  'No configuration has been captured yet. Capture a version before applying one.',
                )}
              </Alert>
            )}

            {hasVersions && diff.isLoading && (
              <Box sx={{display: 'flex', justifyContent: 'center', py: 4}}>
                <CircularProgress size={24} />
              </Box>
            )}

            {hasVersions && diff.error && (
              <Alert severity="error">
                {t('gateways:apply.diffError', 'The changes could not be loaded, so this apply cannot be reviewed.')}
              </Alert>
            )}

            {hasVersions && diff.data && (
              <Stack spacing={2}>
                <Typography variant="subtitle2">
                  {diff.data.fromVersion !== undefined
                    ? t('gateways:apply.diffTitle', 'Changes from version {{from}} to version {{to}}', {
                        from: diff.data.fromVersion,
                        to: diff.data.toVersion,
                      })
                    : t('gateways:apply.diffTitleFirst', 'Changes to apply version {{to}}', {
                        to: diff.data.toVersion,
                      })}
                </Typography>
                <DiffSummaryChips summary={diff.data.summary} />
                <ResourceDiffList diff={diff.data} />
              </Stack>
            )}

            {hasVersions && <MissingValuesCheck gatewayId={gatewayId} check={check} onManage={handleManage} />}

            <FormControlLabel
              control={<Checkbox checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} />}
              label={t('gateways:apply.dryRun', 'Dry run: check the import on the gateway without changing it')}
            />
          </Stack>
        )}

        {applyVersion.error && (
          <Alert severity="error" sx={{mt: 2}}>
            {getApplyErrorMessage(applyVersion.error, tForErrors)}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        {result ? (
          <>
            {result.dryRun && (
              <Button
                variant="outlined"
                onClick={() => appliedVersion && runApply(false, appliedVersion)}
                disabled={applyVersion.isPending || hasMissingValues(result.missing)}
              >
                {applyVersion.isPending
                  ? t('gateways:apply.submitting', 'Applying...')
                  : t('gateways:apply.applyNow', 'Apply now')}
              </Button>
            )}
            <Button variant="contained" onClick={handleClose} disabled={applyVersion.isPending}>
              {t('gateways:apply.done', 'Done')}
            </Button>
          </>
        ) : (
          <>
            <Button variant="outlined" onClick={handleClose} disabled={applyVersion.isPending}>
              {t('common:actions.cancel', 'Cancel')}
            </Button>
            <Button
              variant="contained"
              onClick={() => diff.data && runApply(dryRun, String(diff.data.toVersion))}
              disabled={applyVersion.isPending || !hasVersions || !diff.data || check.isFetching || isMissingValues}
            >
              {applyVersion.isPending
                ? t('gateways:apply.submitting', 'Applying...')
                : dryRun
                  ? t('gateways:apply.submitDryRun', 'Run dry run')
                  : t('gateways:apply.submit', 'Apply')}
            </Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}
