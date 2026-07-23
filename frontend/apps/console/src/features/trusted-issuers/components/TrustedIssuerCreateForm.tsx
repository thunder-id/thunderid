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

import {isConflictError} from '@thunderid/configure-connections';
import {useConfig} from '@thunderid/contexts';
import {
  Box,
  Button,
  Collapse,
  Divider,
  FormControl,
  FormControlLabel,
  FormLabel,
  Paper,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';
import useCreateTrustedIssuer from '../api/useCreateTrustedIssuer';
import validateTrustedIssuerForm, {
  type TrustedIssuerFieldErrorKind,
  type TrustedIssuerFormErrors,
} from '../utils/validateTrustedIssuerForm';

interface TrustedIssuerCreateFormProps {
  /** Connection name collected on the wizard's name step. */
  name: string;
  /** Call when the create request 409s on a duplicate name, to bounce back to the name step. */
  onNameConflict: () => void;
}

/**
 * The trusted-issuer create form: fields, validation, and submission via
 * {@link useCreateTrustedIssuer}. On success, navigates to the created issuer's detail page.
 *
 * Has no back/cancel affordance of its own — callers (the "Add custom connection" wizard) provide
 * that chrome, and collect the connection name on a preceding step.
 */
export default function TrustedIssuerCreateForm({name, onNameConflict}: TrustedIssuerCreateFormProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const createTrustedIssuer = useCreateTrustedIssuer();

  const [issuer, setIssuer] = useState('');
  const [jwksEndpoint, setJwksEndpoint] = useState('');
  const [idJagEnabled, setIdJagEnabled] = useState(false);
  const [tokenExchangeEnabled, setTokenExchangeEnabled] = useState(true);
  const [trustedTokenAudience, setTrustedTokenAudience] = useState('');
  const [touched, setTouched] = useState<Record<string, boolean>>({});

  const errors: TrustedIssuerFormErrors = useMemo(
    () => validateTrustedIssuerForm({name, issuer, jwksEndpoint}),
    [name, issuer, jwksEndpoint],
  );
  const formValid: boolean = Object.keys(errors).length === 0;

  const fieldErrorMessage = (kind: TrustedIssuerFieldErrorKind | undefined): string | undefined => {
    if (kind === 'required') {
      return t('trustedIssuers:validation.required', 'This field is required.');
    }
    if (kind === 'url') {
      return t('trustedIssuers:validation.url', 'Enter a valid https:// URL.');
    }
    return undefined;
  };

  const setTouchedField = (field: string): void => setTouched((prev) => ({...prev, [field]: true}));

  const handleCreate = (): void => {
    if (!formValid) return;

    createTrustedIssuer.mutate(
      {
        name,
        issuer: issuer.trim(),
        jwksEndpoint: jwksEndpoint.trim(),
        idJagEnabled,
        tokenExchangeEnabled,
        trustedTokenAudience: trustedTokenAudience.trim() || undefined,
      },
      {
        onSuccess: (created) => {
          void navigate(RouteConfig.trustedIssuers.detail(created.id));
        },
        onError: (error) => {
          if (isConflictError(error)) {
            onNameConflict();
          }
        },
      },
    );
  };

  return (
    <Stack direction="column" spacing={3}>
      <Stack direction="column" spacing={1}>
        <Typography variant="h1" gutterBottom>
          {t('trustedIssuers:create.title', 'Add trusted issuer')}
        </Typography>
        <Typography variant="subtitle1" gutterBottom>
          {t(
            'trustedIssuers:create.subtitle',
            'Register an external identity provider whose identity assertions ThunderID can exchange for access tokens.',
          )}
        </Typography>
      </Stack>

      <Paper variant="outlined" sx={{p: 3}}>
        <Stack direction="column" spacing={3}>
          <FormControl fullWidth required error={Boolean(touched.issuer && errors.issuer)}>
            <FormLabel htmlFor="trusted-issuer-issuer">
              {t('trustedIssuers:create.form.issuer.label', 'Issuer URI')}
            </FormLabel>
            <TextField
              id="trusted-issuer-issuer"
              fullWidth
              placeholder="https://idp.example.com"
              value={issuer}
              error={Boolean(touched.issuer && errors.issuer)}
              helperText={
                touched.issuer
                  ? fieldErrorMessage(errors.issuer)
                  : t(
                      'trustedIssuers:create.form.issuer.hint',
                      "The issuer URI from the external IdP's OpenID Connect discovery document.",
                    )
              }
              onChange={(e) => setIssuer(e.target.value)}
              onBlur={() => setTouchedField('issuer')}
            />
          </FormControl>

          <FormControl fullWidth required error={Boolean(touched.jwksEndpoint && errors.jwksEndpoint)}>
            <FormLabel htmlFor="trusted-issuer-jwks-endpoint">
              {t('trustedIssuers:create.form.jwksEndpoint.label', 'JWKS endpoint')}
            </FormLabel>
            <TextField
              id="trusted-issuer-jwks-endpoint"
              fullWidth
              placeholder="https://idp.example.com/oauth2/v1/keys"
              value={jwksEndpoint}
              error={Boolean(touched.jwksEndpoint && errors.jwksEndpoint)}
              helperText={
                touched.jwksEndpoint
                  ? fieldErrorMessage(errors.jwksEndpoint)
                  : t(
                      'trustedIssuers:create.form.jwksEndpoint.hint',
                      'The JWKS endpoint used to validate the signature of incoming identity assertions.',
                    )
              }
              onChange={(e) => setJwksEndpoint(e.target.value)}
              onBlur={() => setTouchedField('jwksEndpoint')}
            />
          </FormControl>

          <Box>
            <Divider sx={{mb: 2}} />
            <FormControlLabel
              control={
                <Switch checked={tokenExchangeEnabled} onChange={(e) => setTokenExchangeEnabled(e.target.checked)} />
              }
              label={
                <Typography variant="subtitle2">
                  {t('trustedIssuers:create.form.tokenExchangeEnabled.label', 'Enable token exchange')}
                </Typography>
              }
            />
            <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: '52px'}}>
              {t(
                'trustedIssuers:create.form.tokenExchangeEnabled.hint',
                'Exchange subject tokens from this issuer for access tokens.',
              )}
            </Typography>

            <Collapse in={tokenExchangeEnabled}>
              <FormControl fullWidth sx={{mt: 3}}>
                <FormLabel htmlFor="trusted-issuer-token-audience">
                  {t('trustedIssuers:detail.tokenExchange.audience.label', 'Trusted token audience')}
                </FormLabel>
                <TextField
                  id="trusted-issuer-token-audience"
                  fullWidth
                  placeholder="api://thunderid"
                  value={trustedTokenAudience}
                  helperText={t(
                    'trustedIssuers:detail.tokenExchange.audience.hint',
                    "An additional audience value {{productName}} will accept in subject tokens from this issuer. Tokens whose audience is {{productName}}'s own issuer URL are always accepted.",
                    {productName},
                  )}
                  onChange={(e) => setTrustedTokenAudience(e.target.value)}
                />
              </FormControl>
            </Collapse>
          </Box>

          <Box>
            <Divider sx={{mb: 2}} />
            <FormControlLabel
              control={<Switch checked={idJagEnabled} onChange={(e) => setIdJagEnabled(e.target.checked)} />}
              label={
                <Typography variant="subtitle2">
                  {t(
                    'trustedIssuers:create.form.idJagEnabled.label',
                    'Enable Identity Assertion JWT Authorization Grant (ID-JAG)',
                  )}
                </Typography>
              }
            />
            <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: '52px'}}>
              {t(
                'trustedIssuers:create.form.idJagEnabled.hint',
                'Accept and exchange signed identity assertions from this issuer for access tokens.',
              )}
            </Typography>
          </Box>
        </Stack>
      </Paper>

      <Box sx={{display: 'flex', justifyContent: 'flex-end'}}>
        <Button
          variant="contained"
          disabled={!formValid || createTrustedIssuer.isPending}
          onClick={handleCreate}
          data-testid="trusted-issuer-create-submit"
        >
          {createTrustedIssuer.isPending
            ? t('common:status.saving')
            : t('trustedIssuers:create.submit', 'Add trusted issuer')}
        </Button>
      </Box>
    </Stack>
  );
}
