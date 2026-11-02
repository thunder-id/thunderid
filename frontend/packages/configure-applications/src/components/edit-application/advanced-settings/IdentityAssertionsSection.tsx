// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {useAuthorizationServerMetadata} from '@thunderid/configure-connections';
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
import type {IDJAGConfig, OAuth2Config, OAuth2Token} from '../../../models/oauth';
import {OAuth2GrantTypes, TokenEndpointAuthMethods} from '../../../models/oauth';
import {applyGrantTypesChange, isIdJagIssuanceUnavailable} from '../../../utils/oauth2Rules';

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
 * with an explanatory tooltip while the application is a public client. It also needs the runtime
 * to serve the token exchange grant: when its authorization server metadata shows it does not,
 * ID-JAG cannot be turned on, a client that already has it on can still turn it off (which also
 * drops the grant the runtime would reject on save), and the card says why.
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
  const {data: serverMetadata} = useAuthorizationServerMetadata();
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
  const isIssuanceUnavailable = isIdJagIssuanceUnavailable(serverMetadata?.grant_types_supported);
  const isToggleDisabled = disabled || isPublicClient || (isIssuanceUnavailable && !isEnabled);
  const issuanceUnavailableHint = t(
    'applications:edit.advanced.idJag.tokenExchangeUnavailable',
    'This server does not allow the token exchange grant, so identity assertions cannot be issued.',
  );
  let toggleGuard: string | undefined;
  if (isPublicClient) {
    toggleGuard = oauth2Config.clientIdMetadataDocument
      ? t(
          'applications:edit.advanced.idJag.metadataDocumentPublicClientGuard',
          "Identity assertions require a confidential client. This client's metadata document declares it as a public client.",
        )
      : t(
          'applications:edit.advanced.idJag.publicClientGuard',
          'Identity assertions require a confidential client. Turn off public client to enable.',
        );
  } else if (isIssuanceUnavailable) {
    toggleGuard = issuanceUnavailableHint;
  }

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
    // A metadata document client holds the token exchange grant only for identity assertions. Any
    // client drops it while the runtime does not serve it, since the runtime rejects a client that
    // still holds it and the grants picker cannot remove a grant the runtime does not list.
    if (
      !checked &&
      (oauth2Config.clientIdMetadataDocument || isIssuanceUnavailable) &&
      grantTypes.includes(OAuth2GrantTypes.TOKEN_EXCHANGE)
    ) {
      onTokenConfigChange(
        tokenUpdates,
        applyGrantTypesChange(
          oauth2Config,
          grantTypes.filter((grant) => grant !== OAuth2GrantTypes.TOKEN_EXCHANGE),
        ),
      );
      return;
    }

    onTokenConfigChange(tokenUpdates);
  };

  const handleAudiencesChange = (newValue: string[]) => {
    onTokenConfigChange({idJag: {...currentIdJag, allowedAudiences: newValue}});
  };

  const handleValidityPeriodChange = (value: string) => {
    const parsed = Number(value);
    if (value.trim() === '' || Number.isNaN(parsed) || parsed < 1) {
      setInvalidValidityPeriodInput(value);
      return;
    }

    setInvalidValidityPeriodInput(null);
    onTokenConfigChange({idJag: {...currentIdJag, validityPeriod: parsed}});
  };

  const toggleLabel = t('applications:edit.advanced.idJag.title', 'Identity assertions (ID-JAG)');
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
        toggleGuard ? (
          <Tooltip title={toggleGuard}>
            <span>{toggleSwitch}</span>
          </Tooltip>
        ) : (
          toggleSwitch
        )
      }
    >
      {(isEnabled || isIssuanceUnavailable) && (
        <Stack spacing={3}>
          {isEnabled && (
            <>
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
                      ? t(
                          'applications:edit.advanced.idJag.validityPeriod.error',
                          'Enter a value of at least 1 second.',
                        )
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
            </>
          )}

          <Typography variant="caption" color="text.secondary">
            {isIssuanceUnavailable
              ? issuanceUnavailableHint
              : t(
                  'applications:edit.advanced.idJag.grantTypeHint',
                  'The token exchange grant type is enabled together with this feature.',
                )}
          </Typography>
        </Stack>
      )}
    </SettingsCard>
  );
}
