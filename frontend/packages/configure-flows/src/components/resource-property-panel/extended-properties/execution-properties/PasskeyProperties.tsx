// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FormHelperText, FormLabel, MenuItem, Select, Stack, Typography} from '@wso2/oxygen-ui';
import {useMemo, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {PASSKEY_MODES, PASSKEY_MODES_WITH_RELYING_PARTY} from './constants';
import DraftTextField from './DraftTextField';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

function PasskeyProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const currentMode = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.action?.executor as {mode?: string})?.mode ?? '';
  }, [resource]);

  const currentRelyingPartyId = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties as {relyingPartyId?: string})?.relyingPartyId ?? '';
  }, [resource]);

  const currentRelyingPartyName = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.properties as {relyingPartyName?: string})?.relyingPartyName ?? '';
  }, [resource]);

  const showRelyingPartyConfig = PASSKEY_MODES_WITH_RELYING_PARTY.includes(
    currentMode as (typeof PASSKEY_MODES_WITH_RELYING_PARTY)[number],
  );

  const handlePasskeyModeChange = (selectedMode: string): void => {
    const modeConfig = PASSKEY_MODES.find((mode) => mode.value === selectedMode);

    const updatedData = {
      ...((resource?.data as StepData) ?? {}),
      action: {
        ...((resource?.data as StepData)?.action ?? {}),
        executor: {
          ...((resource?.data as StepData)?.action?.executor ?? {}),
          mode: selectedMode,
        },
      },
      display: {
        ...((resource?.data as StepData)?.['display'] ?? {}),
        label: modeConfig?.displayLabel ?? 'Passkey',
      },
    };

    onChange('data', updatedData, resource);
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.passkey.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="passkey-mode-select">{t('flows:core.executions.passkey.mode.label')}</FormLabel>
        <Select
          id="passkey-mode-select"
          value={currentMode}
          onChange={(e) => handlePasskeyModeChange(e.target.value)}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {t('flows:core.executions.passkey.mode.placeholder')}
          </MenuItem>
          {PASSKEY_MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>
              {t(mode.translationKey)}
            </MenuItem>
          ))}
        </Select>
      </div>

      {showRelyingPartyConfig && (
        <>
          <div>
            <FormLabel htmlFor="relying-party-id">{t('flows:core.executions.passkey.relyingPartyId.label')}</FormLabel>
            <DraftTextField
              id="relying-party-id"
              value={currentRelyingPartyId}
              onCommit={(value) => onChange('data.properties.relyingPartyId', value, resource)}
              placeholder={t('flows:core.executions.passkey.relyingPartyId.placeholder')}
              fullWidth
              size="small"
            />
            <FormHelperText>{t('flows:core.executions.passkey.relyingPartyId.hint')}</FormHelperText>
          </div>

          <div>
            <FormLabel htmlFor="relying-party-name">
              {t('flows:core.executions.passkey.relyingPartyName.label')}
            </FormLabel>
            <DraftTextField
              id="relying-party-name"
              value={currentRelyingPartyName}
              onCommit={(value) => onChange('data.properties.relyingPartyName', value, resource)}
              placeholder={t('flows:core.executions.passkey.relyingPartyName.placeholder')}
              fullWidth
              size="small"
            />
            <FormHelperText>{t('flows:core.executions.passkey.relyingPartyName.hint')}</FormHelperText>
          </div>
        </>
      )}
    </Stack>
  );
}

export default PasskeyProperties;
