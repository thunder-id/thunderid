// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {
  Alert,
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
import {ChevronLeft} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateTrustedIssuer from '../api/useCreateTrustedIssuer';
import useConnectionRoutes from '../hooks/useConnectionRoutes';
import useIsIdJagAcceptanceUnavailable from '../hooks/useIsIdJagAcceptanceUnavailable';
import isConflictError from '../utils/isConflictError';
import validateTrustedIssuerForm, {
  type TrustedIssuerFieldErrorKind,
  type TrustedIssuerFormErrors,
} from '../utils/validateTrustedIssuerForm';

interface TrustedIssuerCreateFormProps {
  /** Connection name collected on the wizard's name step. */
  name: string;
  /** The organization unit that will own the trusted issuer. */
  ouId: string;
  /** Call when the create request 409s on a duplicate name, to bounce back to the name step. */
  onNameConflict: () => void;
  /** Call when the wizard's Back button is pressed, to return to the name step. */
  onBack: () => void;
}

/**
 * The trusted-issuer create form: fields, validation, and submission via
 * {@link useCreateTrustedIssuer}. On success, navigates to the created issuer's detail page.
 *
 * The ID-JAG switch cannot be turned on while the server does not accept identity assertions,
 * but stays available to turn off.
 *
 * Renders its own Back + submit footer so it lines up with the other "Add custom connection"
 * wizard steps; the wizard supplies the surrounding chrome (breadcrumb, progress, close) and
 * collects the connection name on a preceding step.
 */
export default function TrustedIssuerCreateForm({
  name,
  ouId,
  onNameConflict,
  onBack,
}: TrustedIssuerCreateFormProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {config} = useConfig();
  const productName = config.brand.product_name;
  const routes = useConnectionRoutes();
  const createTrustedIssuer = useCreateTrustedIssuer();
  const isIdJagAcceptanceUnavailable = useIsIdJagAcceptanceUnavailable();

  const [issuer, setIssuer] = useState('');
  const [jwksEndpoint, setJwksEndpoint] = useState('');
  const [idJagEnabled, setIdJagEnabled] = useState(false);
  const [tokenExchangeEnabled, setTokenExchangeEnabled] = useState(true);
  const [trustedTokenAudience, setTrustedTokenAudience] = useState('');
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [generalError, setGeneralError] = useState<string | null>(null);

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

  // Resolves an error through the `trustedIssuers` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `trustedIssuers:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `trustedIssuers:${key}`, options),
    [t],
  );

  // A create failure is stale once the user edits any field. Only reset the mutation once it
  // has actually failed: resetting while it's still pending would flip isPending back to false
  // and re-enable the submit button before the in-flight request settles.
  const clearCreateError = useCallback((): void => {
    setGeneralError(null);
    if (createTrustedIssuer.isError) {
      createTrustedIssuer.reset();
    }
  }, [createTrustedIssuer]);

  const handleCreate = (): void => {
    if (!formValid) return;

    if (!ouId) {
      setGeneralError(t('connections:wizard.organizationUnit.required', 'Organization unit is required'));
      return;
    }

    setGeneralError(null);
    createTrustedIssuer.mutate(
      {
        ouId,
        name,
        issuer: issuer.trim(),
        jwksEndpoint: jwksEndpoint.trim(),
        idJagEnabled,
        tokenExchangeEnabled,
        trustedTokenAudience: trustedTokenAudience.trim() || undefined,
      },
      {
        onSuccess: (created) => {
          void navigate(routes.trustedIssuers.detail(created.id));
        },
        onError: (error) => {
          if (isConflictError(error)) {
            onNameConflict();
          } else {
            setGeneralError(
              getErrorMessage(error, tForErrors, 'create.error', 'Failed to create trusted issuer. Please try again.'),
            );
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
          <FormControl fullWidth required error={Boolean(touched['issuer'] && errors.issuer)}>
            <FormLabel htmlFor="trusted-issuer-issuer">
              {t('trustedIssuers:create.form.issuer.label', 'Issuer URI')}
            </FormLabel>
            <TextField
              id="trusted-issuer-issuer"
              fullWidth
              placeholder="https://idp.example.com"
              value={issuer}
              error={Boolean(touched['issuer'] && errors.issuer)}
              helperText={
                (touched['issuer'] ? fieldErrorMessage(errors.issuer) : undefined) ??
                t(
                  'trustedIssuers:create.form.issuer.hint',
                  "The issuer URI from the external IdP's OpenID Connect discovery document.",
                )
              }
              onChange={(e) => {
                clearCreateError();
                setIssuer(e.target.value);
              }}
              onBlur={() => setTouchedField('issuer')}
            />
          </FormControl>

          <FormControl fullWidth required error={Boolean(touched['jwksEndpoint'] && errors.jwksEndpoint)}>
            <FormLabel htmlFor="trusted-issuer-jwks-endpoint">
              {t('trustedIssuers:create.form.jwksEndpoint.label', 'JWKS endpoint')}
            </FormLabel>
            <TextField
              id="trusted-issuer-jwks-endpoint"
              fullWidth
              placeholder="https://idp.example.com/oauth2/v1/keys"
              value={jwksEndpoint}
              error={Boolean(touched['jwksEndpoint'] && errors.jwksEndpoint)}
              helperText={
                (touched['jwksEndpoint'] ? fieldErrorMessage(errors.jwksEndpoint) : undefined) ??
                t(
                  'trustedIssuers:create.form.jwksEndpoint.hint',
                  'The JWKS endpoint used to validate the signature of incoming identity assertions.',
                )
              }
              onChange={(e) => {
                clearCreateError();
                setJwksEndpoint(e.target.value);
              }}
              onBlur={() => setTouchedField('jwksEndpoint')}
            />
          </FormControl>

          <Box>
            <Divider sx={{mb: 2}} />
            <FormControlLabel
              control={
                <Switch
                  checked={tokenExchangeEnabled}
                  onChange={(e) => {
                    clearCreateError();
                    setTokenExchangeEnabled(e.target.checked);
                  }}
                />
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
                  onChange={(e) => {
                    clearCreateError();
                    setTrustedTokenAudience(e.target.value);
                  }}
                />
              </FormControl>
            </Collapse>
          </Box>

          <Box>
            <Divider sx={{mb: 2}} />
            <FormControlLabel
              control={
                <Switch
                  checked={idJagEnabled}
                  disabled={isIdJagAcceptanceUnavailable && !idJagEnabled}
                  onChange={(e) => {
                    clearCreateError();
                    setIdJagEnabled(e.target.checked);
                  }}
                />
              }
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
            {isIdJagAcceptanceUnavailable && (
              <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: 6.5}}>
                {t(
                  'trustedIssuers:idJag.acceptanceUnavailable',
                  'This server does not accept identity assertions from any issuer.',
                )}
              </Typography>
            )}
          </Box>
        </Stack>
      </Paper>

      {generalError && (
        <Alert severity="error" onClose={clearCreateError}>
          {generalError}
        </Alert>
      )}

      <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
        <Button variant="outlined" startIcon={<ChevronLeft size={16} />} onClick={onBack}>
          {t('common:actions.back')}
        </Button>
        <Button
          variant="contained"
          disabled={!formValid || createTrustedIssuer.isPending}
          onClick={handleCreate}
          data-testid="trusted-issuer-create-submit"
        >
          {createTrustedIssuer.isPending
            ? t('common:status.saving')
            : t('connections:form.actions.create', 'Create connection')}
        </Button>
      </Box>
    </Stack>
  );
}
