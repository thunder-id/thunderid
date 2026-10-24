// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, FullScreenCreationWizardLayout} from '@thunderid/components';
import {
  OrganizationUnitPickerScreen,
  useGetOrganizationUnit,
  useHasMultipleOUs,
} from '@thunderid/configure-organization-units';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, CircularProgress, Stack} from '@wso2/oxygen-ui';
import {Home} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Navigate, useNavigate, useParams} from 'react-router';
import useCreateNotificationTemplate from '../api/useCreateNotificationTemplate';
import useGetNotificationTemplates from '../api/useGetNotificationTemplates';
import ConfigureTemplateIdentity from '../components/create-template/ConfigureTemplateIdentity';
import SelectNotificationTemplate, {
  type TemplateSelection,
} from '../components/create-template/SelectNotificationTemplate';
import TemplateContentEditor from '../components/edit-template/TemplateContentEditor';
import TemplatePreviewPanel from '../components/edit-template/TemplatePreviewPanel';
import useNotificationTemplateRoutes from '../hooks/useNotificationTemplateRoutes';
import {
  isNotificationChannel,
  type NotificationChannel,
  type TemplateColorScheme,
} from '../models/notification-template';
import type {CreateTemplateRequest} from '../models/requests';

type CreateStep = 'organizationUnit' | 'template' | 'identity' | 'content';

/**
 * Full-screen wizard to create a notification template. When the deployment has more than one
 * organization unit, the first step picks the owning OU (set at creation, immutable); single-OU
 * deployments skip it and adopt the only OU. The remaining steps choose a starting point (an empty
 * template or a hardcoded sample), capture the identity (display name + handle), then edit the
 * content with a live preview. On create, the new template opens in the editor.
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
  const channelValid = isNotificationChannel(channelParam);

  const createTemplate = useCreateNotificationTemplate();
  // Client-side duplicate hint only (first page of handles); the server still rejects a true
  // duplicate on create, surfaced via onError.
  const {data: templatesData} = useGetNotificationTemplates(channel);
  const existingHandles = useMemo(
    () => (templatesData?.templates ?? []).map((template) => template.handle),
    [templatesData],
  );

  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  const [step, setStep] = useState<CreateStep>('organizationUnit');
  const [ouId, setOuId] = useState('');
  const [selection, setSelection] = useState<TemplateSelection | null>(null);
  const [displayName, setDisplayName] = useState('');
  const [handle, setHandle] = useState('');
  const [handleEdited, setHandleEdited] = useState(false);
  const [identityReady, setIdentityReady] = useState(false);
  const [subject, setSubject] = useState('');
  const [body, setBody] = useState('');
  const [colorScheme, setColorScheme] = useState<TemplateColorScheme>('light');
  const [submitError, setSubmitError] = useState<string | null>(null);

  // A single-OU deployment adopts its only OU; otherwise the user's pick on the OU step.
  const soleOuId: string = !hasMultipleOUs && ouList.length === 1 ? ouList[0].id : '';
  const effectiveOuId: string = ouId !== '' ? ouId : soleOuId;
  const {data: resolvedOrganizationUnit, isLoading: isResolvedOuLoading} = useGetOrganizationUnit(
    effectiveOuId,
    Boolean(effectiveOuId),
  );

  const contentReady = (!isEmail || subject.trim() !== '') && body.trim() !== '';

  // The OU step only exists when there's a choice; single-OU deployments start at the template step.
  const steps = useMemo<CreateStep[]>(() => {
    const base: CreateStep[] = [];
    if (hasMultipleOUs) base.push('organizationUnit');
    base.push('template', 'identity', 'content');
    return base;
  }, [hasMultipleOUs]);

  // Once it's known the deployment has a single OU, skip straight past the OU step.
  const effectiveStep: CreateStep = step === 'organizationUnit' && !isOuLoading && !hasMultipleOUs ? 'template' : step;

  const goToList = (): void => {
    void navigate(routes.notificationTemplates.channel(channel));
  };

  // Seed only the content from the chosen starting point; the name and handle stay empty for the
  // user to fill on the identity step. "Start from scratch" leaves the content empty too.
  const applySelection = (next: TemplateSelection): void => {
    setSelection(next);
    setSubmitError(null);
    if (next.kind === 'blank') {
      setSubject('');
      setBody('');
      setColorScheme('light');
      return;
    }
    const {sample} = next;
    setSubject(sample.subject ?? '');
    setBody(sample.body);
    setColorScheme(sample.colorScheme ?? 'light');
  };

  const handleCreate = (): void => {
    if (!identityReady || !contentReady) return;
    setSubmitError(null);

    const base = {handle: handle.trim(), ouId: effectiveOuId, displayName: displayName.trim()};
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
    ? t('navigation:pages.emailTemplates', 'Email templates')
    : t('navigation:pages.smsTemplates', 'SMS templates');

  const stepLabel = (value: CreateStep): string => {
    if (value === 'organizationUnit') return t('create.steps.organizationUnit', 'Organization unit');
    if (value === 'template') return t('create.steps.template', 'Template');
    if (value === 'identity') return t('create.steps.identity', 'Details');
    return t('create.steps.content', 'Content');
  };

  const stepReady: Record<CreateStep, boolean> = {
    organizationUnit: effectiveOuId !== '',
    template: selection !== null,
    identity: identityReady,
    content: contentReady,
  };

  // An unknown channel segment is not a real page; send it to the canonical list.
  if (!channelValid) {
    return <Navigate to={routes.notificationTemplates.list()} replace />;
  }

  const stepIndex = steps.indexOf(effectiveStep);
  const isFirstStep = stepIndex === 0;
  const isLastStep = effectiveStep === 'content';

  // The OU step is a full-screen picker rendered outside the wizard layout, matching the app and
  // verifiable-presentation create flows.
  const pageTitle = t('listing.addTemplate', 'New template');

  if (effectiveStep === 'organizationUnit') {
    if (isOuLoading || !hasMultipleOUs) {
      return (
        <Box sx={{minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
          <DocumentTitle title={pageTitle} />
          <CircularProgress />
        </Box>
      );
    }
    return (
      <>
        <DocumentTitle title={pageTitle} />
        <OrganizationUnitPickerScreen
          icon={<Home size={26} />}
          title={t('create.organizationUnit.title', 'Where should this template belong?')}
          subtitle={t(
            'create.organizationUnit.subtitle',
            "Choose the organization unit that will own this template. You can't change this once created.",
          )}
          value={effectiveOuId}
          onChange={(value: string): void => {
            setSubmitError(null);
            setOuId(value);
          }}
          onBack={goToList}
          onContinue={() => setStep('template')}
          backLabel={t('common:actions.back', 'Back')}
          continueLabel={t('common:actions.continue', 'Continue')}
        />
      </>
    );
  }

  return (
    <FullScreenCreationWizardLayout
      onClose={() => {
        if (!createTemplate.isPending) goToList();
      }}
      progress={((stepIndex + 1) / steps.length) * 100}
      contentMaxWidth={effectiveStep === 'identity' ? 800 : false}
      breadcrumbItems={[
        {key: 'templates', label: channelCrumb, onClick: goToList},
        ...steps.slice(0, stepIndex + 1).map((value, index, shown) => ({
          key: value,
          label: stepLabel(value),
          onClick: index < shown.length - 1 ? (): void => setStep(value) : undefined,
        })),
      ]}
      footer={
        <Stack spacing={2}>
          {submitError && <Alert severity="error">{submitError}</Alert>}
          <Box sx={{display: 'flex', justifyContent: isFirstStep ? 'flex-end' : 'space-between', gap: 1.5}}>
            {isFirstStep ? (
              <Button variant="outlined" onClick={goToList} disabled={createTemplate.isPending}>
                {t('common:actions.cancel', 'Cancel')}
              </Button>
            ) : (
              <Button
                variant="outlined"
                onClick={() => setStep(steps[stepIndex - 1])}
                disabled={createTemplate.isPending}
              >
                {t('common:actions.back', 'Back')}
              </Button>
            )}
            {isLastStep ? (
              <Button variant="contained" onClick={handleCreate} disabled={!contentReady || createTemplate.isPending}>
                {createTemplate.isPending ? t('common:status.creating', 'Creating...') : t('create.submit', 'Create')}
              </Button>
            ) : (
              <Button
                variant="contained"
                onClick={() => setStep(steps[stepIndex + 1])}
                disabled={!stepReady[effectiveStep]}
              >
                {t('common:actions.continue', 'Continue')}
              </Button>
            )}
          </Box>
        </Stack>
      }
    >
      <DocumentTitle title={pageTitle} />
      {effectiveStep === 'template' && (
        <SelectNotificationTemplate channel={channel} selected={selection} onSelect={applySelection} />
      )}

      {effectiveStep === 'identity' && (
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
          hasMultipleOUs={hasMultipleOUs}
          organizationUnitName={resolvedOrganizationUnit?.name}
          organizationUnitLogoUrl={resolvedOrganizationUnit?.logoUrl}
          isOrganizationUnitLoading={isResolvedOuLoading}
          onChangeOu={() => {
            setSubmitError(null);
            setStep('organizationUnit');
          }}
        />
      )}

      {effectiveStep === 'content' && (
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
            onSubjectChange={(value) => {
              setSubject(value);
              setSubmitError(null);
            }}
            onBodyChange={(value) => {
              setBody(value);
              setSubmitError(null);
            }}
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
