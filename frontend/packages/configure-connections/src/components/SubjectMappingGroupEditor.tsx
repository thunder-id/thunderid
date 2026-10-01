// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetAgentType} from '@thunderid/configure-agent-types';
import {useGetUserType} from '@thunderid/configure-user-types';
import {
  Autocomplete,
  Box,
  Button,
  FormControl,
  FormLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Link2, Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {BUILT_IN_OPTIONAL_SUBJECT_ATTRIBUTES} from '../constants/subject-mapping';
import type {SubjectMappingGroup, SubjectTypeOption} from '../models/subject-mapping';
import {flattenUserTypeAttributes} from '../utils/attributeConfiguration';
import {withStoredOption} from '../utils/selectOptions';
import {isDuplicateSubjectAttribute} from '../utils/subjectMapping';

interface SubjectMappingGroupEditorProps {
  group: SubjectMappingGroup;
  subjectTypes: SubjectTypeOption[];
  otherUsedUserTypes: string[];
  canRemove: boolean;
  onUserTypeChange: (userType: string) => void;
  onAddRow: () => void;
  onRemoveRow: (rowKey: number) => void;
  onUpdateRow: (rowKey: number, part: 'attribute' | 'pdpAttribute', value: string) => void;
  onRemoveGroup: () => void;
}

export default function SubjectMappingGroupEditor({
  group,
  subjectTypes,
  otherUsedUserTypes,
  canRemove,
  onUserTypeChange,
  onAddRow,
  onRemoveRow,
  onUpdateRow,
  onRemoveGroup,
}: SubjectMappingGroupEditorProps): JSX.Element {
  const {t} = useTranslation('connections');
  const [pdpAttributeRows, setPdpAttributeRows] = useState<Set<number>>(new Set());
  const selectedType = subjectTypes.find((type) => type.name === group.userType && type.category === group.category);
  const userTypeDetail = useGetUserType(group.category === 'user' ? selectedType?.id : undefined);
  const agentTypeDetail = useGetAgentType(group.category === 'agent' ? selectedType?.id : undefined);
  const userAttributes = useMemo(
    () => flattenUserTypeAttributes(userTypeDetail.data?.schema ?? agentTypeDetail.data?.schema),
    [agentTypeDetail.data?.schema, userTypeDetail.data?.schema],
  );
  const subjectAttributeOptions = useMemo(
    () => [...new Set([...BUILT_IN_OPTIONAL_SUBJECT_ATTRIBUTES, ...userAttributes])].sort(),
    [userAttributes],
  );
  const userTypeOptions = useMemo(() => {
    const usedElsewhere = new Set(otherUsedUserTypes);
    return withStoredOption(
      subjectTypes.filter((type) => !usedElsewhere.has(type.name)).map((type) => type.name),
      group.userType,
    );
  }, [group.userType, otherUsedUserTypes, subjectTypes]);
  const entityTypeLabel =
    group.category === 'user'
      ? t('subjectMapping.attributes.userType.label', 'User type')
      : t('subjectMapping.attributes.agentType.label', 'Agent type');
  const entityTypePlaceholder =
    group.category === 'user'
      ? t('subjectMapping.attributes.userType.placeholder', 'Select a user type')
      : t('subjectMapping.attributes.agentType.placeholder', 'Select an agent type');
  const hasOnlyAvailableType = userTypeOptions.length === 1 && group.userType === userTypeOptions[0];
  const showEntityType = !hasOnlyAvailableType;
  const lastRow = group.rows[group.rows.length - 1];
  const lastRowIsEmpty = lastRow?.attribute.trim() === '' && lastRow?.pdpAttribute.trim() === '';

  return (
    <Box
      sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, p: 2}}
      data-testid={`subject-mapping-group-${group.key}`}
    >
      <Stack direction="row" spacing={2} alignItems="flex-end" sx={{mb: 2}}>
        {showEntityType && hasOnlyAvailableType && (
          <FormControl sx={{minWidth: 220}}>
            <FormLabel sx={{mb: 0.75}} htmlFor={`subject-mapping-group-user-type-${group.key}`}>
              {entityTypeLabel}
            </FormLabel>
            <TextField
              id={`subject-mapping-group-user-type-${group.key}`}
              value={group.userType}
              slotProps={{input: {readOnly: true}}}
              data-testid={`subject-mapping-group-user-type-value-${group.key}`}
            />
          </FormControl>
        )}
        {showEntityType && !hasOnlyAvailableType && (
          <FormControl sx={{minWidth: 220}}>
            <FormLabel sx={{mb: 0.75}} htmlFor={`subject-mapping-group-user-type-${group.key}`}>
              {entityTypeLabel}
            </FormLabel>
            <Select
              id={`subject-mapping-group-user-type-${group.key}`}
              displayEmpty
              value={group.userType}
              onChange={(event) => onUserTypeChange(event.target.value)}
              renderValue={(value) => (value ? value : entityTypePlaceholder)}
              data-testid={`subject-mapping-group-user-type-select-${group.key}`}
            >
              {userTypeOptions.map((name) => (
                <MenuItem key={name} value={name}>
                  {name}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        )}
        <Box sx={{flex: 1}} />
        {canRemove && (
          <Button
            variant="text"
            color="error"
            size="small"
            startIcon={<Trash2 size={16} />}
            onClick={onRemoveGroup}
            data-testid={`subject-mapping-group-remove-${group.key}`}
          >
            {t('subjectMapping.mappings.remove', 'Remove')}
          </Button>
        )}
      </Stack>
      <Stack direction="column" spacing={1.5}>
        <Stack direction="row" spacing={1.5}>
          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
            {t('subjectMapping.mappings.thunderIdAttribute', 'ThunderID Attribute')}
          </Typography>
          <Box sx={{width: 40}} />
        </Stack>
        {group.rows.map((row, index) => {
          const isOnlyEmptyRow =
            group.rows.length === 1 && row.attribute.trim() === '' && row.pdpAttribute.trim() === '';
          const attributesUsedByOtherRows = new Set(
            group.rows
              .filter((otherRow) => otherRow.key !== row.key)
              .map((otherRow) => otherRow.attribute.trim())
              .filter((attribute) => attribute !== ''),
          );
          const availableSubjectAttributeOptions = subjectAttributeOptions.filter(
            (attribute) => !attributesUsedByOtherRows.has(attribute),
          );
          const duplicateAttribute = isDuplicateSubjectAttribute(group.rows, row.attribute);
          const hasPDPAttributeRename = row.pdpAttribute.trim() !== '' && row.pdpAttribute !== row.attribute;
          const showPDPAttribute = hasPDPAttributeRename || pdpAttributeRows.has(row.key);
          return (
            <Stack key={row.key} direction="column" spacing={1}>
              <Stack direction="row" spacing={1.5} alignItems="center">
                <Autocomplete
                  fullWidth
                  freeSolo
                  sx={{width: 'calc(50% - 20px)'}}
                  options={availableSubjectAttributeOptions}
                  inputValue={row.attribute}
                  onInputChange={(_event, nextValue) => onUpdateRow(row.key, 'attribute', nextValue)}
                  renderInput={(params) => (
                    <TextField
                      {...params}
                      id={`subject-mapping-attribute-${group.key}-${index + 1}`}
                      placeholder={t('subjectMapping.mappings.thunderIdPlaceholder', 'e.g. email')}
                      inputProps={{
                        ...params.inputProps,
                        'aria-label': t('subjectMapping.mappings.thunderIdAttribute', 'ThunderID Attribute'),
                      }}
                      error={duplicateAttribute}
                      helperText={
                        duplicateAttribute
                          ? t(
                              'subjectMapping.mappings.duplicateAttribute',
                              'Each ThunderID attribute can be mapped only once for an entity type.',
                            )
                          : undefined
                      }
                    />
                  )}
                />
                {showPDPAttribute ? (
                  <TextField
                    fullWidth
                    sx={{width: 'calc(50% - 20px)'}}
                    id={`subject-mapping-pdp-attribute-${group.key}-${index + 1}`}
                    label={t('subjectMapping.mappings.pdpAttribute', 'PDP Attribute')}
                    value={row.pdpAttribute}
                    onChange={(event) => onUpdateRow(row.key, 'pdpAttribute', event.target.value)}
                  />
                ) : (
                  <Tooltip title={t('subjectMapping.mappings.mapPdpAttribute', 'Map to a different PDP attribute')}>
                    <IconButton
                      size="small"
                      aria-label={t('subjectMapping.mappings.mapPdpAttribute', 'Map to a different PDP attribute')}
                      onClick={() => setPdpAttributeRows((previous) => new Set(previous).add(row.key))}
                      data-testid={`subject-mapping-map-pdp-attribute-${group.key}-${index + 1}`}
                    >
                      <Link2 size={16} />
                    </IconButton>
                  </Tooltip>
                )}
                {isOnlyEmptyRow ? (
                  <Box sx={{width: 40}} />
                ) : (
                  <IconButton
                    onClick={() => onRemoveRow(row.key)}
                    aria-label={t('form.keyValue.remove', 'Remove')}
                    data-testid={`subject-mapping-remove-${group.key}-${index + 1}`}
                  >
                    <Trash2 size={16} />
                  </IconButton>
                )}
              </Stack>
            </Stack>
          );
        })}
        <Box>
          <Button
            variant="text"
            size="small"
            startIcon={<Plus size={16} />}
            onClick={onAddRow}
            disabled={lastRowIsEmpty}
            data-testid={`subject-mapping-add-${group.key}`}
          >
            {t('subjectMapping.mappings.add', 'Add Mapping')}
          </Button>
        </Box>
      </Stack>
    </Box>
  );
}
