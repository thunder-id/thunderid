// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryErrorNotice, SettingsCard} from '@thunderid/components';
import {Box, Button, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {Rocket, Undo2} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ApplyVersionDialog from './ApplyVersionDialog';
import RevertDialog from './RevertDialog';
import useGetAppliedVersion from '../api/useGetAppliedVersion';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import formatTimestamp from '../utils/formatTimestamp';

export interface DeployedConfigurationCardProps {
  gatewayId: string;
  gatewayName: string;
}

function Field({label, value}: {label: string; value: string}): JSX.Element {
  return (
    <Box sx={{minWidth: 160}}>
      <Typography variant="caption" color="text.secondary" component="div">
        {label}
      </Typography>
      <Typography variant="body1" sx={{fontWeight: 500}}>
        {value}
      </Typography>
    </Box>
  );
}

/**
 * What a gateway holds, and the actions that move it: apply a version, or revert to the one
 * it held before.
 */
export default function DeployedConfigurationCard({
  gatewayId,
  gatewayName,
}: DeployedConfigurationCardProps): JSX.Element {
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const {data, isLoading, error, refetch} = useGetAppliedVersion(gatewayId);
  const [applyOpen, setApplyOpen] = useState(false);
  // Held from the moment the dialog opens, so its title does not move once the revert lands.
  const [revertTarget, setRevertTarget] = useState<number | null>(null);

  const versionLabel = (version?: number): string =>
    version === undefined
      ? t('gateways:applied.notApplied', 'Not applied')
      : t('gateways:versions.versionNumber', 'Version {{version}}', {version});

  const previousVersion = data?.previousVersion;

  return (
    <SettingsCard
      title={t('gateways:deployed.title', 'Deployed configuration')}
      description={t('gateways:deployed.description', 'The configuration version this gateway holds.')}
      headerAction={
        <Stack direction="row" spacing={1}>
          {previousVersion !== undefined && (
            <Button
              variant="outlined"
              color="warning"
              startIcon={<Undo2 size={16} />}
              onClick={() => setRevertTarget(previousVersion)}
            >
              {t('gateways:revert.action', 'Revert to version {{version}}', {version: previousVersion})}
            </Button>
          )}
          <Button variant="contained" startIcon={<Rocket size={16} />} onClick={() => setApplyOpen(true)}>
            {t('gateways:apply.action', 'Apply a version')}
          </Button>
        </Stack>
      }
    >
      {isLoading && (
        <Box sx={{display: 'flex', justifyContent: 'center', py: 2}}>
          <CircularProgress size={24} />
        </Box>
      )}
      {error && (
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="inline"
          fallbackKey="deployed.error"
          fallbackDefaultValue="Failed to load the deployed configuration"
          onRetry={() => void refetch()}
        />
      )}
      {data && (
        <Stack direction="row" spacing={4} flexWrap="wrap" useFlexGap>
          <Field label={t('gateways:deployed.applied', 'Applied version')} value={versionLabel(data.appliedVersion)} />
          <Field label={t('gateways:deployed.appliedAt', 'Applied')} value={formatTimestamp(data.appliedAt)} />
          <Field
            label={t('gateways:deployed.previous', 'Previous version')}
            value={
              previousVersion === undefined ? t('gateways:deployed.noPrevious', 'None') : versionLabel(previousVersion)
            }
          />
        </Stack>
      )}

      <ApplyVersionDialog
        open={applyOpen}
        gatewayId={gatewayId}
        gatewayName={gatewayName}
        onClose={() => setApplyOpen(false)}
      />
      {revertTarget !== null && (
        <RevertDialog
          open
          gatewayId={gatewayId}
          gatewayName={gatewayName}
          previousVersion={revertTarget}
          onClose={() => setRevertTarget(null)}
        />
      )}
    </SettingsCard>
  );
}
