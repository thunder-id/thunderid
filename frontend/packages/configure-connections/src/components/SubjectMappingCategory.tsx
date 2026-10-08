// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Button, Stack, Typography} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import SubjectMappingGroupEditor from './SubjectMappingGroupEditor';
import type {SubjectMappingGroup, SubjectTypeOption} from '../models/subject-mapping';

interface SubjectMappingCategoryProps {
  category: 'agent' | 'user';
  groups: SubjectMappingGroup[];
  subjectTypes: SubjectTypeOption[];
  onAddGroup: () => void;
  onUpdateGroupType: (groupKey: number, userType: string) => void;
  onAddRow: (groupKey: number) => void;
  onRemoveRow: (groupKey: number, rowKey: number) => void;
  onUpdateRow: (groupKey: number, rowKey: number, part: 'attribute' | 'pdpAttribute', value: string) => void;
  onRemoveGroup: (groupKey: number) => void;
}

export default function SubjectMappingCategory({
  category,
  groups,
  subjectTypes,
  onAddGroup,
  onUpdateGroupType,
  onAddRow,
  onRemoveRow,
  onUpdateRow,
  onRemoveGroup,
}: SubjectMappingCategoryProps): JSX.Element | null {
  const {t} = useTranslation('connections');
  const categoryGroups = groups.filter((group) => group.category === category);
  const categoryTypes = subjectTypes.filter((type) => type.category === category);
  if (categoryTypes.length === 0) {
    return null;
  }
  const usedTypeHandles = new Set(
    categoryGroups.map((group) => group.userType).filter((userType) => userType.trim() !== ''),
  );
  const showAddMapping = categoryTypes.some((type) => !usedTypeHandles.has(type.handle));
  const sectionTitle =
    category === 'user'
      ? t('subjectMapping.mappings.user', 'User mappings')
      : t('subjectMapping.mappings.agent', 'Agent mappings');
  const addLabel =
    category === 'user'
      ? t('subjectMapping.mappings.addUser', 'Add User Mapping')
      : t('subjectMapping.mappings.addAgent', 'Add Agent Mapping');

  return (
    <Stack direction="column" spacing={2} data-testid={`subject-mapping-category-${category}`}>
      <Typography variant="subtitle2">{sectionTitle}</Typography>
      {categoryGroups.map((group) => (
        <SubjectMappingGroupEditor
          key={group.key}
          group={group}
          subjectTypes={categoryTypes}
          otherUsedUserTypes={categoryGroups
            .filter((other) => other.key !== group.key)
            .map((other) => other.userType)
            .filter((userType) => userType.trim() !== '')}
          canRemove={categoryGroups.length > 1}
          onUserTypeChange={(userType) => onUpdateGroupType(group.key, userType)}
          onAddRow={() => onAddRow(group.key)}
          onRemoveRow={(rowKey) => onRemoveRow(group.key, rowKey)}
          onUpdateRow={(rowKey, part, value) => onUpdateRow(group.key, rowKey, part, value)}
          onRemoveGroup={() => onRemoveGroup(group.key)}
        />
      ))}
      {showAddMapping && (
        <Box>
          <Button
            variant="text"
            color="primary"
            size="small"
            startIcon={<Plus size={16} />}
            onClick={onAddGroup}
            data-testid={`subject-mapping-add-${category}`}
          >
            {addLabel}
          </Button>
        </Box>
      )}
    </Stack>
  );
}
