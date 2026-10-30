// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Button, Dialog, DialogActions, DialogContent, DialogContentText, DialogTitle} from '@wso2/oxygen-ui';
import {useState, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import getProjectErrorMessage from './getProjectErrorMessage';
import useDeleteProject from './useDeleteProject';
import type {Project} from './useGetProjects';

export interface ProjectDeleteDialogProps {
  /** The project to delete; the dialog is open while one is set. */
  project: Project | null;
  onClose: () => void;
}

/** Confirms deleting a project. It closes only once the project is deleted. */
export default function ProjectDeleteDialog({project, onClose}: ProjectDeleteDialogProps): ReactNode {
  const {t} = useTranslation();
  const deleteProject = useDeleteProject();
  const [error, setError] = useState<string | null>(null);

  const handleCancel = (): void => {
    if (deleteProject.isPending) return;
    setError(null);
    deleteProject.reset();
    onClose();
  };

  const handleConfirm = (): void => {
    if (!project) return;

    setError(null);
    deleteProject.mutate(project.id, {
      onSuccess: () => onClose(),
      onError: (err: Error) => {
        setError(
          getProjectErrorMessage(
            err,
            t,
            'common:projects.delete.error',
            'Failed to delete the project. Please try again.',
          ),
        );
      },
    });
  };

  return (
    <Dialog open={project !== null} onClose={handleCancel} maxWidth="sm" fullWidth>
      <DialogTitle>{t('common:projects.delete.title', 'Delete project')}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{mb: 2}}>
          {t('common:projects.delete.message', 'Are you sure you want to delete the project "{{name}}"?', {
            name: project?.displayName,
          })}
        </DialogContentText>
        <Alert severity="warning">
          {t(
            'common:projects.delete.disclaimer',
            'This action cannot be undone. The project and its root organization unit will be permanently removed.',
          )}
        </Alert>
        {error && (
          <Alert severity="error" sx={{mt: 2}}>
            {error}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={handleCancel} disabled={deleteProject.isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button onClick={handleConfirm} color="error" variant="contained" disabled={deleteProject.isPending}>
          {deleteProject.isPending ? t('common:status.deleting', 'Deleting...') : t('common:actions.delete', 'Delete')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
