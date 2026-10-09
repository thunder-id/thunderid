// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {Stack, Button, Alert} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AddAssignmentDialog from './AddAssignmentDialog';
import ManageAssignmentsSection from './ManageAssignmentsSection';
import useAddRoleAssignments from '../../../api/useAddRoleAssignments';
import useRemoveRoleAssignments from '../../../api/useRemoveRoleAssignments';
import type {RoleAssignment} from '../../../models/role';

interface EditAssignmentsSettingsProps {
  roleId: string;
  isReadOnly?: boolean;
}

/**
 * Assignments tab content for the Role edit page.
 * Provides assignment listing, add, and remove functionality.
 */
export default function EditAssignmentsSettings({
  roleId,
  isReadOnly = false,
}: EditAssignmentsSettingsProps): JSX.Element {
  const {t} = useTranslation('roles');
  const addRoleAssignments = useAddRoleAssignments();
  const removeRoleAssignments = useRemoveRoleAssignments();
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const [addError, setAddError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [activeAssignmentTab, setActiveAssignmentTab] = useState(0);

  const handleAddAssignments = useCallback(
    (newAssignments: RoleAssignment[]) => {
      if (newAssignments.length === 0) return;
      addRoleAssignments.mutate(
        {roleId, assignments: newAssignments},
        {
          onSuccess: () => {
            setAddDialogOpen(false);
            setAddError(null);
          },
          onError: (err: Error) => {
            setAddError(
              getErrorMessage(err, t, 'assignments.add.error', 'Failed to add assignment. Please try again.'),
            );
          },
        },
      );
    },
    [roleId, addRoleAssignments, t],
  );

  const handleRemoveAssignment = useCallback(
    (assignmentToRemove: RoleAssignment) => {
      removeRoleAssignments.mutate(
        {roleId, assignment: {id: assignmentToRemove.id, type: assignmentToRemove.type}},
        {
          onSuccess: () => {
            setError(null);
          },
          onError: (err: Error) => {
            setError(
              getErrorMessage(err, t, 'assignments.remove.error', 'Failed to remove assignment. Please try again.'),
            );
          },
        },
      );
    },
    [roleId, removeRoleAssignments, t],
  );

  return (
    <Stack spacing={3}>
      {error && (
        <Alert severity="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <ManageAssignmentsSection
        roleId={roleId}
        onRemoveAssignment={handleRemoveAssignment}
        activeAssignmentTab={activeAssignmentTab}
        onAssignmentTabChange={setActiveAssignmentTab}
        isReadOnly={isReadOnly}
        headerAction={
          !isReadOnly ? (
            <Button
              variant="contained"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={() => {
                setAddError(null);
                setAddDialogOpen(true);
              }}
            >
              {t('edit.assignments.sections.manage.addAssignment', 'Add')}
            </Button>
          ) : undefined
        }
      />

      {addDialogOpen && !isReadOnly && (
        <AddAssignmentDialog
          open={addDialogOpen}
          roleId={roleId}
          onClose={() => {
            setAddDialogOpen(false);
            setAddError(null);
          }}
          onAdd={handleAddAssignments}
          error={addError}
          onErrorDismiss={() => setAddError(null)}
          isSubmitting={addRoleAssignments.isPending}
          initialTab={activeAssignmentTab}
        />
      )}
    </Stack>
  );
}
