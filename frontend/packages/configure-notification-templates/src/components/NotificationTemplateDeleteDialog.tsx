// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {Alert, Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useDeleteNotificationTemplate from '../api/useDeleteNotificationTemplate';
import type {NotificationChannel} from '../models/notification-template';

/**
 * Props for the {@link NotificationTemplateDeleteDialog} component.
 *
 * @public
 */
export interface NotificationTemplateDeleteDialogProps {
  /**
   * Whether the dialog is open.
   */
  open: boolean;
  /**
   * The channel the template belongs to.
   */
  channel: NotificationChannel;
  /**
   * The id of the template to delete, or null when none is selected.
   */
  templateId: string | null;
  /**
   * The display name of the template to delete, shown in the confirmation message.
   */
  templateName?: string;
  /**
   * Callback when the dialog should be closed.
   */
  onClose: () => void;
  /**
   * Callback when the template is successfully deleted.
   */
  onSuccess?: () => void;
}

/**
 * Dialog that confirms deletion of a single notification template.
 *
 * @public
 */
export default function NotificationTemplateDeleteDialog({
  open,
  channel,
  templateId,
  templateName = undefined,
  onClose,
  onSuccess = undefined,
}: NotificationTemplateDeleteDialogProps): JSX.Element {
  const {t} = useTranslation('notificationTemplates');
  const deleteTemplate = useDeleteNotificationTemplate();
  const [error, setError] = useState<string | null>(null);

  const handleCancel = (): void => {
    if (deleteTemplate.isPending) return;
    setError(null);
    onClose();
  };

  const handleConfirm = (): void => {
    if (!templateId) return;

    deleteTemplate.mutate(
      {channel, id: templateId},
      {
        onSuccess: (): void => {
          setError(null);
          onClose();
          onSuccess?.();
        },
        onError: (err) => {
          setError(getErrorMessage(err, t, 'delete.error', 'Failed to delete template. Please try again.'));
        },
      },
    );
  };

  return (
    <Dialog open={open} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('delete.title', 'Delete Template')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {templateName
            ? t('delete.messageNamed', {
                name: templateName,
                defaultValue:
                  'Are you sure you want to delete "{{name}}"? This action cannot be undone.',
              })
            : t('delete.message', 'Are you sure you want to delete this template? This action cannot be undone.')}
        </DialogContentText>
        {error && <Alert severity="error">{error}</Alert>}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={deleteTemplate.isPending}>
          {t('common:actions.cancel')}
        </Button>
        <Button onClick={handleConfirm} color="error" variant="contained" disabled={deleteTemplate.isPending}>
          {deleteTemplate.isPending ? t('common:status.deleting') : t('common:actions.delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
