// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {UseQueryResult} from '@tanstack/react-query';
import {Alert, Box, Button, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import MissingValuesNotice from './MissingValuesNotice';
import SetMissingValuesForm from './SetMissingValuesForm';
import {GatewayDetailTabs, type GatewayValuesTab} from '../constants/gateway-values';
import type {ApplyResult} from '../models/gateway';
import {hasMissingValues} from '../utils/missingValues';

export interface MissingValuesCheckProps {
  gatewayId: string;
  /** The dry run that checks the gateway for the values a version needs. */
  check: UseQueryResult<ApplyResult>;
  /** Opens the gateway's variables or secrets tab. */
  onManage: (tab: GatewayValuesTab) => void;
}

function Checking({label}: {label: string}): JSX.Element {
  return (
    <Stack direction="row" spacing={1} alignItems="center">
      <CircularProgress size={16} />
      <Typography variant="body2" color="text.secondary">
        {label}
      </Typography>
    </Stack>
  );
}

/**
 * The outcome of checking a gateway for the variables and secrets a version needs: in progress,
 * not possible, or the names it lacks, with a way to set them in place. Nothing is shown when the
 * gateway holds them all.
 */
export default function MissingValuesCheck({gatewayId, check, onManage}: MissingValuesCheckProps): JSX.Element | null {
  const {t} = useTranslation();
  const [showForm, setShowForm] = useState(false);

  if (check.isLoading) {
    return <Checking label={t('gateways:missing.checking', 'Checking that the gateway holds the values needed...')} />;
  }

  if (check.error && !check.data) {
    return (
      <Alert severity="info">
        {t('gateways:missing.checkError', 'Could not check whether the gateway holds the values needed.')}
      </Alert>
    );
  }

  const missing = check.data?.missing;
  if (!hasMissingValues(missing)) {
    return check.isFetching ? (
      <Checking label={t('gateways:missing.checking', 'Checking that the gateway holds the values needed...')} />
    ) : null;
  }

  return (
    <Stack spacing={2}>
      <MissingValuesNotice
        missing={missing}
        onManageVariables={() => onManage(GatewayDetailTabs.VARIABLES)}
        onManageSecrets={() => onManage(GatewayDetailTabs.SECRETS)}
      />
      {showForm ? (
        <SetMissingValuesForm gatewayId={gatewayId} missing={missing} />
      ) : (
        <Box>
          <Button variant="outlined" onClick={() => setShowForm(true)}>
            {t('gateways:missing.setValues', 'Set values')}
          </Button>
        </Box>
      )}
      {check.isFetching && <Checking label={t('gateways:missing.rechecking', 'Checking the gateway again...')} />}
    </Stack>
  );
}
