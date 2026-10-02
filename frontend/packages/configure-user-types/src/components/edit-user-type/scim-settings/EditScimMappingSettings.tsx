// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Grid,
  Stack,
  Typography,
  Select,
  MenuItem,
  ListSubheader,
  Checkbox,
  FormControlLabel,
  Alert,
} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ScimPayloadPreview from './ScimPayloadPreview';
import {
  SCIM_CORE_TARGETS,
  SCIM_ENTERPRISE_TARGETS,
  SCIM_TARGET_GROUP_ORDER,
  SCIM_MULTI_VALUED_TARGETS,
  SCIM_MULTI_VALUED_TYPE_OPTIONS,
} from '../../../constants/scimTargets';
import type {SchemaPropertyInput, ScimMultiValuedMeta} from '../../../types/user-types';
import buildScimPreviewPayload from '../../../utils/buildScimPreviewPayload';

export interface EditScimMappingSettingsProps {
  properties: SchemaPropertyInput[];
  mapping: Record<string, string>;
  meta: Record<string, ScimMultiValuedMeta>;
  onMappingChange: (mapping: Record<string, string>) => void;
  onMetaChange: (meta: Record<string, ScimMultiValuedMeta>) => void;
  userTypeName: string;
  disabled?: boolean;
}

const MAPPABLE_TYPES = new Set(['string', 'number', 'boolean', 'enum']);
const ALL_TARGETS = [...SCIM_CORE_TARGETS, ...SCIM_ENTERPRISE_TARGETS];
const MULTI_VALUED_TARGET_SET = new Set<string>(SCIM_MULTI_VALUED_TARGETS);

export default function EditScimMappingSettings({
  properties,
  mapping,
  meta,
  onMappingChange,
  onMetaChange,
  userTypeName,
  disabled = false,
}: EditScimMappingSettingsProps): JSX.Element {
  const {t} = useTranslation();

  const eligibleProperties = properties.filter((p) => MAPPABLE_TYPES.has(p.type) && p.name.trim().length > 0);

  // Single-valued targets may only be picked by one property; multi-valued targets
  // (emails/phoneNumbers/photos) may be picked by several, each becoming its own array entry.
  const targetsUsedElsewhere = (propertyName: string): Set<string> =>
    new Set(
      Object.entries(mapping)
        .filter(([name, target]) => name !== propertyName && target && !MULTI_VALUED_TARGET_SET.has(target))
        .map(([, target]) => target),
    );

  // The type options and primary flag belong to the target they were set for, so a property starts clean
  // when it moves to another target.
  const handleTargetChange = (propertyName: string, target: string): void => {
    onMappingChange({...mapping, [propertyName]: target});
    if (meta[propertyName]) {
      onMetaChange(Object.fromEntries(Object.entries(meta).filter(([name]) => name !== propertyName)));
    }
  };

  // Only one property per multi-valued target can be primary: marking one clears the others.
  const handlePrimaryChange = (propertyName: string, target: string, primary: boolean): void => {
    const next = {...meta};
    if (primary) {
      Object.keys(next).forEach((name) => {
        if (name !== propertyName && mapping[name] === target && next[name].primary) {
          next[name] = {...next[name], primary: false};
        }
      });
    }
    next[propertyName] = {type: meta[propertyName]?.type ?? '', primary};
    onMetaChange(next);
  };

  const allPropertyNames = properties.map((p) => p.name.trim()).filter((name) => name.length > 0);
  const payload = buildScimPreviewPayload(mapping, meta, allPropertyNames, userTypeName);
  const hasUserNameMapped = Object.values(mapping).includes('userName');

  return (
    <Grid container spacing={3}>
      <Grid size={{xs: 12, lg: 7}}>
        <Stack>
          {!hasUserNameMapped && (
            <Alert severity="warning" sx={{mb: 1}}>
              {t(
                'userTypes:edit.scimMapping.userNameNotMappedWarning',
                'No property is mapped to the SCIM userName attribute. SCIM requires it - most SCIM clients expect it present.',
              )}
            </Alert>
          )}
          {eligibleProperties.map((prop) => {
            const propName = prop.name.trim();
            const excluded = targetsUsedElsewhere(propName);
            const currentValue = mapping[propName] ?? '';
            const isMultiValued = MULTI_VALUED_TARGET_SET.has(currentValue);
            const currentMeta = meta[propName];
            const isOnlyMapped = Object.values(mapping).filter((target) => target === currentValue).length === 1;
            const typeOptions = isMultiValued ? (SCIM_MULTI_VALUED_TYPE_OPTIONS[currentValue] ?? []) : [];

            return (
              <Stack
                key={prop.id}
                spacing={1}
                data-testid={`scim-mapping-row-${propName}`}
                sx={{
                  px: 1.5,
                  py: 1,
                  bgcolor: isMultiValued ? 'action.hover' : 'transparent',
                  borderRadius: 1,
                }}
              >
                <Stack direction="row" alignItems="center" spacing={2}>
                  <Typography variant="body2" sx={{minWidth: 160}}>
                    {propName}
                  </Typography>
                  <Select
                    value={currentValue}
                    onChange={(event) => handleTargetChange(propName, event.target.value)}
                    disabled={disabled}
                    size="small"
                    fullWidth
                    displayEmpty
                  >
                    <MenuItem value="">
                      {t('userTypes:edit.scimMapping.notMapped', 'Not mapped (custom attribute)')}
                    </MenuItem>
                    {SCIM_TARGET_GROUP_ORDER.flatMap((group) => {
                      const groupTargets = ALL_TARGETS.filter(
                        (target) =>
                          target.group === group && (!excluded.has(target.value) || currentValue === target.value),
                      );
                      if (groupTargets.length === 0) return [];
                      return [
                        <ListSubheader key={`group-${group}`}>{group}</ListSubheader>,
                        ...groupTargets.map((target) => (
                          <MenuItem key={target.value} value={target.value}>
                            {target.label}
                          </MenuItem>
                        )),
                      ];
                    })}
                  </Select>
                </Stack>

                {isMultiValued && (
                  <Stack direction="row" alignItems="center" spacing={2} sx={{pl: '176px'}}>
                    <Select
                      value={currentMeta?.type ?? ''}
                      onChange={(event) =>
                        onMetaChange({
                          ...meta,
                          [propName]: {primary: currentMeta?.primary ?? false, type: event.target.value},
                        })
                      }
                      disabled={disabled}
                      size="small"
                      displayEmpty
                      sx={{minWidth: 140}}
                    >
                      <MenuItem value="">{t('userTypes:edit.scimMapping.typePlaceholder', 'Type (optional)')}</MenuItem>
                      {typeOptions.map((option) => (
                        <MenuItem key={option} value={option}>
                          {option}
                        </MenuItem>
                      ))}
                    </Select>
                    <FormControlLabel
                      control={
                        <Checkbox
                          size="small"
                          checked={isOnlyMapped || (currentMeta?.primary ?? false)}
                          disabled={disabled || isOnlyMapped}
                          onChange={(event) => handlePrimaryChange(propName, currentValue, event.target.checked)}
                        />
                      }
                      label={t('userTypes:edit.scimMapping.primaryLabel', 'Primary')}
                    />
                  </Stack>
                )}
              </Stack>
            );
          })}
        </Stack>
      </Grid>
      <Grid size={{xs: 12, lg: 5}} sx={{minWidth: 0}}>
        <ScimPayloadPreview payload={payload} />
      </Grid>
    </Grid>
  );
}
