// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import type {InboundAuthConfig, OAuth2Config} from '@thunderid/configure-applications';
import {
  Alert,
  Card,
  CardContent,
  Chip,
  FormControl,
  FormLabel,
  Grid,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {useEffect, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import JwtPreview from './JwtPreview';
import TokenConstants from '../../../constants/token-constants';

// The client_credentials grant carries no scopes, so `scope` never appears on this token.
const ACCESS_TOKEN_DEFAULT_CLAIMS = TokenConstants.DEFAULT_TOKEN_ATTRIBUTES.filter((attr) => attr !== 'scope');
const SUB_TYPE_CLAIM = 'sub_type';

/** Display copy for the section, supplied by the caller so agents and applications read differently. */
export interface ClientAccessTokenCopy {
  attributesTitle: string;
  attributesDescription: string;
  attributesLabel: string;
  attributesHint: string;
  attributesEmpty: string;
  validityTitle: string;
  validityDescription: string;
  validityLabel: string;
  validityHint: string;
  validityError: string;
}

interface ClientAccessTokenSectionProps {
  oauth2Config?: OAuth2Config;
  inboundAuthConfig?: InboundAuthConfig[];
  onFieldChange: (field: 'inboundAuthConfig', value: InboundAuthConfig[]) => void;
  /** Claims the caller offers as selectable chips (agent schema attributes, or fixed OU/roles/groups). */
  availableAttributes: string[];
  isLoadingAttributes?: boolean;
  disabled?: boolean;
  onValidationChange?: (hasErrors: boolean) => void;
  copy: ClientAccessTokenCopy;
  inputId?: string;
  /** Value shown for the `sub` claim in the preview (the client entity's own ID). */
  subjectValue?: string;
  /** Identity class the server stamps as `sub_type` on this client's token. */
  subjectType: 'application' | 'agent';
}

/**
 * Access-token settings for the OAuth client acting as its own subject (client_credentials grant) —
 * the `clientConfig` half of AccessTokenConfig, with its own attribute set and validity period.
 * Shared by the application and agent token tabs; the selectable attributes and copy are supplied
 * by the caller.
 */
export default function ClientAccessTokenSection({
  oauth2Config = undefined,
  inboundAuthConfig = undefined,
  onFieldChange,
  availableAttributes,
  isLoadingAttributes = false,
  disabled = false,
  onValidationChange = undefined,
  copy,
  inputId = 'client-access-token-validity',
  subjectValue = '<sub>',
  subjectType,
}: ClientAccessTokenSectionProps): JSX.Element {
  const {t} = useTranslation();

  const clientConfig = oauth2Config?.token?.accessToken?.clientConfig;
  const currentAttributes = clientConfig?.attributes ?? [];

  const [validityInput, setValidityInput] = useState<string>(String(clientConfig?.validityPeriod ?? 3600));
  const parsedValidity = parseInt(validityInput, 10);
  const isValidityInvalid = validityInput.trim() === '' || Number.isNaN(parsedValidity) || parsedValidity < 1;

  useEffect(() => {
    onValidationChange?.(isValidityInvalid);
  }, [isValidityInvalid, onValidationChange]);

  const commitClientConfig = (updates: {attributes?: string[]; validityPeriod?: number}): void => {
    if (!oauth2Config || disabled) return;
    const updatedConfig: OAuth2Config = {
      ...oauth2Config,
      token: {
        ...oauth2Config.token,
        accessToken: {
          ...oauth2Config.token?.accessToken,
          clientConfig: {...clientConfig, ...updates},
        },
      } as OAuth2Config['token'],
    };
    const currentInboundAuth = inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: updatedConfig} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleAttributeClick = (attr: string): void => {
    const nextAttrs = currentAttributes.includes(attr)
      ? currentAttributes.filter((a) => a !== attr)
      : [...currentAttributes, attr];
    commitClientConfig({attributes: nextAttrs});
  };

  const handleValidityChange = (value: string): void => {
    setValidityInput(value);
    const parsed = parseInt(value, 10);
    if (value.trim() !== '' && !Number.isNaN(parsed) && parsed >= 1) {
      commitClientConfig({validityPeriod: parsed});
    }
  };

  const previewValueFor = (claim: string): string => {
    if (claim === 'sub') return subjectValue;
    // sub_type carries its literal value: the server fixes it from the client's identity class.
    if (claim === SUB_TYPE_CLAIM) return subjectType;
    return `<${claim}>`;
  };

  const jwtPreview: Record<string, unknown> = {};
  ACCESS_TOKEN_DEFAULT_CLAIMS.forEach((attr) => {
    jwtPreview[attr] = previewValueFor(attr);
  });
  currentAttributes.forEach((attr) => {
    jwtPreview[attr] = previewValueFor(attr);
  });

  return (
    <Stack spacing={3}>
      <SettingsCard title={copy.attributesTitle} description={copy.attributesDescription}>
        <Grid container spacing={3}>
          <Grid size={{xs: 12, lg: 7}}>
            <Typography variant="body2" sx={{mb: 1}}>
              {copy.attributesLabel}
            </Typography>
            <Typography variant="body2" color="text.disabled" sx={{mb: 2}}>
              {copy.attributesHint}
            </Typography>
            <Card>
              <CardContent>
                {isLoadingAttributes && (
                  <Typography variant="body2" color="text.secondary">
                    {t('common:status.loading', 'Loading…')}
                  </Typography>
                )}
                {!isLoadingAttributes && availableAttributes.length > 0 && (
                  <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                    {availableAttributes.map((attr) => {
                      const isActive = currentAttributes.includes(attr);
                      return (
                        <Chip
                          key={attr}
                          label={attr}
                          size="small"
                          variant={isActive ? 'filled' : 'outlined'}
                          color={isActive ? 'primary' : 'default'}
                          onClick={disabled ? undefined : () => handleAttributeClick(attr)}
                          sx={{cursor: disabled ? 'default' : 'pointer'}}
                        />
                      );
                    })}
                  </Stack>
                )}
                {!isLoadingAttributes && availableAttributes.length === 0 && (
                  <Alert severity="info">{copy.attributesEmpty}</Alert>
                )}
              </CardContent>
            </Card>
          </Grid>
          <Grid size={{xs: 12, lg: 5}} sx={{minWidth: 0}}>
            <JwtPreview payload={jwtPreview} defaultClaims={ACCESS_TOKEN_DEFAULT_CLAIMS} />
          </Grid>
        </Grid>
      </SettingsCard>

      <SettingsCard title={copy.validityTitle} description={copy.validityDescription}>
        <FormControl fullWidth required>
          <FormLabel htmlFor={inputId}>{copy.validityLabel}</FormLabel>
          <TextField
            id={inputId}
            type="number"
            fullWidth
            value={validityInput}
            onChange={(e) => handleValidityChange(e.target.value)}
            error={isValidityInvalid}
            helperText={isValidityInvalid ? copy.validityError : copy.validityHint}
            inputProps={{min: 1}}
            disabled={disabled}
          />
        </FormControl>
      </SettingsCard>
    </Stack>
  );
}
