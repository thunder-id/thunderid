// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {PageLoadingAnimation, QueryErrorNotice, SettingsCard, UnsavedChangesBar} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {getErrorMessage} from '@thunderid/utils';
import {
  Button,
  Chip,
  FormControl,
  FormLabel,
  PageContent,
  PageTitle,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {ArrowLeft, KeyRound} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetGateway from '../api/useGetGateway';
import useUpdateGateway from '../api/useUpdateGateway';
import GatewayDeleteDialog from '../components/GatewayDeleteDialog';
import RotateKeyDialog from '../components/RotateKeyDialog';
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import useGatewayRoutes from '../hooks/useGatewayRoutes';
import type {Gateway, UpdateGatewayRequest} from '../models/gateway';

type EditableField = 'name' | 'baseUrl' | 'caCertificate';

export default function GatewayDetailPage(): JSX.Element {
  const {gatewayId = ''} = useParams<{gatewayId: string}>();
  const {t} = useTranslation();
  const tForErrors = useGatewayErrorTranslator();
  const navigate = useNavigate();
  const routes = useGatewayRoutes();
  const logger = useLogger('GatewayDetailPage');
  const {data: gateway, isLoading, error, refetch} = useGetGateway(gatewayId);
  const updateGateway = useUpdateGateway();
  const [edited, setEdited] = useState<Partial<Record<EditableField, string>>>({});
  // The page is reused when only the gateway in the URL changes, so edits made to another gateway
  // are dropped rather than saved to this one.
  const [editedGatewayId, setEditedGatewayId] = useState(gatewayId);
  if (editedGatewayId !== gatewayId) {
    setEditedGatewayId(gatewayId);
    setEdited({});
  }
  const [rotateOpen, setRotateOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);

  const valueOf = (field: EditableField, source: Gateway): string => edited[field] ?? source[field] ?? '';

  const changes = useMemo((): UpdateGatewayRequest => {
    if (!gateway) return {};
    const result: UpdateGatewayRequest = {};
    (Object.keys(edited) as EditableField[]).forEach((field) => {
      const next = (edited[field] ?? '').trim();
      if (next !== (gateway[field] ?? '').trim()) {
        result[field] = next;
      }
    });
    return result;
  }, [edited, gateway]);

  const hasChanges = Object.keys(changes).length > 0;
  const isInvalid =
    (edited.name !== undefined && !edited.name.trim()) || (edited.baseUrl !== undefined && !edited.baseUrl.trim());

  const handleChange = (field: EditableField, value: string): void => {
    if (updateGateway.isError) updateGateway.reset();
    setEdited((prev) => ({...prev, [field]: value}));
  };

  const handleReset = (): void => {
    if (updateGateway.isError) updateGateway.reset();
    setEdited({});
  };

  const handleSave = (): void => {
    if (!gateway || !hasChanges || isInvalid) return;

    updateGateway.mutate(
      {id: gateway.id, data: changes},
      {
        onSuccess: () => setEdited({}),
        onError: (err: Error) => {
          logger.error('Failed to update gateway', {error: err});
        },
      },
    );
  };

  const goToList = (): void => {
    (async (): Promise<void> => {
      await navigate(routes.list());
    })().catch((err: unknown) => {
      logger.error('Failed to navigate back to gateways', {error: err});
    });
  };

  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  if (error || !gateway) {
    return (
      <PageContent>
        <QueryErrorNotice
          error={error ?? new Error('Gateway not found')}
          t={tForErrors}
          variant="block"
          title={t('gateways:detail.error', 'Failed to load the gateway')}
          onRetry={() => void refetch()}
          action={
            <Button onClick={goToList} startIcon={<ArrowLeft size={16} />}>
              {t('gateways:detail.back', 'Back to gateways')}
            </Button>
          }
        />
      </PageContent>
    );
  }

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.BackButton component={<Link to={routes.list()} />}>
          {t('gateways:detail.back', 'Back to gateways')}
        </PageTitle.BackButton>
        <PageTitle.Header>
          <Stack direction="row" spacing={1.5} alignItems="center" component="span">
            <span>{gateway.name}</span>
            {gateway.isDefault && (
              <Chip size="small" color="primary" variant="outlined" label={t('gateways:default', 'Default')} />
            )}
          </Stack>
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <Typography component="span" variant="body2" color="text.secondary" sx={{fontFamily: 'monospace'}}>
            {gateway.baseUrl}
          </Typography>
        </PageTitle.SubHeader>
      </PageTitle>

      <Stack spacing={3}>
        <SettingsCard
          title={t('gateways:detail.connection.title', 'Connection')}
          description={t('gateways:detail.connection.description', 'Where this gateway answers and how it is trusted.')}
        >
          <Stack spacing={3}>
            <FormControl fullWidth required>
              <FormLabel htmlFor="gateway-edit-name-input">{t('gateways:form.name.label', 'Name')}</FormLabel>
              <TextField
                id="gateway-edit-name-input"
                fullWidth
                value={valueOf('name', gateway)}
                onChange={(e) => handleChange('name', e.target.value)}
              />
            </FormControl>
            <FormControl fullWidth required>
              <FormLabel htmlFor="gateway-edit-base-url-input">
                {t('gateways:form.baseUrl.label', 'Base URL')}
              </FormLabel>
              <TextField
                id="gateway-edit-base-url-input"
                fullWidth
                value={valueOf('baseUrl', gateway)}
                onChange={(e) => handleChange('baseUrl', e.target.value)}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel htmlFor="gateway-edit-ca-certificate-input">
                {t('gateways:form.caCertificate.label', 'CA certificate (optional)')}
              </FormLabel>
              <TextField
                id="gateway-edit-ca-certificate-input"
                fullWidth
                multiline
                minRows={4}
                value={valueOf('caCertificate', gateway)}
                onChange={(e) => handleChange('caCertificate', e.target.value)}
                helperText={t(
                  'gateways:form.caCertificate.hint',
                  'A PEM certificate to trust when calling this gateway, for one serving a certificate no public authority signed.',
                )}
                slotProps={{htmlInput: {sx: {fontFamily: 'monospace', fontSize: '0.8rem'}}}}
              />
            </FormControl>
          </Stack>
        </SettingsCard>

        <SettingsCard
          title={t('gateways:detail.key.title', 'Key')}
          description={t(
            'gateways:detail.key.description',
            'The key presented to this gateway is never shown after it is set. Rotate it to replace it.',
          )}
          headerAction={
            <Button variant="outlined" startIcon={<KeyRound size={16} />} onClick={() => setRotateOpen(true)}>
              {t('gateways:rotateKey.action', 'Rotate key')}
            </Button>
          }
        >
          <Typography variant="body2" color="text.secondary">
            {t(
              'gateways:detail.key.hint',
              'After rotating, configure the gateway with the new key so it keeps accepting configuration from here.',
            )}
          </Typography>
        </SettingsCard>

        <SettingsCard
          title={t('gateways:detail.dangerZone.title', 'Danger Zone')}
          description={t(
            'gateways:detail.dangerZone.description',
            'Removing the registration stops this deployment administering the gateway.',
          )}
          headerAction={
            <Button variant="contained" color="error" onClick={() => setDeleteOpen(true)}>
              {t('gateways:delete.title', 'Remove gateway')}
            </Button>
          }
        >
          <Typography variant="body2" color="text.secondary">
            {t(
              'gateways:delete.disclaimer',
              'The gateway itself is not changed. It keeps running with the configuration it already holds.',
            )}
          </Typography>
        </SettingsCard>
      </Stack>

      {(hasChanges || isInvalid) && (
        <UnsavedChangesBar
          message={t('gateways:edit.unsavedChanges', 'You have unsaved changes.')}
          resetLabel={t('common:actions.reset', 'Reset')}
          saveLabel={t('common:actions.save', 'Save')}
          savingLabel={t('common:status.saving', 'Saving...')}
          isSaving={updateGateway.isPending}
          saveDisabled={isInvalid}
          error={
            updateGateway.error
              ? getErrorMessage(updateGateway.error, tForErrors, 'edit.error', 'The gateway could not be updated.')
              : undefined
          }
          onReset={handleReset}
          onSave={handleSave}
        />
      )}

      <RotateKeyDialog
        open={rotateOpen}
        gatewayId={gateway.id}
        gatewayName={gateway.name}
        onClose={() => setRotateOpen(false)}
      />
      <GatewayDeleteDialog
        open={deleteOpen}
        gateway={gateway}
        onClose={() => setDeleteOpen(false)}
        onSuccess={() => {
          setDeleteOpen(false);
          goToList();
        }}
      />
    </PageContent>
  );
}
