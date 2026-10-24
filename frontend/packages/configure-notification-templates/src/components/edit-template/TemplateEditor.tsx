// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {UnsavedChangesBar} from '@thunderid/components';
import {getErrorMessage} from '@thunderid/utils';
import {Box, Button, IconButton, PageTitle, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {ArrowLeft, Edit} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import TemplateContentEditor from './TemplateContentEditor';
import TemplatePreviewPanel from './TemplatePreviewPanel';
import useUpdateNotificationTemplate from '../../api/useUpdateNotificationTemplate';
import useNotificationTemplateRoutes from '../../hooks/useNotificationTemplateRoutes';
import type {
  EmailTemplate,
  NotificationChannel,
  NotificationTemplate,
  TemplateColorScheme,
} from '../../models/notification-template';
import type {UpdateTemplateRequest} from '../../models/requests';

/**
 * Props for the {@link TemplateEditor} component.
 *
 * @public
 */
export interface TemplateEditorProps {
  /** The channel the template belongs to. */
  channel: NotificationChannel;
  /** The loaded template to edit. */
  template: NotificationTemplate;
}

/**
 * The template editor body: a config pane (design + subject + body with an insertable token
 * catalog) and a preview pane. Rendered once the template has loaded so its state seeds cleanly.
 *
 * @public
 */
export default function TemplateEditor({channel, template}: TemplateEditorProps): JSX.Element {
  const {t} = useTranslation('notificationTemplates');
  const navigate = useNavigate();
  const routes = useNotificationTemplateRoutes();
  const update = useUpdateNotificationTemplate();
  const isEmail = channel === 'email';

  // Loaded values, used to seed state and to detect unsaved changes. The editor is keyed by
  // template id in the edit page, so these stay fixed for the component's lifetime.
  const originalSubject = isEmail ? (template as EmailTemplate).content.subject : '';
  const originalBody = template.content.body;
  const originalColorScheme: TemplateColorScheme = isEmail
    ? ((template as EmailTemplate).design?.colorScheme ?? 'light')
    : 'light';

  const [displayName, setDisplayName] = useState(template.displayName);
  const [description, setDescription] = useState(template.description ?? '');
  const [subject, setSubject] = useState(originalSubject);
  const [body, setBody] = useState(originalBody);

  const [isEditingName, setIsEditingName] = useState(false);
  const [tempName, setTempName] = useState(template.displayName);
  const [isEditingDescription, setIsEditingDescription] = useState(false);
  const [tempDescription, setTempDescription] = useState(template.description ?? '');

  const commitName = (): void => {
    const trimmed = tempName.trim();
    if (trimmed) setDisplayName(trimmed);
    setIsEditingName(false);
  };
  const commitDescription = (): void => {
    setDescription(tempDescription.trim());
    setIsEditingDescription(false);
  };
  const [colorScheme, setColorScheme] = useState<TemplateColorScheme>(originalColorScheme);
  const [saveError, setSaveError] = useState<string | null>(null);

  // Mirror the create wizard: email needs a subject and both channels need a body.
  const contentValid = (!isEmail || subject.trim() !== '') && body.trim() !== '';

  // Why save is blocked, shown in the unsaved-changes bar so an empty field is explained rather
  // than just disabling the button.
  const validationError = contentValid
    ? undefined
    : isEmail
      ? t('editor.validation.emailRequired', 'A subject and body are required.')
      : t('editor.validation.bodyRequired', 'A body is required.');

  const hasChanges = useMemo(
    () =>
      displayName !== template.displayName ||
      description.trim() !== (template.description ?? '').trim() ||
      body !== originalBody ||
      (isEmail && subject !== originalSubject) ||
      (isEmail && colorScheme !== originalColorScheme),
    [
      displayName,
      description,
      body,
      subject,
      colorScheme,
      isEmail,
      originalBody,
      originalSubject,
      originalColorScheme,
      template,
    ],
  );

  const handleBack = (): void => {
    void navigate(routes.notificationTemplates.channel(channel));
  };

  const handleReset = (): void => {
    update.reset();
    setSaveError(null);
    setDisplayName(template.displayName);
    setDescription(template.description ?? '');
    setSubject(originalSubject);
    setBody(originalBody);
    setColorScheme(originalColorScheme);
    setIsEditingName(false);
    setIsEditingDescription(false);
  };

  const handleSave = (): void => {
    if (!contentValid) return;
    setSaveError(null);
    const base = {displayName, description: description.trim() ? description.trim() : undefined};
    const data: UpdateTemplateRequest = isEmail
      ? {...base, design: {colorScheme}, content: {subject, body}}
      : {...base, content: {body}};
    update.mutate(
      {channel, id: template.id, data},
      {
        onError: (error) => {
          setSaveError(getErrorMessage(error, t, 'update.error', 'Failed to save template. Please try again.'));
        },
      },
    );
  };

  return (
    <Box sx={{pb: hasChanges ? 12 : 0}}>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={handleBack}
        sx={{mb: 1, color: 'text.secondary'}}
      >
        {t('editor.back', 'Notification templates')}
      </Button>

      <PageTitle>
        <PageTitle.Header>
          <Stack direction="row" alignItems="center" spacing={1}>
            {isEditingName ? (
              <TextField
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={commitName}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    commitName();
                  } else if (e.key === 'Escape') {
                    setIsEditingName(false);
                  }
                }}
                size="small"
                inputProps={{maxLength: 255}}
              />
            ) : (
              <>
                <Typography variant="h3">{displayName}</Typography>
                <IconButton
                  size="small"
                  aria-label={t('common:actions.edit', 'Edit')}
                  onClick={() => {
                    setTempName(displayName);
                    setIsEditingName(true);
                  }}
                  sx={{opacity: 0.6, '&:hover': {opacity: 1}}}
                >
                  <Edit size={16} />
                </IconButton>
              </>
            )}
          </Stack>
        </PageTitle.Header>
        <PageTitle.SubHeader>
          <Stack direction="row" alignItems="flex-start" spacing={1}>
            {isEditingDescription ? (
              <TextField
                fullWidth
                multiline
                rows={2}
                value={tempDescription}
                onChange={(e) => setTempDescription(e.target.value)}
                onBlur={commitDescription}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) {
                    commitDescription();
                  } else if (e.key === 'Escape') {
                    setIsEditingDescription(false);
                  }
                }}
                size="small"
                placeholder={t('editor.description.placeholder', 'Add a description')}
                inputProps={{maxLength: 512}}
                sx={{maxWidth: 600, '& .MuiInputBase-root': {fontSize: '0.875rem'}}}
              />
            ) : (
              <>
                <Typography variant="body2" color="text.secondary">
                  {description || t('editor.description.empty', 'No description')}
                </Typography>
                <IconButton
                  size="small"
                  aria-label={t('common:actions.edit', 'Edit')}
                  onClick={() => {
                    setTempDescription(description);
                    setIsEditingDescription(true);
                  }}
                  sx={{opacity: 0.6, '&:hover': {opacity: 1}, mt: -0.5}}
                >
                  <Edit size={14} />
                </IconButton>
              </>
            )}
          </Stack>
        </PageTitle.SubHeader>
      </PageTitle>

      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', lg: 'minmax(0, 1fr) minmax(320px, 440px)'},
          gap: 3,
          mt: 2,
          alignItems: 'start',
        }}
      >
        {/* ── Config pane ─────────────────────────────────────────────── */}
        <TemplateContentEditor
          channel={channel}
          subject={subject}
          body={body}
          colorScheme={colorScheme}
          onSubjectChange={(value) => {
            setSubject(value);
            setSaveError(null);
          }}
          onBodyChange={(value) => {
            setBody(value);
            setSaveError(null);
          }}
          onColorSchemeChange={setColorScheme}
        />

        {/* ── Preview pane ────────────────────────────────────────────── */}
        <TemplatePreviewPanel
          channel={channel}
          subject={isEmail ? subject : undefined}
          body={body}
          colorScheme={colorScheme}
        />
      </Box>

      {hasChanges && (
        <UnsavedChangesBar
          message={t('editor.unsavedChanges', 'You have unsaved changes')}
          resetLabel={t('editor.reset', 'Reset')}
          saveLabel={t('editor.save', 'Save changes')}
          savingLabel={t('common:status.saving', 'Saving...')}
          isSaving={update.isPending}
          saveDisabled={!contentValid}
          error={saveError ?? validationError}
          onReset={handleReset}
          onSave={handleSave}
        />
      )}
    </Box>
  );
}
