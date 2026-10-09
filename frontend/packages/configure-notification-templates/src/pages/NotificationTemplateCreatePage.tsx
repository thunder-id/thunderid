// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, Stack} from '@wso2/oxygen-ui';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useCreateNotificationTemplate from '../api/useCreateNotificationTemplate';
import useGetNotificationTemplates from '../api/useGetNotificationTemplates';
import ConfigureTemplateIdentity from '../components/create-template/ConfigureTemplateIdentity';
import TemplateContentEditor from '../components/edit-template/TemplateContentEditor';
import TemplatePreviewPanel from '../components/edit-template/TemplatePreviewPanel';
import useNotificationTemplateRoutes from '../hooks/useNotificationTemplateRoutes';
import type {NotificationChannel, TemplateColorScheme} from '../models/notification-template';
import type {CreateTemplateRequest} from '../models/requests';

type CreateStep = 'identity' | 'content';

/**
 * Two-step full-screen wizard to create a notification template. Step 1 captures the identity
 * (display name + handle); step 2 is the content editor with a live preview. On create, the new
 * template opens in the editor.
 *
 * @public
 */
export default function NotificationTemplateCreatePage(): JSX.Element {
  const {t} = useTranslation('notificationTemplates');
  const navigate = useNavigate();
  const routes = useNotificationTemplateRoutes();
  const {channel: channelParam} = useParams<{channel: string}>();
  const channel: NotificationChannel = channelParam === 'sms' ? 'sms' : 'email';
  const isEmail = channel === 'email';

  const createTemplate = useCreateNotificationTemplate();
  // Client-side duplicate hint only (first page of handles); the server still rejects a true
  // duplicate on create, surfaced via onError.
  const {data: templatesData} = useGetNotificationTemplates(channel);
  const existingHandles = useMemo(
    () => (templatesData?.templates ?? []).map((template) => template.handle),
    [templatesData],
  );

  const [step, setStep] = useState<CreateStep>('identity');
  const [displayName, setDisplayName] = useState('');
  const [handle, setHandle] = useState('');
  const [handleEdited, setHandleEdited] = useState(false);
  const [identityReady, setIdentityReady] = useState(false);
  const [subject, setSubject] = useState('');
  const [body, setBody] = useState('');
  const [colorScheme, setColorScheme] = useState<TemplateColorScheme>('light');
  const [submitError, setSubmitError] = useState<string | null>(null);

  const contentReady = (!isEmail || subject.trim() !== '') && body.trim() !== '';

  const goToList = (): void => {
    void navigate(routes.notificationTemplates.channel(channel));
  };

  const handleCreate = (): void => {
    if (!identityReady || !contentReady) return;
    setSubmitError(null);

    const base = {handle: handle.trim(), displayName: displayName.trim()};
    const data: CreateTemplateRequest = isEmail
      ? {...base, design: {colorScheme}, content: {subject: subject.trim(), body}}
      : {...base, content: {body}};

    createTemplate.mutate(
      {channel, data},
      {
        onSuccess: (template) => {
          void navigate(routes.notificationTemplates.detail(channel, template.id));
        },
        onError: (error) => {
          setSubmitError(getErrorMessage(error, t, 'create.error', 'Failed to create template. Please try again.'));
        },
      },
    );
  };

  const channelCrumb = isEmail
    ? t('navigation:pages.emailTemplates', 'Email Templates')
    : t('navigation:pages.smsTemplates', 'SMS Templates');

  return (
    <FullScreenCreationWizardLayout
      onClose={() => {
        if (!createTemplate.isPending) goToList();
      }}
      progress={step === 'identity' ? 40 : 90}
      contentMaxWidth={step === 'content' ? false : 800}
      breadcrumbItems={[
        {key: 'templates', label: channelCrumb, onClick: goToList},
        {
          key: 'identity',
          label: t('create.steps.identity', 'Details'),
          onClick: step === 'content' ? () => setStep('identity') : undefined,
        },
        ...(step === 'content' ? [{key: 'content', label: t('create.steps.content', 'Content')}] : []),
      ]}
      footer={
        <Stack spacing={2}>
          {submitError && <Alert severity="error">{submitError}</Alert>}
          <Box sx={{display: 'flex', justifyContent: 'flex-end', gap: 1.5}}>
            {step === 'identity' ? (
              <>
                <Button variant="outlined" onClick={goToList} disabled={createTemplate.isPending}>
                  {t('common:actions.cancel')}
                </Button>
                <Button variant="contained" onClick={() => setStep('content')} disabled={!identityReady}>
                  {t('common:actions.continue', 'Continue')}
                </Button>
              </>
            ) : (
              <>
                <Button variant="outlined" onClick={() => setStep('identity')} disabled={createTemplate.isPending}>
                  {t('common:actions.back', 'Back')}
                </Button>
                <Button
                  variant="contained"
                  onClick={handleCreate}
                  disabled={!contentReady || createTemplate.isPending}
                >
                  {createTemplate.isPending ? t('common:status.creating', 'Creating...') : t('create.submit', 'Create')}
                </Button>
              </>
            )}
          </Box>
        </Stack>
      }
    >
      {step === 'identity' ? (
        <ConfigureTemplateIdentity
          channel={channel}
          displayName={displayName}
          handle={handle}
          handleEdited={handleEdited}
          onDisplayNameChange={setDisplayName}
          onHandleChange={setHandle}
          onHandleEditedChange={setHandleEdited}
          onReadyChange={setIdentityReady}
          existingHandles={existingHandles}
        />
      ) : (
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', lg: 'minmax(0, 1fr) minmax(320px, 440px)'},
            gap: 3,
            alignItems: 'start',
          }}
        >
          <TemplateContentEditor
            channel={channel}
            subject={subject}
            body={body}
            colorScheme={colorScheme}
            onSubjectChange={setSubject}
            onBodyChange={setBody}
            onColorSchemeChange={setColorScheme}
          />
          <TemplatePreviewPanel
            channel={channel}
            subject={isEmail ? subject : undefined}
            body={body}
            colorScheme={colorScheme}
          />
        </Box>
      )}
    </FullScreenCreationWizardLayout>
  );
}
