/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {SettingsCard} from '@thunderid/components';
import {
  Autocomplete,
  Chip,
  FormControl,
  FormLabel,
  InputAdornment,
  Stack,
  Switch,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {
  OAuth2GrantTypes,
  TokenEndpointAuthMethods,
  type IDJAGConfig,
  type OAuth2Config,
  type OAuth2Token,
} from '../../../models/oauth';
import {applyGrantTypesChange} from '../../../utils/oauth2Rules';

const DEFAULT_VALIDITY_PERIOD = 300;

/**
 * Props for the {@link IdentityAssertionsSection} component.
 */
interface IdentityAssertionsSectionProps {
  /**
   * OAuth2 configuration to display
   */
  oauth2Config?: OAuth2Config;
  /**
   * Callback to handle changes to the token configuration's ID-JAG settings. Accepts an optional
   * `oauth2Updates` argument so that enabling ID-JAG (which also adds the token exchange grant
   * type) can be applied as a single combined update instead of two separate ones, avoiding a
   * lost update when both are derived from the same stale snapshot.
   */
  onTokenConfigChange: (tokenUpdates: Partial<OAuth2Token>, oauth2Updates?: Partial<OAuth2Config>) => void;
  /**
   * Whether inputs should be disabled (e.g. read-only resource).
   */
  disabled?: boolean;
  /**
   * Callback to report whether this section currently has validation errors (feeds the Save bar).
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

/**
 * Section component for configuring Identity Assertion Authorization Grant (ID-JAG) issuance.
 *
 * Lets admins enable signed identity assertions for an application, and configure the audiences
 * an issued assertion may target and its validity period. Enabling ID-JAG also adds the token
 * exchange grant type. ID-JAG issuance requires a confidential client, so the toggle is disabled
 * with an explanatory tooltip while the application is a public client.
 *
 * @param props - Component props
 * @returns Identity assertions configuration UI within a SettingsCard, or null
 */
export default function IdentityAssertionsSection({
  oauth2Config = undefined,
  onTokenConfigChange,
  disabled = false,
  onValidationChange = undefined,
}: IdentityAssertionsSectionProps) {
  const {t} = useTranslation();
  const rawIdJag = oauth2Config?.token?.idJag;
  const currentIdJag: IDJAGConfig = rawIdJag ?? {enabled: false};
  const [invalidValidityPeriodInput, setInvalidValidityPeriodInput] = useState<string | null>(null);
  const [reconciledIdJag, setReconciledIdJag] = useState<IDJAGConfig | undefined>(rawIdJag);

  // Reconciles the locally-held invalid input with the external idJag config whenever the latter
  // is replaced (e.g. on discard), so a rejected value doesn't linger after the page goes clean.
  // Adjusted during render (rather than in an effect) per React's guidance for resetting state
  // when a prop changes.
  if (rawIdJag !== reconciledIdJag) {
    setReconciledIdJag(rawIdJag);
    setInvalidValidityPeriodInput(null);
  }

  // Computed ahead of the `!oauth2Config` early return (and reported via an effect below) so a
  // previously-reported error is cleared when the section stops rendering its fields, either
  // because ID-JAG is toggled off or this section is unmounted entirely (oauth2Config removed).
  const isEnabled = currentIdJag.enabled;
  const allowedAudiences = currentIdJag.allowedAudiences ?? [];
  const showValidityPeriodError = invalidValidityPeriodInput !== null;
  const showAudiencesError = isEnabled && allowedAudiences.length === 0;

  useEffect(() => {
    onValidationChange?.(showAudiencesError || showValidityPeriodError);
  }, [showAudiencesError, showValidityPeriodError, onValidationChange]);

  if (!oauth2Config) return null;

  const validityPeriod = currentIdJag.validityPeriod ?? DEFAULT_VALIDITY_PERIOD;
  const validityPeriodValue = invalidValidityPeriodInput ?? validityPeriod;
  const isPublicClient =
    oauth2Config.publicClient === true || oauth2Config.tokenEndpointAuthMethod === TokenEndpointAuthMethods.NONE;
  const isToggleDisabled = disabled || isPublicClient;

  const handleToggle = (checked: boolean) => {
    setInvalidValidityPeriodInput(null);

    const tokenUpdates: Partial<OAuth2Token> = {
      idJag: checked
        ? {
            enabled: true,
            allowedAudiences: currentIdJag.allowedAudiences ?? [],
            validityPeriod: currentIdJag.validityPeriod ?? DEFAULT_VALIDITY_PERIOD,
          }
        : {
            enabled: false,
            allowedAudiences: currentIdJag.allowedAudiences,
            validityPeriod: currentIdJag.validityPeriod,
          },
    };

    const grantTypes = oauth2Config.grantTypes ?? [];
    if (checked && !grantTypes.includes(OAuth2GrantTypes.TOKEN_EXCHANGE)) {
      // Combine both updates into a single call so they're applied against the same snapshot
      // instead of the second one clobbering the first (see EditAdvancedSettings.handleTokenConfigChange).
      onTokenConfigChange(
        tokenUpdates,
        applyGrantTypesChange(oauth2Config, [...grantTypes, OAuth2GrantTypes.TOKEN_EXCHANGE]),
      );
      return;
    }

    onTokenConfigChange(tokenUpdates);
  };

  const handleAudiencesChange = (newValue: string[]) => {
    onTokenConfigChange({idJag: {...currentIdJag, allowedAudiences: newValue}});
  };

  const handleValidityPeriodChange = (value: string) => {
    if (value === '') {
      setInvalidValidityPeriodInput(null);
      onTokenConfigChange({idJag: {...currentIdJag, validityPeriod: undefined}});
      return;
    }

    const parsed = Number(value);
    if (Number.isNaN(parsed) || parsed < 1) {
      setInvalidValidityPeriodInput(value);
      return;
    }

    setInvalidValidityPeriodInput(null);
    onTokenConfigChange({idJag: {...currentIdJag, validityPeriod: parsed}});
  };

  const toggleLabel = t('applications:edit.advanced.idJag.title', 'Identity Assertions (ID-JAG)');
  const toggleSwitch = (
    <Switch
      checked={isEnabled}
      disabled={isToggleDisabled}
      onChange={(e) => handleToggle(e.target.checked)}
      slotProps={{input: {'aria-label': toggleLabel, role: 'switch'}}}
    />
  );

  return (
    <SettingsCard
      title={toggleLabel}
      description={t(
        'applications:edit.advanced.idJag.description',
        "Issue signed assertions of the signed-in user's identity that external services accept for token issuance.",
      )}
      headerAction={
        isPublicClient ? (
          <Tooltip
            title={t(
              'applications:edit.advanced.idJag.publicClientGuard',
              'Identity assertions require a confidential client. Turn off Public Client to enable.',
            )}
          >
            <span>{toggleSwitch}</span>
          </Tooltip>
        ) : (
          toggleSwitch
        )
      }
    >
      {isEnabled && (
        <Stack spacing={3}>
          <FormControl fullWidth size="small" error={showAudiencesError}>
            <FormLabel htmlFor="idjag_allowed_audiences">
              {t('applications:edit.advanced.idJag.labels.allowedAudiences', 'Allowed audiences')} *
            </FormLabel>
            <Autocomplete
              id="idjag_allowed_audiences"
              multiple
              freeSolo
              fullWidth
              options={[]}
              value={allowedAudiences}
              disabled={disabled}
              onChange={(_event, newValue) => handleAudiencesChange(newValue as string[])}
              renderTags={(value, getTagProps) =>
                value.map((option, index) => <Chip label={option} {...getTagProps({index})} key={option} />)
              }
              renderInput={(params) => (
                <TextField
                  {...params}
                  placeholder={t(
                    'applications:edit.advanced.idJag.allowedAudiences.placeholder',
                    'Type an audience and press Enter',
                  )}
                  error={showAudiencesError}
                  helperText={
                    showAudiencesError
                      ? t('applications:edit.advanced.idJag.allowedAudiences.error', 'Add at least one audience.')
                      : t(
                          'applications:edit.advanced.idJag.allowedAudiences.hint',
                          'Each assertion targets exactly one of these audiences.',
                        )
                  }
                />
              )}
            />
          </FormControl>

          <FormControl size="small" error={showValidityPeriodError}>
            <FormLabel htmlFor="idjag_validity_period">
              {t('applications:edit.advanced.idJag.labels.validityPeriod', 'Assertion validity')}
            </FormLabel>
            <TextField
              id="idjag_validity_period"
              type="number"
              size="small"
              disabled={disabled}
              value={validityPeriodValue}
              error={showValidityPeriodError}
              helperText={
                showValidityPeriodError
                  ? t('applications:edit.advanced.idJag.validityPeriod.error', 'Enter a value of at least 1 second.')
                  : undefined
              }
              onChange={(e) => handleValidityPeriodChange(e.target.value)}
              inputProps={{min: 1}}
              InputProps={{
                endAdornment: (
                  <InputAdornment position="end">
                    {t('applications:edit.advanced.idJag.labels.seconds', 'seconds')}
                  </InputAdornment>
                ),
              }}
              sx={{maxWidth: 220}}
            />
            {!showValidityPeriodError && (
              <Typography variant="caption" color="text.secondary" sx={{mt: 0.5}}>
                {t(
                  'applications:edit.advanced.idJag.validityPeriod.hint',
                  'How long an issued assertion stays valid. Default 300.',
                )}
              </Typography>
            )}
          </FormControl>

          <Typography variant="caption" color="text.secondary">
            {t(
              'applications:edit.advanced.idJag.grantTypeHint',
              'The token exchange grant type is enabled together with this feature.',
            )}
          </Typography>
        </Stack>
      )}
    </SettingsCard>
  );
}
