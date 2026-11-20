// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice} from '@thunderid/components';
import {useHiddenResources} from '@thunderid/contexts';
import {
  Alert,
  Box,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {ShieldCheck, Undo2} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ChangedValuesNotice from './ChangedValuesNotice';
import ChangeList from './ChangeList';
import {useRevert, useRevertPreview} from '../api/useGatewayVersions';
import useProjectScope from '../contexts/ProjectScope';
import useChangeName from '../hooks/useChangeName';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import type {ApplyResult, Change, ImportResourceResult, RemovalInUse} from '../models/deploy';
import type {Gateway} from '../models/gateway';
import {APPLIED_CHANGED, errorCodeOf, failureText, isForbidden, REMOVALS_NEED_CONFIRMATION} from '../utils/apiError';

export interface RevertPanelProps {
  gateway: Gateway;
  /** The label of the version the gateway would go back to. */
  previousLabel?: string;
  /** The rollback undoes the last deploy, going back within the version it runs, not to an older one. */
  undo?: boolean;
  /** What the gateway is called, its gateway's name otherwise. */
  targetName?: string;
  onClose: () => void;
  onReverted: (gateway: Gateway, result: ApplyResult) => void;
}

/**
 * Puts a gateway back on the version it ran before, after showing what that changes, as previewed from
 * what is recorded of the gateway. The gateway itself checks the rollback only when asked to.
 */
export default function RevertPanel({
  gateway,
  previousLabel = undefined,
  undo = false,
  targetName = undefined,
  onClose,
  onReverted,
}: RevertPanelProps): JSX.Element {
  const {t} = useTranslation('deploy');
  const {isHiddenResource} = useHiddenResources();
  const tForErrors = useGatewayErrorTranslator();
  const nameOfChange = useChangeName();
  const revert = useRevert();
  // What the gateway itself says of the rollback, once asked: a dry run of it.
  const gatewayCheck = useRevert();
  // In a project only its own applications and agents go back.
  const project = useProjectScope();
  const previewQuery = useRevertPreview(gateway.id);
  const preview: ApplyResult | undefined = previewQuery.data;
  // What the rollback itself was refused for; a failed read of the preview is shown apart from it.
  const [error, setError] = useState<Error | null>(null);
  const [isWriting, setIsWriting] = useState(false);
  // When the preview the gateway's check was asked from was read: the check holds only for that preview.
  const [checkedOn, setCheckedOn] = useState<number | undefined>(undefined);
  // When the preview whose plan the rollback was refused for was read: until a newer preview is read, the
  // rollback and the check wait, so the refused plan is not sent again.
  const [rejectedOn, setRejectedOn] = useState<number | undefined>(undefined);
  const refreshing = rejectedOn !== undefined && previewQuery.dataUpdatedAt <= rejectedOn;
  // Set once the control plane refused the rollback for removing more than its threshold allows.
  const [confirmRemovals, setConfirmRemovals] = useState(false);

  useEffect(() => {
    setError(null);
    setConfirmRemovals(false);
    setRejectedOn(undefined);
    setCheckedOn(undefined);
    gatewayCheck.reset();
    // Another gateway or project starts over; the mutation object changes identity on every render.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [gateway.id, project]);

  // The preview no longer says what the gateway holds: it is read again, and what was worked out from it
  // waits for the new one.
  const readAgain = (): void => {
    setConfirmRemovals(false);
    setRejectedOn(previewQuery.dataUpdatedAt);
    gatewayCheck.reset();
    void previewQuery.refetch();
  };
  // The gateway's check of the rollback the preview shows, bound to its plan, so it checks no other.
  const runGatewayCheck = (): void => {
    setCheckedOn(previewQuery.dataUpdatedAt);
    gatewayCheck.mutate(
      {gatewayId: gateway.id, dryRun: true, project, plan: preview?.diff.plan},
      {
        onError: (caught: Error) => {
          if (errorCodeOf(caught) === APPLIED_CHANGED) readAgain();
        },
      },
    );
  };

  const confirm = async (): Promise<void> => {
    setIsWriting(true);
    setError(null);
    try {
      // The rollback is bound to the plan the preview showed, so it changes no more than was shown.
      const plan = preview?.diff.plan;
      onReverted(
        gateway,
        await revert.mutateAsync({gatewayId: gateway.id, confirmRemovals: confirming, project, plan}),
      );
    } catch (caught: unknown) {
      if (errorCodeOf(caught) === REMOVALS_NEED_CONFIRMATION) {
        setConfirmRemovals(true);
        return;
      }
      // The gateway changed since the preview: it is read again, and the rollback waits for another click.
      if (errorCodeOf(caught) === APPLIED_CHANGED) {
        readAgain();
        return;
      }
      setError(caught instanceof Error ? caught : new Error(String(caught)));
    } finally {
      setIsWriting(false);
    }
  };

  const changes = (preview?.diff.changes ?? []).filter((change: Change) => change.change !== 'unchanged');
  // Counted as the control plane counts them for its threshold, sharing policies dropped included; a
  // resource without an id cannot be named to the gateway, so it is not a removal.
  const removals =
    preview?.removals?.count ??
    changes.filter((change: Change) => change.change === 'deleted' && Boolean(change.id)).length;
  // Above the threshold the preview gives, the rollback is confirmed from the start rather than refused first.
  const threshold = preview?.removals?.threshold ?? 0;
  const confirming = confirmRemovals || (threshold > 0 && removals > threshold);
  // The gateway's check, and its failure, hold only for the preview it was asked from.
  const checkIsCurrent =
    gatewayCheck.variables?.gatewayId === gateway.id &&
    gatewayCheck.variables.project === project &&
    checkedOn === previewQuery.dataUpdatedAt;
  // What the gateway's check says it would refuse.
  const refusedByGateway = (gatewayCheck.data?.import?.results ?? []).filter(
    (result: ImportResourceResult) => result.status === 'failed',
  );
  // The removals the control plane refuses while a resource the gateway keeps still needs them. A rollback
  // sends what its preview shows, so it waits until a preview has none.
  const removalsInUse = previewQuery.isSuccess ? (preview?.diff.removalsInUse ?? []) : [];
  const nameOfKey = (key: string): string => {
    const change = preview?.diff.changes.find((candidate: Change) => candidate.key === key);
    const [resourceType, ...rest] = key.split('/');
    return nameOfChange(change ?? {key, resourceType, id: rest.join('/')});
  };
  const canRollBack = previewQuery.isSuccess && !refreshing && removalsInUse.length === 0 && !isWriting;
  // Hidden resources are rolled back with the rest, but not shown.
  const shown = changes.filter((change: Change) => !isHiddenResource(change.resourceType, change.id));
  const header = (
    <Stack direction="row" alignItems="center" spacing={1.5}>
      <Box sx={{display: 'flex', color: 'error.main'}}>
        <Undo2 size={18} />
      </Box>
      <Typography variant="subtitle1" fontWeight={600}>
        {undo
          ? t('revertPanel.titleUndo', 'Undo the last deploy to {{target}}', {target: targetName ?? gateway.name})
          : previousLabel
            ? t('revertPanel.title', 'Roll back {{gateway}} to {{label}}', {
                gateway: gateway.name,
                label: previousLabel,
              })
            : t('revertPanel.titleUnknown', 'Roll back {{gateway}}', {gateway: gateway.name})}
      </Typography>
    </Stack>
  );
  const footer = (
    <>
      <Button
        variant="outlined"
        color="inherit"
        startIcon={gatewayCheck.isPending ? <CircularProgress size={14} color="inherit" /> : <ShieldCheck size={14} />}
        onClick={runGatewayCheck}
        disabled={!previewQuery.isSuccess || refreshing || gatewayCheck.isPending || isWriting}
        data-testid="revert-check-with-gateway"
        sx={{borderRadius: 100, mr: 'auto'}}
      >
        {t('revertPanel.checkWithGateway', 'Check with gateway')}
      </Button>
      <Button variant="text" color="inherit" onClick={onClose} disabled={isWriting}>
        {t('revertPanel.cancel', 'Cancel')}
      </Button>
      <Button
        variant="contained"
        color="error"
        startIcon={isWriting ? <CircularProgress size={14} color="inherit" /> : <Undo2 size={14} />}
        onClick={() => void confirm()}
        disabled={!canRollBack}
        sx={{borderRadius: 100}}
      >
        {confirming
          ? t('revertPanel.submitConfirm', 'Roll back and remove {{count}}', {count: removals})
          : t('revertPanel.submit', 'Roll back')}
      </Button>
    </>
  );
  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="md">
      <Box data-testid="deploy-revert-panel">
        <DialogTitle>{header}</DialogTitle>
        <DialogContent dividers>
          <Stack spacing={1.5}>
            <Typography variant="body2" data-testid="deploy-revert-what">
              {project
                ? t(
                    'revertPanel.whatProject',
                    "Only this project's applications and agents on {{gateway}} go back one step. Shared resources and other projects' resources stay as they are.",
                    {gateway: gateway.name},
                  )
                : previousLabel
                  ? t(
                      'revertPanel.what',
                      'A rollback goes back one step, to {{label}}, the version {{gateway}} ran before. To go further back, deploy an older version.',
                      {label: previousLabel, gateway: gateway.name},
                    )
                  : t(
                      'revertPanel.whatUnknown',
                      'A rollback goes back one step, to the version {{gateway}} ran before. To go further back, deploy an older version.',
                      {gateway: gateway.name},
                    )}
            </Typography>
            {!preview && !previewQuery.isError && (
              <Box sx={{display: 'flex', alignItems: 'center', gap: 1.5}}>
                <CircularProgress size={18} />
                <Typography variant="body2" color="text.secondary">
                  {t('revertPanel.checking', 'Working out what rolling back would change...')}
                </Typography>
              </Box>
            )}
            {preview && (
              <Typography variant="body2" color="text.secondary">
                {shown.length === 0
                  ? t('revertPanel.noChanges', 'Rolling back changes nothing the gateway runs.')
                  : t(
                      'revertPanel.changes',
                      'Rolling back writes {{count}} resources as the earlier version has them.',
                      {
                        count: shown.length,
                      },
                    )}
              </Typography>
            )}
            {preview?.changed && <ChangedValuesNotice changed={preview.changed} />}
            {preview && shown.length > 0 && <ChangeList changes={shown} />}
            {gatewayCheck.isError && checkIsCurrent && errorCodeOf(gatewayCheck.error) !== APPLIED_CHANGED && (
              <Alert severity="error" data-testid="revert-gateway-check-failed">
                {failureText(
                  gatewayCheck.error,
                  tForErrors,
                  t('revertPanel.gatewayCheckFailed', '{{gateway}} could not check the rollback.', {
                    gateway: gateway.name,
                  }),
                )}
              </Alert>
            )}
            {gatewayCheck.isSuccess && checkIsCurrent && (
              <Alert
                severity={refusedByGateway.length > 0 ? 'warning' : 'success'}
                data-testid="revert-gateway-checked"
              >
                {refusedByGateway.length > 0
                  ? t('revertPanel.gatewayWouldRefuse', {
                      count: refusedByGateway.length,
                      gateway: gateway.name,
                      names: refusedByGateway
                        .map(
                          (result: ImportResourceResult) =>
                            result.resourceName ?? result.resourceId ?? result.resourceType,
                        )
                        .join(', '),
                      defaultValue_one: '{{gateway}} would refuse {{count}} resource: {{names}}.',
                      defaultValue_other: '{{gateway}} would refuse {{count}} resources: {{names}}.',
                    })
                  : t('revertPanel.gatewayAccepts', '{{gateway}} checked the rollback and would refuse none of it.', {
                      gateway: gateway.name,
                    })}
              </Alert>
            )}
            {confirming && (
              <Alert severity="warning" data-testid="deploy-confirm-removals">
                {t(
                  'revertPanel.confirmRemovals',
                  'Rolling back removes {{count}} resources from {{gateway}}. Roll back again to go ahead.',
                  {count: removals, gateway: gateway.name},
                )}
              </Alert>
            )}
            {removalsInUse.length > 0 && (
              <Alert severity="warning" data-testid="revert-removals-in-use">
                {t('revertPanel.removalsInUse', {
                  count: removalsInUse.length,
                  gateway: gateway.name,
                  names: removalsInUse
                    .map((removal: RemovalInUse) =>
                      t('revertPanel.removalInUse', '{{resource}}, which {{holder}} needs', {
                        resource: nameOfKey(removal.key),
                        holder: nameOfKey(removal.neededBy),
                      }),
                    )
                    .join('; '),
                  defaultValue_one:
                    '{{gateway}} keeps a resource that still needs what rolling back removes: {{names}}. The rollback waits until it no longer does.',
                  defaultValue_other:
                    '{{gateway}} keeps resources that still need what rolling back removes: {{names}}. The rollback waits until they no longer do.',
                })}
              </Alert>
            )}
            {rejectedOn !== undefined && (
              <Alert severity="info" data-testid="revert-preview-changed">
                {t(
                  'revertPanel.previewChanged',
                  '{{gateway}} changed since this rollback was previewed, so the preview is read again. Check what it changes, then roll back again.',
                  {gateway: gateway.name},
                )}
              </Alert>
            )}
            {previewQuery.isError && (
              <QueryErrorNotice
                error={previewQuery.error}
                t={tForErrors}
                variant="inline"
                fallbackKey="deploy:revertPanel.previewFailed"
                fallbackDefaultValue={t(
                  'revertPanel.previewFailed',
                  'What rolling back would change could not be read.',
                )}
                resolveErrorMessage={(failure, translate, fallbackKey, fallbackDefaultValue) =>
                  isForbidden(failure)
                    ? t('revertPanel.forbidden', 'Your account may not roll this gateway back.')
                    : failureText(failure, translate, translate(fallbackKey, {defaultValue: fallbackDefaultValue}))
                }
                onRetry={() => void previewQuery.refetch()}
              />
            )}
            {error && (
              <Alert severity="error" data-testid="revert-failed">
                {isForbidden(error)
                  ? t('revertPanel.forbidden', 'Your account may not roll this gateway back.')
                  : failureText(error, tForErrors, t('revertPanel.failed', 'The gateway could not be rolled back.'))}
              </Alert>
            )}
          </Stack>
        </DialogContent>
        <DialogActions>{footer}</DialogActions>
      </Box>
    </Dialog>
  );
}
