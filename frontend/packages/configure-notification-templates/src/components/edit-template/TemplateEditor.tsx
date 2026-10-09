// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, IconButton, PageTitle, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {ArrowLeft, Edit} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
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

  const [displayName, setDisplayName] = useState(template.displayName);
  const [description, setDescription] = useState(template.description ?? '');
  const [subject, setSubject] = useState(isEmail ? (template as EmailTemplate).content.subject : '');
  const [body, setBody] = useState(template.content.body);

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
  const [colorScheme, setColorScheme] = useState<TemplateColorScheme>(
    isEmail ? ((template as EmailTemplate).design?.colorScheme ?? 'light') : 'light',
  );
  const [saveError, setSaveError] = useState<string | null>(null);

  const handleBack = (): void => {
    void navigate(routes.notificationTemplates.channel(channel));
  };

  const handleSave = (): void => {
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
    <Box>
      <Button
        variant="text"
        size="small"
        startIcon={<ArrowLeft size={16} />}
        onClick={handleBack}
        sx={{mb: 1, color: 'text.secondary'}}
      >
        {t('editor.back', 'Notification Templates')}
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
                  aria-label={t('common:actions.edit')}
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
                  aria-label={t('common:actions.edit')}
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
        <PageTitle.Actions>
          <Button variant="contained" onClick={handleSave} disabled={update.isPending}>
            {update.isPending ? t('common:status.saving', 'Saving...') : t('editor.save', 'Save changes')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {saveError && (
        <Alert severity="error" sx={{mb: 2}}>
          {saveError}
        </Alert>
      )}

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
          onSubjectChange={setSubject}
          onBodyChange={setBody}
          onColorSchemeChange={setColorScheme}
        />

        {/* ── Preview pane ────────────────────────────────────────────── */}
        <TemplatePreviewPanel channel={channel} subject={isEmail ? subject : undefined} body={body} colorScheme={colorScheme} />
      </Box>
    </Box>
  );
}
