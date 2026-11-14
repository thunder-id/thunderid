// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetUserTypeAttributes, type AggregatedUserTypeAttribute} from '@thunderid/configure-user-types';
import {
  Autocomplete,
  FormHelperText,
  FormLabel,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
  type AutocompleteRenderInputParams,
} from '@wso2/oxygen-ui';
import {useMemo, useState, type HTMLAttributes, type ReactNode, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import CheckboxWithHint from './CheckboxWithHint';
import {IDENTIFYING_MODES} from './constants';
import type {CommonResourcePropertiesPropsInterface} from './types';
import type {StepData} from '../../../../models/steps';

const DEFAULT_LOGIN_HINT_ATTRIBUTE = 'username';

function getAttributeName(option: string | AggregatedUserTypeAttribute): string {
  return typeof option === 'string' ? option : option.attribute;
}

function IdentifyingProperties({resource, onChange}: CommonResourcePropertiesPropsInterface): ReactNode {
  const {t} = useTranslation();

  const currentMode = useMemo(() => {
    const stepData = resource?.data as StepData | undefined;
    return (stepData?.action?.executor as {mode?: string})?.mode ?? '';
  }, [resource]);

  const properties = useMemo(() => (resource?.data as StepData | undefined)?.properties ?? {}, [resource]);

  // The user type is not known while the flow is authored, so attributes from every user type are
  // suggested. Only unique ones are offered, since identify fails when more than one user matches.
  const {attributes: allAttributes, isLoading: isLoadingAttributes} = useGetUserTypeAttributes();
  const attributeOptions = useMemo(
    () => allAttributes.filter((attribute) => attribute.unique && !attribute.credential),
    [allAttributes],
  );

  const loginHintAttribute =
    typeof properties['loginHintAttribute'] === 'string' ? properties['loginHintAttribute'] : '';
  const isCibaLoginHint = loginHintAttribute !== '';
  const isIdentifyMode = currentMode === '' || currentMode === 'identify';

  // Typed text stays local until blur or Enter, like DraftTextField, and is kept apart from the
  // Autocomplete value so MUI keeps filtering the suggestions while typing.
  const [loginHintInput, setLoginHintInput] = useState<string>(loginHintAttribute);
  const [syncedLoginHintAttribute, setSyncedLoginHintAttribute] = useState<string>(loginHintAttribute);
  if (loginHintAttribute !== syncedLoginHintAttribute) {
    setSyncedLoginHintAttribute(loginHintAttribute);
    setLoginHintInput(loginHintAttribute);
  }

  const handleBooleanPropertyChange = (propertyName: string, value: boolean): void => {
    onChange(`data.properties.${propertyName}`, value, resource);
  };

  // A CIBA login hint is on while an attribute is set, so it cannot be cleared from the field; an empty
  // value restores the attribute and only the checkbox turns it off.
  const commitLoginHintAttribute = (value: string): void => {
    const attribute = value.trim();
    if (attribute === '' || attribute === loginHintAttribute) {
      setLoginHintInput(loginHintAttribute);
      return;
    }
    setLoginHintInput(attribute);
    onChange('data.properties.loginHintAttribute', attribute, resource);
  };

  // The panel's change handler saves a missing value as '', which the executor reads as not set.
  const handleCibaLoginHintChange = (checked: boolean): void => {
    onChange('data.properties.loginHintAttribute', checked ? DEFAULT_LOGIN_HINT_ATTRIBUTE : '', resource);
  };

  const handleIdentifyingModeChange = (selectedMode: string): void => {
    const modeConfig = IDENTIFYING_MODES.find((mode) => mode.value === selectedMode);

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
        label: modeConfig?.displayLabel ?? 'Identify User',
      },
    };

    onChange('data', updatedData, resource);
  };

  return (
    <Stack gap={2}>
      <Typography variant="body2" color="text.secondary">
        {t('flows:core.executions.identifying.description')}
      </Typography>

      <div>
        <FormLabel htmlFor="identifying-mode-select">{t('flows:core.executions.identifying.mode.label')}</FormLabel>
        <Select
          id="identifying-mode-select"
          value={currentMode}
          onChange={(e) => handleIdentifyingModeChange(e.target.value)}
          displayEmpty
          fullWidth
        >
          <MenuItem value="" disabled>
            {t('flows:core.executions.identifying.mode.placeholder')}
          </MenuItem>
          {IDENTIFYING_MODES.map((mode) => (
            <MenuItem key={mode.value} value={mode.value}>
              {t(mode.translationKey)}
            </MenuItem>
          ))}
        </Select>
      </div>

      {isIdentifyMode && (
        <>
          <CheckboxWithHint
            checked={properties['boundToUserType'] === true}
            label={t('flows:core.executions.identifying.boundToUserType.label', 'Bound to the user type')}
            hint={t(
              'flows:core.executions.identifying.boundToUserType.hint',
              'Accept only a user of the user type the flow resolved. A user of another type fails the step.',
            )}
            onChange={(checked) => handleBooleanPropertyChange('boundToUserType', checked)}
          />
          <CheckboxWithHint
            checked={properties['continueOnNoMatch'] === true}
            label={t('flows:core.executions.identifying.continueOnNoMatch.label', 'Continue when no user matches')}
            hint={t(
              'flows:core.executions.identifying.continueOnNoMatch.hint',
              'Follow the incomplete path without an error when no user matches, keeping the entered value.',
            )}
            onChange={(checked) => handleBooleanPropertyChange('continueOnNoMatch', checked)}
          />
        </>
      )}

      {/* Offered in identify mode, and in any mode where it is already on so it can be turned off. */}
      {(isIdentifyMode || isCibaLoginHint) && (
        <CheckboxWithHint
          checked={isCibaLoginHint}
          label={t('flows:core.executions.identifying.cibaLoginHint.label', 'Identify from a CIBA login hint')}
          hint={t(
            'flows:core.executions.identifying.cibaLoginHint.hint',
            'For backchannel (CIBA) sign-in. The user is matched by the login hint the client sends instead of the step inputs.',
          )}
          onChange={handleCibaLoginHintChange}
        />
      )}

      {isCibaLoginHint && (
        <div>
          <Autocomplete
            id="login-hint-attribute"
            freeSolo
            disablePortal
            disableClearable
            options={attributeOptions}
            loading={isLoadingAttributes}
            getOptionLabel={getAttributeName}
            renderOption={(
              props: HTMLAttributes<HTMLLIElement> & {key?: string},
              option: string | AggregatedUserTypeAttribute,
            ) => (
              <li {...props} key={getAttributeName(option)}>
                <Stack>
                  <Typography variant="body2">{getAttributeName(option)}</Typography>
                  {typeof option !== 'string' && (
                    <Typography variant="caption" color="text.secondary">
                      {option.userTypes.join(', ')}
                    </Typography>
                  )}
                </Stack>
              </li>
            )}
            renderInput={(params: AutocompleteRenderInputParams) => (
              <>
                <FormLabel htmlFor="login-hint-attribute">
                  {t('flows:core.executions.identifying.loginHintAttribute.label', 'Login hint attribute')}
                </FormLabel>
                <TextField
                  {...params}
                  size="small"
                  placeholder={t('flows:core.executions.identifying.loginHintAttribute.placeholder', 'username')}
                />
              </>
            )}
            value=""
            inputValue={loginHintInput}
            onChange={(_: SyntheticEvent, option: string | AggregatedUserTypeAttribute) =>
              commitLoginHintAttribute(getAttributeName(option))
            }
            onInputChange={(_: SyntheticEvent, value: string, reason: string) => {
              if (reason === 'input') {
                setLoginHintInput(value);
              }
            }}
            onBlur={() => commitLoginHintAttribute(loginHintInput)}
          />
          <FormHelperText>
            {t(
              'flows:core.executions.identifying.loginHintAttribute.hint',
              'The unique user attribute the login hint is matched against, such as username.',
            )}
          </FormHelperText>
        </div>
      )}
    </Stack>
  );
}

export default IdentifyingProperties;
