// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
import {Stack, Button, Alert} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AddMemberDialog from './AddMemberDialog';
import ManageMembersSection from './ManageMembersSection';
import useAddGroupMembers from '../../../api/useAddGroupMembers';
import useRemoveGroupMembers from '../../../api/useRemoveGroupMembers';
import type {Group, Member} from '../../../models/group';

interface EditMembersSettingsProps {
  group: Group;
}

/**
 * Members tab content for the Group edit page.
 * Provides member listing, add, and remove functionality.
 */
export default function EditMembersSettings({group}: EditMembersSettingsProps): JSX.Element {
  const {t} = useTranslation('groups');
  const addGroupMembers = useAddGroupMembers();
  const removeGroupMembers = useRemoveGroupMembers();
  const [addDialogOpen, setAddDialogOpen] = useState(false);
  const [addError, setAddError] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const handleAddMembers = useCallback(
    (newMembers: Member[]) => {
      addGroupMembers.mutate(
        {
          groupId: group.id,
          members: newMembers,
        },
        {
          onSuccess: () => {
            setAddDialogOpen(false);
            setAddError(null);
          },
          onError: (err: Error) => {
            setAddError(getErrorMessage(err, t, 'addMember.error', 'Failed to add member. Please try again.'));
          },
        },
      );
    },
    [group.id, addGroupMembers, t],
  );

  const handleRemoveMember = useCallback(
    (memberToRemove: Member) => {
      removeGroupMembers.mutate(
        {
          groupId: group.id,
          members: [{id: memberToRemove.id, type: memberToRemove.type}],
        },
        {
          onSuccess: () => {
            setError(null);
          },
          onError: (err: Error) => {
            setError(getErrorMessage(err, t, 'removeMember.error', 'Failed to remove member. Please try again.'));
          },
        },
      );
    },
    [group.id, removeGroupMembers, t],
  );

  return (
    <Stack spacing={3}>
      {error && (
        <Alert severity="error" onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <ManageMembersSection
        groupId={group.id}
        onRemoveMember={handleRemoveMember}
        isReadOnly={group.isReadOnly}
        headerAction={
          !group.isReadOnly ? (
            <Button
              variant="contained"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={() => {
                setAddError(null);
                setAddDialogOpen(true);
              }}
            >
              {t('edit.members.sections.manage.addMember', 'Add Member')}
            </Button>
          ) : undefined
        }
      />

      {addDialogOpen && !group.isReadOnly && (
        <AddMemberDialog
          open={addDialogOpen}
          onClose={() => {
            setAddDialogOpen(false);
            setAddError(null);
          }}
          onAdd={handleAddMembers}
          groupId={group.id}
          excludeGroupId={group.id}
          error={addError}
          onErrorDismiss={() => setAddError(null)}
          isSubmitting={addGroupMembers.isPending}
        />
      )}
    </Stack>
  );
}
