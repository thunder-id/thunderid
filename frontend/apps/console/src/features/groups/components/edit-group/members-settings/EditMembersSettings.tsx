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
  const {t} = useTranslation();
  const addGroupMembers = useAddGroupMembers();
  const removeGroupMembers = useRemoveGroupMembers();
  const [addDialogOpen, setAddDialogOpen] = useState(false);
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
            setError(null);
          },
          onError: (err: Error) => {
            setError(err.message ?? t('groups:addMember.error'));
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
            setError(err.message ?? t('groups:removeMember.error'));
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
              onClick={() => setAddDialogOpen(true)}
            >
              {t('groups:edit.members.sections.manage.addMember')}
            </Button>
          ) : undefined
        }
      />

      {addDialogOpen && !group.isReadOnly && (
        <AddMemberDialog open={addDialogOpen} onClose={() => setAddDialogOpen(false)} onAdd={handleAddMembers} />
      )}
    </Stack>
  );
}
