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

import {ResourceAvatar, SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {ConnectionConstants, isConflictError} from '@thunderid/configure-connections';
import {useConfig} from '@thunderid/contexts';
import {
  Alert,
  Box,
  Button,
  FormControl,
  FormLabel,
  PageContent,
  Skeleton,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {ChevronLeft, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';
import useTrustedIssuer from '../api/useTrustedIssuer';
import useUpdateTrustedIssuer from '../api/useUpdateTrustedIssuer';
import TrustedIssuerDeleteDialog from '../components/TrustedIssuerDeleteDialog';
import type {TrustedIssuerFormData} from '../models/trusted-issuer';
import isTrustedIssuerFormDirty from '../utils/isTrustedIssuerFormDirty';
import validateTrustedIssuerForm, {
  type TrustedIssuerFieldErrorKind,
  type TrustedIssuerFormErrors,
} from '../utils/validateTrustedIssuerForm';

export default function TrustedIssuerDetailPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {id} = useParams<{id: string}>();
  const {config} = useConfig();
  const productName = config.brand.product_name;

  const trustedIssuerQuery = useTrustedIssuer(id);
  const updateMutation = useUpdateTrustedIssuer(id ?? '');

  const [editedValues, setEditedValues] = useState<Partial<TrustedIssuerFormData>>({});
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [nameError, setNameError] = useState<string | null>(null);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const data = trustedIssuerQuery.data;

  const baseline: TrustedIssuerFormData = useMemo(
    () => ({
      name: data?.name ?? '',
      issuer: data?.issuer ?? '',
      jwksEndpoint: data?.jwksEndpoint ?? '',
      idJagEnabled: data?.idJagEnabled ?? false,
      tokenExchangeEnabled: data?.tokenExchangeEnabled ?? false,
      trustedTokenAudience: data?.trustedTokenAudience ?? undefined,
    }),
    [data],
  );

  const values: TrustedIssuerFormData = useMemo(() => ({...baseline, ...editedValues}), [baseline, editedValues]);
  const dirty: boolean = isTrustedIssuerFormDirty(values, baseline);

  const errors: TrustedIssuerFormErrors = useMemo(() => validateTrustedIssuerForm(values), [values]);
  const valid: boolean = Object.keys(errors).length === 0;

  const fieldErrorMessage = (kind: TrustedIssuerFieldErrorKind | undefined): string | undefined => {
    if (kind === 'required') {
      return t('trustedIssuers:validation.required', 'This field is required.');
    }
    if (kind === 'url') {
      return t('trustedIssuers:validation.url', 'Enter a valid https:// URL.');
    }
    return undefined;
  };

  const setField = <K extends keyof TrustedIssuerFormData>(field: K, value: TrustedIssuerFormData[K]): void => {
    setEditedValues((prev) => ({...prev, [field]: value}));
  };

  const setTouchedField = (field: string): void => setTouched((prev) => ({...prev, [field]: true}));

  const resetEdits = (): void => {
    setEditedValues({});
    setTouched({});
    setNameError(null);
  };

  const isLoading: boolean = trustedIssuerQuery.isLoading;
  const notFound: boolean = !isLoading && !data;

  const handleSave = (): void => {
    if (!valid || !id) return;

    setNameError(null);
    updateMutation.mutate(values, {
      onSuccess: () => {
        void trustedIssuerQuery.refetch();
        resetEdits();
      },
      onError: (error) => {
        if (isConflictError(error)) {
          setNameError(t('trustedIssuers:detail.duplicateName', 'A trusted issuer with this name already exists.'));
        }
      },
    });
  };

  return (
    <PageContent>
      <Button
        variant="text"
        startIcon={<ChevronLeft size={16} />}
        onClick={() => void navigate(RouteConfig.connections.list())}
        sx={{mb: 2, alignSelf: 'flex-start'}}
      >
        {t('trustedIssuers:detail.back', 'Back to connections')}
      </Button>

      {isLoading ? (
        <Skeleton variant="rounded" height={480} />
      ) : notFound || trustedIssuerQuery.isError ? (
        <Alert severity="error">{t('trustedIssuers:detail.loadError', 'Failed to load trusted issuer.')}</Alert>
      ) : (
        <>
          <Stack direction="row" spacing={2} alignItems="flex-start" sx={{mb: 3}}>
            <Box
              sx={{
                width: 52,
                height: 52,
                borderRadius: 2,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                bgcolor: 'action.hover',
                flexShrink: 0,
              }}
            >
              <ResourceAvatar variant="rounded" size={55} fallback={ConnectionConstants.DEFAULT_TRUSTED_IDP_AVATAR} />
            </Box>
            <Stack direction="column" spacing={0.5}>
              <Typography variant="h5" fontWeight={700}>
                {data?.name}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                {data?.issuer}
              </Typography>
            </Stack>
          </Stack>

          <Stack direction="column" spacing={4}>
            <SettingsCard
              title={t('trustedIssuers:detail.general.title', 'General')}
              description={t('trustedIssuers:detail.general.description', 'Core identity of this trusted issuer.')}
            >
              <Stack direction="column" spacing={3}>
                <FormControl fullWidth required error={Boolean(nameError ?? (touched.name && errors.name))}>
                  <FormLabel htmlFor="trusted-issuer-name">
                    {t('trustedIssuers:create.form.name.label', 'Name')}
                  </FormLabel>
                  <TextField
                    id="trusted-issuer-name"
                    fullWidth
                    value={values.name}
                    error={Boolean(nameError ?? (touched.name && errors.name))}
                    helperText={nameError ?? (touched.name ? fieldErrorMessage(errors.name) : undefined)}
                    onChange={(e) => {
                      setField('name', e.target.value);
                      setNameError(null);
                    }}
                    onBlur={() => setTouchedField('name')}
                  />
                </FormControl>

                <FormControl fullWidth required error={Boolean(touched.issuer && errors.issuer)}>
                  <FormLabel htmlFor="trusted-issuer-issuer">
                    {t('trustedIssuers:create.form.issuer.label', 'Issuer URI')}
                  </FormLabel>
                  <TextField
                    id="trusted-issuer-issuer"
                    fullWidth
                    value={values.issuer}
                    error={Boolean(touched.issuer && errors.issuer)}
                    helperText={touched.issuer ? fieldErrorMessage(errors.issuer) : undefined}
                    onChange={(e) => setField('issuer', e.target.value)}
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
                    value={values.jwksEndpoint}
                    error={Boolean(touched.jwksEndpoint && errors.jwksEndpoint)}
                    helperText={touched.jwksEndpoint ? fieldErrorMessage(errors.jwksEndpoint) : undefined}
                    onChange={(e) => setField('jwksEndpoint', e.target.value)}
                    onBlur={() => setTouchedField('jwksEndpoint')}
                  />
                </FormControl>
              </Stack>
            </SettingsCard>

            <SettingsCard
              title={t('trustedIssuers:detail.tokenExchange.title', 'Token Exchange')}
              description={t(
                'trustedIssuers:detail.tokenExchange.description',
                'Exchange subject tokens from this issuer for access tokens.',
              )}
              enabled={values.tokenExchangeEnabled}
              onToggle={(checked) => setField('tokenExchangeEnabled', checked)}
            >
              <FormControl fullWidth>
                <FormLabel htmlFor="trusted-issuer-token-audience">
                  {t('trustedIssuers:detail.tokenExchange.audience.label', 'Trusted token audience')}
                </FormLabel>
                <TextField
                  id="trusted-issuer-token-audience"
                  fullWidth
                  placeholder="api://thunderid"
                  value={values.trustedTokenAudience ?? ''}
                  helperText={t(
                    'trustedIssuers:detail.tokenExchange.audience.hint',
                    "An additional audience value {{productName}} will accept in subject tokens from this issuer. Tokens whose audience is {{productName}}'s own issuer URL are always accepted.",
                    {productName},
                  )}
                  onChange={(e) => setField('trustedTokenAudience', e.target.value || undefined)}
                />
              </FormControl>
            </SettingsCard>

            <SettingsCard
              title={t(
                'trustedIssuers:detail.consumption.title',
                'Identity Assertion JWT Authorization Grant (ID-JAG)',
              )}
              description={t(
                'trustedIssuers:detail.idJag.description',
                'Accept and exchange signed identity assertions from this issuer for access tokens.',
              )}
              enabled={values.idJagEnabled}
              onToggle={(checked) => setField('idJagEnabled', checked)}
            >
              <Typography variant="body2" color="text.secondary">
                {t(
                  'trustedIssuers:detail.idJag.enabledNote',
                  'Identity assertions from this issuer are accepted via the ID-JAG protocol.',
                )}
              </Typography>
            </SettingsCard>

            <SettingsCard title={t('trustedIssuers:detail.dangerZone.title', 'Danger zone')}>
              <Typography variant="h6" gutterBottom color="error">
                {t('trustedIssuers:detail.dangerZone.delete.title', 'Delete trusted issuer')}
              </Typography>
              <Typography variant="body2" color="text.secondary" sx={{mb: 3}}>
                {t(
                  'trustedIssuers:detail.dangerZone.delete.description',
                  'Applications relying on assertions from this issuer will stop receiving tokens. This cannot be undone.',
                )}
              </Typography>
              <Button
                variant="contained"
                color="error"
                startIcon={<Trash2 size={16} />}
                onClick={() => setDeleteOpen(true)}
                data-testid="trusted-issuer-delete-button"
              >
                {t('common:actions.delete')}
              </Button>
            </SettingsCard>
          </Stack>

          {dirty && (
            <UnsavedChangesBar
              message={t('trustedIssuers:detail.saveBar.unsaved', 'You have unsaved changes')}
              resetLabel={t('trustedIssuers:detail.saveBar.reset', 'Reset')}
              saveLabel={t('trustedIssuers:detail.saveBar.save', 'Save changes')}
              savingLabel={t('common:status.saving', 'Saving...')}
              isSaving={updateMutation.isPending}
              saveDisabled={!valid}
              onReset={resetEdits}
              onSave={handleSave}
            />
          )}

          <TrustedIssuerDeleteDialog
            open={deleteOpen}
            trustedIssuerId={id ?? null}
            trustedIssuerName={data?.name ?? ''}
            onClose={() => setDeleteOpen(false)}
            onSuccess={() => void navigate(RouteConfig.connections.list())}
          />
        </>
      )}
    </PageContent>
  );
}
