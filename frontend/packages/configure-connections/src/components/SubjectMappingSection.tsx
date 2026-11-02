// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Alert, Box, Button, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {Route, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import SettingsCardIcon from './SettingsCardIcon';
import SubjectMappingCategory from './SubjectMappingCategory';
import useSubjectMappingTypes from '../api/useSubjectMappingTypes';
import type {SubjectMappingValues} from '../models/connection';
import type {SubjectMappingGroup, SubjectRowsState, SubjectTypeOption} from '../models/subject-mapping';
import {sanitizeKeyValuePart} from '../utils/keyValuePairs';
import {
  buildSubjectRows,
  canonicalSubjectAttributeMappings,
  serializeSubjectAttributeMappings,
} from '../utils/subjectMapping';

interface SubjectMappingSectionProps {
  values: SubjectMappingValues;
  onChange: <K extends keyof SubjectMappingValues>(field: K, value: NonNullable<SubjectMappingValues[K]>) => void;
}

export default function SubjectMappingSection({values, onChange}: SubjectMappingSectionProps): JSX.Element {
  const {t} = useTranslation('connections');
  const subjectMappingTypesQuery = useSubjectMappingTypes();
  const subjectTypesLoading = subjectMappingTypesQuery.isLoading;
  const subjectTypesError = subjectMappingTypesQuery.error;
  const userTypeList = useMemo(() => subjectMappingTypesQuery.data?.userTypes ?? [], [subjectMappingTypesQuery.data]);
  const agentTypeList = useMemo(() => subjectMappingTypesQuery.data?.agentTypes ?? [], [subjectMappingTypesQuery.data]);
  const subjectTypes = useMemo<SubjectTypeOption[]>(
    () => [...userTypeList, ...agentTypeList],
    [agentTypeList, userTypeList],
  );
  const [state, setState] = useState<SubjectRowsState>(() => buildSubjectRows(values.subjectAttributeMappings, 0));

  if (canonicalSubjectAttributeMappings(values.subjectAttributeMappings) !== state.syncedAttributeMappings) {
    setState(buildSubjectRows(values.subjectAttributeMappings, state.seq));
  }
  const groups = useMemo(() => {
    const categoryResolvedGroups = state.groups.map((group) => {
      if (group.category === 'user' || group.category === 'agent') return group;
      const matches = subjectTypes.filter((type) => type.handle === group.userType);
      return matches.length === 1 ? {...group, category: matches[0].category} : group;
    });
    return categoryResolvedGroups.map((group) => {
      if (group.userType !== '') return group;
      const used = new Set(
        categoryResolvedGroups
          .filter((other) => other.key !== group.key && other.category === group.category)
          .map((other) => other.userType)
          .filter(Boolean),
      );
      const available = subjectTypes.filter((type) => type.category === group.category && !used.has(type.handle));
      return available.length === 1 ? {...group, userType: available[0].handle} : group;
    });
  }, [state.groups, subjectTypes]);

  const commit = (updatedGroups: SubjectMappingGroup[], seq: number): void => {
    const mappings = serializeSubjectAttributeMappings(updatedGroups);
    setState({groups: updatedGroups, seq, syncedAttributeMappings: canonicalSubjectAttributeMappings(mappings)});
    onChange('subjectAttributeMappings', mappings);
  };
  const updateGroupType = (key: number, userType: string): void =>
    commit(
      groups.map((group) => (group.key === key ? {...group, userType} : group)),
      state.seq,
    );
  const addGroup = (category: 'agent' | 'user'): void =>
    setState((previous) => {
      const used = new Set(
        previous.groups
          .filter((group) => group.category === category)
          .map((group) => group.userType)
          .filter(Boolean),
      );
      const available = subjectTypes.filter((type) => type.category === category && !used.has(type.handle));
      return {
        ...previous,
        groups: [
          ...previous.groups,
          {
            key: previous.seq + 1,
            category,
            userType: available.length === 1 ? available[0].handle : '',
            rows: [{key: previous.seq + 2, attribute: '', pdpAttribute: ''}],
          },
        ],
        seq: previous.seq + 2,
      };
    });
  const removeGroup = (key: number): void =>
    commit(
      groups.filter((group) => group.key !== key),
      state.seq,
    );
  const addRow = (key: number): void =>
    setState((previous) => ({
      ...previous,
      groups: previous.groups.map((group) =>
        group.key === key
          ? {...group, rows: [...group.rows, {key: previous.seq + 1, attribute: '', pdpAttribute: ''}]}
          : group,
      ),
      seq: previous.seq + 1,
    }));
  const removeRow = (groupKey: number, rowKey: number): void => {
    const updated = groups.map((group) => {
      if (group.key !== groupKey) return group;
      const rows = group.rows.filter((row) => row.key !== rowKey);
      return {...group, rows: rows.length > 0 ? rows : [{key: state.seq + 1, attribute: '', pdpAttribute: ''}]};
    });
    commit(updated, state.seq + 1);
  };
  const updateRow = (groupKey: number, rowKey: number, part: 'attribute' | 'pdpAttribute', value: string): void => {
    const sanitized = sanitizeKeyValuePart(value, part === 'attribute' ? 'name' : 'value');
    commit(
      groups.map((group) =>
        group.key === groupKey
          ? {...group, rows: group.rows.map((row) => (row.key === rowKey ? {...row, [part]: sanitized} : row))}
          : group,
      ),
      state.seq,
    );
  };
  const unresolvedGroups = groups.filter((group) => group.category === '');

  return (
    <Stack direction="column" spacing={3} data-testid="subject-mapping-section">
      <SettingsCard
        title={t('subjectMapping.attributes.title', 'Subject attribute mapping')}
        description={t(
          'subjectMapping.attributes.description',
          'Choose user or agent attributes this PDP needs and optionally rename them for the AuthZEN request.',
        )}
        titleIcon={
          <SettingsCardIcon>
            <Route size={16} />
          </SettingsCardIcon>
        }
      >
        {subjectTypesLoading ? (
          <Alert severity="info" icon={<CircularProgress size={16} />} data-testid="subject-mapping-types-loading">
            {t('subjectMapping.mappings.loadingTypes', 'Loading user and agent types...')}
          </Alert>
        ) : subjectTypesError ? (
          <Alert
            severity="error"
            data-testid="subject-mapping-types-error"
            action={
              <Button
                color="inherit"
                size="small"
                onClick={() => {
                  void subjectMappingTypesQuery.refetch();
                }}
              >
                {t('common:actions.retry', 'Retry')}
              </Button>
            }
          >
            {t('subjectMapping.mappings.loadTypesError', 'Failed to load user and agent types.')}
          </Alert>
        ) : (
          <Stack direction="column" spacing={3}>
            <SubjectMappingCategory
              category="user"
              groups={groups}
              subjectTypes={subjectTypes}
              onAddGroup={() => addGroup('user')}
              onUpdateGroupType={updateGroupType}
              onAddRow={addRow}
              onRemoveRow={removeRow}
              onUpdateRow={updateRow}
              onRemoveGroup={removeGroup}
            />
            <SubjectMappingCategory
              category="agent"
              groups={groups}
              subjectTypes={subjectTypes}
              onAddGroup={() => addGroup('agent')}
              onUpdateGroupType={updateGroupType}
              onAddRow={addRow}
              onRemoveRow={removeRow}
              onUpdateRow={updateRow}
              onRemoveGroup={removeGroup}
            />
            {unresolvedGroups.length > 0 && (
              <Stack direction="column" spacing={2} data-testid="subject-mapping-unresolved">
                <Alert severity="warning">
                  {t(
                    'subjectMapping.mappings.unresolved.warning',
                    'These mappings could not be matched to exactly one user or agent type. Remove them before saving this connection.',
                  )}
                </Alert>
                {unresolvedGroups.map((group) => {
                  const attributes = group.rows
                    .filter((row) => row.attribute.trim() !== '')
                    .map((row) =>
                      row.pdpAttribute.trim() !== '' && row.pdpAttribute !== row.attribute
                        ? `${row.attribute} → ${row.pdpAttribute}`
                        : row.attribute,
                    );
                  return (
                    <Box
                      key={group.key}
                      sx={{border: '1px solid', borderColor: 'warning.main', borderRadius: 2, p: 2}}
                      data-testid={`subject-mapping-unresolved-${group.key}`}
                    >
                      <Stack direction="row" spacing={2} alignItems="center">
                        <Box sx={{flex: 1}}>
                          <Typography variant="subtitle2">
                            {t('subjectMapping.mappings.unresolved.entityType', 'Entity type: {{entityType}}', {
                              entityType: group.userType,
                            })}
                          </Typography>
                          {attributes.length > 0 && (
                            <Typography variant="body2" color="text.secondary">
                              {t('subjectMapping.mappings.unresolved.attributes', 'Attributes: {{attributes}}', {
                                attributes: attributes.join(', '),
                              })}
                            </Typography>
                          )}
                        </Box>
                        <Button
                          variant="text"
                          color="error"
                          size="small"
                          startIcon={<Trash2 size={16} />}
                          onClick={() => removeGroup(group.key)}
                          data-testid={`subject-mapping-unresolved-remove-${group.key}`}
                        >
                          {t('subjectMapping.mappings.remove', 'Remove')}
                        </Button>
                      </Stack>
                    </Box>
                  );
                })}
              </Stack>
            )}
            <Typography variant="caption" color="text.secondary">
              {t(
                'subjectMapping.mappings.hint',
                'Select extra user or agent attributes to include in the PDP request. The PDP attribute is optional and is only needed when the PDP expects a different name.',
              )}
            </Typography>
          </Stack>
        )}
      </SettingsCard>
    </Stack>
  );
}
