// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useLogger} from '@thunderid/logger/react';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Dialog,
  DialogActions,
  DialogContent,
  DialogContentText,
  DialogTitle,
  FormControlLabel,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ApplyResultSummary from './ApplyResultSummary';
import MissingValuesCheck from './MissingValuesCheck';
import useRevertDryRun from '../api/useRevertDryRun';
import useRevertGateway from '../api/useRevertGateway';
import type {GatewayValuesTab} from '../constants/gateway-values';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import useOpenGatewayTab from '../hooks/useOpenGatewayTab';
import type {ApplyResult} from '../models/gateway';
import getApplyErrorMessage from '../utils/getApplyErrorMessage';
import {hasMissingValues, isMissingValuesError} from '../utils/missingValues';

export interface RevertDialogProps {
  open: boolean;
  gatewayId: string;
  gatewayName: string;
  /** The version a revert returns the gateway to. */
  previousVersion: number;
  onClose: () => void;
}

/**
 * Confirms returning a gateway to the version it held before, then shows the gateway's account. A
 * dry run checks first that the gateway holds every variable and secret that version refers to,
 * and the revert waits until it does.
 */
export default function RevertDialog({
  open,
  gatewayId,
  gatewayName,
  previousVersion,
  onClose,
}: RevertDialogProps): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const logger = useLogger('RevertDialog');
  const revert = useRevertGateway();
  const [dryRun, setDryRun] = useState(false);
  const [result, setResult] = useState<ApplyResult | null>(null);
  const openGatewayTab = useOpenGatewayTab();
  const check = useRevertDryRun(gatewayId, open && result === null);
  const isMissingValues = hasMissingValues(check.data?.missing);

  const handleClose = (): void => {
    if (revert.isPending) return;
    setDryRun(false);
    setResult(null);
    revert.reset();
    onClose();
  };

  const handleManage = (tab: GatewayValuesTab): void => {
    if (revert.isPending) return;
    handleClose();
    openGatewayTab(gatewayId, tab);
  };

  const handleRevert = (): void => {
    revert.mutate(
      {gatewayId, data: {dryRun}},
      {
        onSuccess: (reverted: ApplyResult) => {
          setResult(reverted);
        },
        onError: (err: Error) => {
          logger.error('Failed to revert gateway', {error: err});
          // The gateway lost a value since the check ran, so check again to name it.
          if (isMissingValuesError(err)) void check.refetch();
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {t('gateways:revert.title', 'Revert to version {{version}}', {version: previousVersion})}
      </DialogTitle>
      <DialogContent>
        {result ? (
          <ApplyResultSummary result={result} onManage={handleManage} />
        ) : (
          <>
            <DialogContentText>
              {t(
                'gateways:revert.message',
                'Return {{name}} to version {{version}}, the version it held before? Resources that version does not have are removed from the gateway.',
                {name: gatewayName, version: previousVersion},
              )}
            </DialogContentText>
            <Alert severity="info" sx={{mt: 2}}>
              {t(
                'gateways:revert.disclaimer',
                'The version reverted from becomes the revert target, so a revert can itself be undone.',
              )}
            </Alert>
            <Box sx={{mt: 2, '&:empty': {display: 'none'}}}>
              <MissingValuesCheck gatewayId={gatewayId} check={check} onManage={handleManage} />
            </Box>
            <FormControlLabel
              sx={{mt: 1}}
              control={<Checkbox checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} />}
              label={t('gateways:revert.dryRun', 'Dry run: check the import on the gateway without changing it')}
            />
          </>
        )}
        {revert.error && (
          <Alert severity="error" sx={{mt: 2}}>
            {getApplyErrorMessage(revert.error, tForErrors)}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        {result ? (
          <Button variant="contained" onClick={handleClose}>
            {t('gateways:apply.done', 'Done')}
          </Button>
        ) : (
          <>
            <Button variant="outlined" onClick={handleClose} disabled={revert.isPending}>
              {t('common:actions.cancel', 'Cancel')}
            </Button>
            <Button
              variant="contained"
              color="warning"
              onClick={handleRevert}
              disabled={revert.isPending || check.isFetching || isMissingValues}
            >
              {revert.isPending
                ? t('gateways:revert.submitting', 'Reverting...')
                : t('gateways:revert.submit', 'Revert')}
            </Button>
          </>
        )}
      </DialogActions>
    </Dialog>
  );
}
