// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Grid,
  Stack,
  Typography,
  Select,
  MenuItem,
  Checkbox,
  FormControlLabel,
  Alert,
  Button,
  Chip,
  IconButton,
  Menu,
  TextField,
  Tooltip,
} from '@wso2/oxygen-ui';
import {Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useState} from 'react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ScimPayloadPreview from './ScimPayloadPreview';
import {
  SCIM_CORE_TARGETS,
  SCIM_ENTERPRISE_TARGETS,
  SCIM_MAPPABLE_PROPERTY_TYPES,
  SCIM_MULTI_VALUED_TARGETS,
  SCIM_MULTI_VALUED_TYPE_OPTIONS,
} from '../../../constants/scimTargets';
import type {SchemaPropertyInput, ScimMultiValuedMeta} from '../../../types/user-types';
import buildScimPreviewPayload from '../../../utils/buildScimPreviewPayload';
import {addDefaultMultiValuedTypes, getUsedMultiValuedTypes} from '../../../utils/multiValuedTypeDefaults';

export interface EditScimMappingSettingsProps {
  properties: SchemaPropertyInput[];
  mapping: Record<string, string>;
  meta: Record<string, ScimMultiValuedMeta>;
  onMappingChange: (mapping: Record<string, string>) => void;
  onMetaChange: (meta: Record<string, ScimMultiValuedMeta>) => void;
  userTypeHandle: string;
  scimCoreUserType: boolean;
  onRequestScimCoreChange: () => void;
  /** Display name of the user type that is currently the SCIM core type, when it is another one. */
  currentCoreUserTypeName?: string | null;
  /** The only user type is always the SCIM core type. */
  scimCoreLocked?: boolean;
  disabled?: boolean;
}

const ALL_TARGET_VALUES = [...SCIM_CORE_TARGETS, ...SCIM_ENTERPRISE_TARGETS].map((target) => target.value);
// The SCIM attribute column, aligned with the Type and Primary controls under it.
const LEFT_COLUMN = '0 0 45%';
const MULTI_VALUED_TARGET_SET = new Set<string>(SCIM_MULTI_VALUED_TARGETS);
const NO_MAPPING: Record<string, string> = {};
const NO_META: Record<string, ScimMultiValuedMeta> = {};

export default function EditScimMappingSettings({
  properties,
  mapping: storedMapping,
  meta: storedMeta,
  onMappingChange,
  onMetaChange,
  userTypeHandle,
  scimCoreUserType,
  onRequestScimCoreChange,
  currentCoreUserTypeName = null,
  scimCoreLocked = false,
  disabled = false,
}: EditScimMappingSettingsProps): JSX.Element {
  const {t} = useTranslation();
  // Only the core user type is exposed through SCIM core attributes, so any other type reads as unmapped.
  const mapping = scimCoreUserType ? storedMapping : NO_MAPPING;
  const meta = scimCoreUserType ? storedMeta : NO_META;
  // The custom attribute chip whose SCIM attribute menu is open.
  const [menuState, setMenuState] = useState<{anchor: HTMLElement; property: string} | null>(null);

  // The multi-valued SCIM attribute whose "Add another" menu is open.
  const [addAnotherState, setAddAnotherState] = useState<{anchor: HTMLElement; target: string} | null>(null);

  // Tracks which property is currently being given a custom type value.
  const [customTypeEditId, setCustomTypeEditId] = useState<string | null>(null);
  const [customTypeInput, setCustomTypeInput] = useState('');

  const eligiblePropertyNames = properties
    .filter((p) => SCIM_MAPPABLE_PROPERTY_TYPES.has(p.type) && p.name.trim().length > 0)
    .map((p) => p.name.trim());

  // The display order is kept apart from the mapping object, whose key order changes when it is
  // saved and reloaded. Mappings that arrive together (load, auto-map) start in SCIM order; one
  // the user adds goes to the end instead of re-sorting the rest.
  const mappedNames = Object.keys(mapping).filter((name) => mapping[name] && eligiblePropertyNames.includes(name));
  const [order, setOrder] = useState<string[]>([]);
  const keptNames = order.filter((name) => mappedNames.includes(name));
  const newNames = mappedNames
    .filter((name) => !keptNames.includes(name))
    .sort((a, b) => ALL_TARGET_VALUES.indexOf(mapping[a]) - ALL_TARGET_VALUES.indexOf(mapping[b]));
  const orderedNames = [...keptNames, ...newNames];
  if (orderedNames.length !== order.length || orderedNames.some((name, index) => name !== order[index])) {
    setOrder(orderedNames);
  }

  const userNameProperty = mappedNames.find((name) => mapping[name] === 'userName') ?? '';

  // userName is always the first fixed row; other mapped SCIM attributes follow in the order they first appear.
  const rows = orderedNames.reduce<{target: string; propertyNames: string[]}[]>(
    (acc, propertyName) => {
      const target = mapping[propertyName];
      if (target === 'userName') return acc;
      const existing = acc.find((row) => row.target === target);
      if (existing) {
        existing.propertyNames.push(propertyName);
      } else {
        acc.push({target, propertyNames: [propertyName]});
      }
      return acc;
    },
    [{target: 'userName', propertyNames: [userNameProperty]}],
  );

  // A single-valued SCIM attribute can be mapped once; multi-valued ones keep accepting more properties.
  const usedTargets = new Set(mappedNames.map((name) => mapping[name]));
  const availableTargets = ALL_TARGET_VALUES.filter(
    (target) => MULTI_VALUED_TARGET_SET.has(target) || !usedTargets.has(target),
  );
  const unmappedPropertyNames = eligiblePropertyNames.filter((name) => !mapping[name]);

  // The type options and primary flag belong to the target they were set for, so a property starts
  // clean when it is removed or replaced.
  const dropMeta = (propertyName: string): void => {
    if (meta[propertyName]) {
      onMetaChange(Object.fromEntries(Object.entries(meta).filter(([name]) => name !== propertyName)));
    }
  };

  const commitMeta = (next: Record<string, ScimMultiValuedMeta>): void => {
    const keys = Object.keys(next);
    if (keys.length !== Object.keys(meta).length || keys.some((name) => next[name] !== meta[name])) {
      onMetaChange(next);
    }
  };

  // A property newly mapped to a multi-valued target starts with the first type the others do not use.
  const handleMap = (propertyName: string, nextMapping: Record<string, string>): void => {
    onMappingChange(nextMapping);
    commitMeta(addDefaultMultiValuedTypes(nextMapping, meta, [propertyName]));
  };

  // Replaces the property in place so the row keeps its position.
  const handlePropertyChange = (oldName: string, newName: string): void => {
    setOrder(orderedNames.map((name) => (name === oldName ? newName : name)));
    const nextMapping = Object.fromEntries(
      Object.entries(mapping).map(([name, target]) => [name === oldName ? newName : name, target]),
    );
    onMappingChange(nextMapping);
    const withoutOld = Object.fromEntries(Object.entries(meta).filter(([name]) => name !== oldName));
    commitMeta(addDefaultMultiValuedTypes(nextMapping, withoutOld, [newName]));
  };

  const handleRemove = (propertyName: string): void => {
    onMappingChange(Object.fromEntries(Object.entries(mapping).filter(([name]) => name !== propertyName)));
    dropMeta(propertyName);
  };

  const handleTargetPick = (target: string): void => {
    if (menuState) {
      handleMap(menuState.property, {...mapping, [menuState.property]: target});
    }
    setMenuState(null);
  };

  const handleAddAnotherPick = (propertyName: string): void => {
    if (addAnotherState) {
      handleMap(propertyName, {...mapping, [propertyName]: addAnotherState.target});
    }
    setAddAnotherState(null);
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

  const commitCustomType = (propName: string, value: string): void => {
    const trimmed = value.trim();
    if (trimmed) {
      onMetaChange({
        ...meta,
        [propName]: {primary: meta[propName]?.primary ?? false, type: trimmed},
      });
    }
    setCustomTypeEditId(null);
    setCustomTypeInput('');
  };

  const allPropertyNames = properties.map((p) => p.name.trim()).filter((name) => name.length > 0);
  // Entries of properties that are no longer mappable (for example after a type change) are ignored.
  const effectiveMapping = Object.fromEntries(mappedNames.map((name) => [name, mapping[name]]));
  const payload = buildScimPreviewPayload(effectiveMapping, meta, allPropertyNames, userTypeHandle);
  const hasUserNameMapped = userNameProperty !== '';
  const customAttributeNames = allPropertyNames.filter((name) => !effectiveMapping[name]);

  const selectLabel = t('userTypes:edit.scimMapping.select', 'Select');
  const removeLabel = t('userTypes:edit.scimMapping.removeMapping', 'Remove mapping');

  const renderPropertySelect = (
    value: string,
    onChange: (next: string) => void,
    options: string[],
    onDelete?: () => void,
    deleteLabel?: string,
  ): JSX.Element => (
    <Stack direction="row" alignItems="center" spacing={1}>
      <Select
        value={value}
        onChange={(event) => onChange(event.target.value)}
        disabled={disabled}
        size="small"
        displayEmpty
        renderValue={(selected) => selected || selectLabel}
        sx={{flex: 1, minWidth: 0}}
      >
        {options.map((name) => (
          <MenuItem key={name} value={name}>
            {name}
          </MenuItem>
        ))}
      </Select>
      {onDelete && deleteLabel ? (
        <Tooltip title={deleteLabel}>
          <span>
            <IconButton size="small" aria-label={deleteLabel} disabled={disabled} onClick={onDelete}>
              <Trash2 size={16} />
            </IconButton>
          </span>
        </Tooltip>
      ) : (
        <IconButton size="small" disabled tabIndex={-1} aria-hidden="true" sx={{visibility: 'hidden'}}>
          <Trash2 size={16} />
        </IconButton>
      )}
    </Stack>
  );

  const renderPrimaryCheckbox = (propName: string, target: string, isOnlyMapped: boolean): JSX.Element => (
    <FormControlLabel
      control={
        <Checkbox
          size="small"
          checked={isOnlyMapped || (meta[propName]?.primary ?? false)}
          disabled={disabled || isOnlyMapped}
          onChange={(event) => handlePrimaryChange(propName, target, event.target.checked)}
        />
      }
      label={t('userTypes:edit.scimMapping.primaryLabel', 'Primary')}
      sx={{mr: 0, flexShrink: 0}}
    />
  );

  /**
   * Type dropdown with a "+ Custom type..." menu item at the bottom.
   * Selecting it replaces the dropdown with an inline text field; Enter or blur commits it.
   */
  const renderTypeWithCustom = (propName: string, target: string): JSX.Element => {
    const currentMeta = meta[propName];
    const predefined = SCIM_MULTI_VALUED_TYPE_OPTIONS[target] ?? [];
    // A type another property of this target already uses is not offered again.
    const usedByOthers = getUsedMultiValuedTypes(target, propName, mapping, meta);
    const isCustomType = Boolean(currentMeta?.type && !predefined.includes(currentMeta.type));
    const isEditingCustom = customTypeEditId === propName;
    const defineCustomTypeLabel = t('userTypes:edit.scimMapping.defineCustomType', 'Custom type...');
    const CUSTOM_TYPE_OPTION = '__custom__';

    if (isEditingCustom) {
      return (
        <TextField
          size="small"
          inputRef={(el: HTMLInputElement | null) => el?.focus()}
          placeholder={t('userTypes:edit.scimMapping.customTypePlaceholder', 'Custom type')}
          value={customTypeInput}
          onChange={(e) => setCustomTypeInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              commitCustomType(propName, customTypeInput);
            } else if (e.key === 'Escape') {
              setCustomTypeEditId(null);
              setCustomTypeInput('');
            }
          }}
          onBlur={() => commitCustomType(propName, customTypeInput)}
          sx={{width: 130, flexShrink: 0}}
          inputProps={{'aria-label': defineCustomTypeLabel}}
        />
      );
    }

    return (
      <Select
        value={currentMeta?.type ?? ''}
        onChange={(event) => {
          if (event.target.value === CUSTOM_TYPE_OPTION) {
            setCustomTypeInput(isCustomType ? (currentMeta?.type ?? '') : '');
            setCustomTypeEditId(propName);
            return;
          }
          onMetaChange({
            ...meta,
            [propName]: {primary: currentMeta?.primary ?? false, type: event.target.value},
          });
        }}
        disabled={disabled}
        size="small"
        displayEmpty
        renderValue={(selected) => selected || t('userTypes:edit.scimMapping.typeShort', 'Type')}
        sx={{width: 120, flexShrink: 0}}
      >
        {predefined
          .filter((option) => option === currentMeta?.type || !usedByOthers.has(option))
          .map((option) => (
            <MenuItem key={option} value={option}>
              {option}
            </MenuItem>
          ))}
        {/* Keep any committed custom value selectable */}
        {isCustomType && (
          <MenuItem key={currentMeta.type} value={currentMeta.type}>
            {currentMeta.type}
          </MenuItem>
        )}
        <MenuItem value={CUSTOM_TYPE_OPTION}>
          <Stack direction="row" spacing={0.75} alignItems="center" sx={{color: 'primary.main'}}>
            <Plus size={14} />
            <span>{defineCustomTypeLabel}</span>
          </Stack>
        </MenuItem>
      </Select>
    );
  };

  return (
    <Stack spacing={3}>
      <Grid container spacing={3}>
        <Grid size={{xs: 12, lg: scimCoreUserType ? 7 : 12}}>
          <Stack spacing={2}>
            {!scimCoreUserType && (
              <Alert
                severity="info"
                data-testid="scim-core-info"
                sx={{alignItems: 'center', '& .MuiAlert-action': {alignItems: 'center', mr: 0, pt: 0, pl: 3}}}
                action={
                  <Button
                    variant="outlined"
                    size="small"
                    disabled={disabled}
                    onClick={onRequestScimCoreChange}
                    sx={{whiteSpace: 'nowrap', px: 2, py: 0.5}}
                  >
                    {t('userTypes:edit.scimMapping.core.setButton', 'Set as SCIM Core Type')}
                  </Button>
                }
              >
                {currentCoreUserTypeName
                  ? t('userTypes:edit.scimMapping.core.currentCoreDescription', {
                      name: currentCoreUserTypeName,
                      defaultValue:
                        '{{name}} is currently the SCIM core user type. To use this user type as the SCIM core type, set it as the core type. Until then, SCIM requests and responses for this user type use the payload shown below.',
                    })
                  : t(
                      'userTypes:edit.scimMapping.core.inactiveDescription',
                      'Set this user type as the SCIM core user type to map its schema attributes to SCIM fields. Until then, SCIM requests and responses for this user type use the payload shown below.',
                    )}
              </Alert>
            )}

            {scimCoreUserType && (
              <>
                <Typography variant="body2" color="text.secondary" sx={{px: 1.5}}>
                  {scimCoreLocked
                    ? t(
                        'userTypes:edit.scimMapping.core.onlyTypeHint',
                        'This is the only user type, so it is the SCIM core type by default. Create another user type to choose a different one.',
                      )
                    : t(
                        'userTypes:edit.scimMapping.core.activeDescription',
                        'This is the SCIM core user type. Map its schema attributes to SCIM fields below.',
                      )}
                </Typography>

                {!hasUserNameMapped && (
                  <Alert severity="warning">
                    {t(
                      'userTypes:edit.scimMapping.userNameNotMappedWarning',
                      'No property is mapped to the SCIM userName attribute. SCIM requires it - most SCIM clients expect it present.',
                    )}
                  </Alert>
                )}

                <Stack direction="row" spacing={2} sx={{px: 1.5}}>
                  <Typography variant="overline" color="text.secondary" sx={{flex: LEFT_COLUMN}}>
                    {t('userTypes:edit.scimMapping.scimAttributeHeader', 'SCIM attribute')}
                  </Typography>
                  <Typography variant="overline" color="text.secondary" sx={{flex: 1}}>
                    {t('userTypes:edit.scimMapping.userTypeAttributeHeader', 'User type attribute')}
                  </Typography>
                </Stack>

                <Stack spacing={0.5}>
                  {rows.map(({target, propertyNames}) => {
                    const isFixedRow = target === 'userName';
                    const isMultiValued = MULTI_VALUED_TARGET_SET.has(target);

                    return (
                      <Stack
                        key={target}
                        direction="row"
                        spacing={2}
                        data-testid={`scim-mapping-row-${target}`}
                        sx={{
                          px: 1.5,
                          py: isMultiValued ? 0.75 : 1,
                          bgcolor: isMultiValued ? 'action.hover' : 'transparent',
                          border: isMultiValued ? 1 : 0,
                          borderColor: 'divider',
                          borderRadius: 1,
                        }}
                      >
                        {/* LEFT: SCIM attribute label */}
                        <Typography
                          variant="body2"
                          sx={{flex: LEFT_COLUMN, minWidth: 0, pt: 1, wordBreak: 'break-word'}}
                        >
                          {target}
                        </Typography>

                        {/* RIGHT: user type property entries stacked vertically */}
                        <Stack sx={{flex: 1, minWidth: 0}} spacing={1}>
                          {propertyNames.map((propName) => (
                            <Stack
                              key={propName || '__unmapped__'}
                              spacing={0.5}
                              data-testid={propName ? `scim-mapping-entry-${propName}` : 'scim-mapping-entry-userName'}
                            >
                              {/* Property select (no delete button on the fixed userName row) */}
                              {renderPropertySelect(
                                propName,
                                (next) => {
                                  if (propName) {
                                    handlePropertyChange(propName, next);
                                  } else {
                                    onMappingChange({...mapping, [next]: target});
                                  }
                                },
                                propName ? [propName, ...unmappedPropertyNames] : unmappedPropertyNames,
                                isFixedRow ? undefined : () => handleRemove(propName),
                                isFixedRow ? undefined : removeLabel,
                              )}
                              {/* Type + primary below the property select (multi-valued only) */}
                              {isMultiValued && (
                                <Stack direction="row" spacing={1} alignItems="center">
                                  {renderTypeWithCustom(propName, target)}
                                  {renderPrimaryCheckbox(propName, target, propertyNames.length === 1)}
                                </Stack>
                              )}
                            </Stack>
                          ))}

                          {isMultiValued && unmappedPropertyNames.length > 0 && (
                            <Button
                              size="small"
                              startIcon={<Plus size={14} />}
                              disabled={disabled}
                              onClick={(event) => setAddAnotherState({anchor: event.currentTarget, target})}
                              sx={{alignSelf: 'flex-start', py: 0}}
                            >
                              {t('userTypes:edit.scimMapping.addAnother', 'Add another')}
                            </Button>
                          )}
                        </Stack>
                      </Stack>
                    );
                  })}
                </Stack>

                <Menu
                  anchorEl={addAnotherState?.anchor ?? null}
                  open={addAnotherState !== null}
                  onClose={() => setAddAnotherState(null)}
                  slotProps={{paper: {sx: {maxHeight: 320}}}}
                >
                  {unmappedPropertyNames.map((name) => (
                    <MenuItem key={name} onClick={() => handleAddAnotherPick(name)}>
                      {name}
                    </MenuItem>
                  ))}
                </Menu>
              </>
            )}

            {scimCoreUserType && customAttributeNames.length > 0 && (
              <Stack
                spacing={1}
                data-testid="scim-custom-attributes"
                sx={{px: 1.5, pt: 2, borderTop: 1, borderColor: 'divider'}}
              >
                <Typography variant="subtitle2">
                  {t('userTypes:edit.scimMapping.customAttributes.title', 'Custom attributes')}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {t(
                    'userTypes:edit.scimMapping.customAttributes.description',
                    'Attributes not mapped above are sent under this user type custom extension schema. Click on an attribute to map it to a SCIM attribute.',
                  )}
                </Typography>
                <Stack direction="row" sx={{flexWrap: 'wrap', gap: 0.75}}>
                  {customAttributeNames.map((name) => {
                    const mappable = eligiblePropertyNames.includes(name);
                    return (
                      <Chip
                        key={name}
                        label={name}
                        size="small"
                        color="default"
                        variant="filled"
                        onClick={
                          mappable && !disabled
                            ? (event) => setMenuState({anchor: event.currentTarget, property: name})
                            : undefined
                        }
                        sx={{maxWidth: '100%'}}
                      />
                    );
                  })}
                </Stack>
                <Menu
                  anchorEl={menuState?.anchor ?? null}
                  open={menuState !== null}
                  onClose={() => setMenuState(null)}
                  slotProps={{paper: {sx: {maxHeight: 320}}}}
                >
                  {availableTargets.map((target) => (
                    <MenuItem key={target} onClick={() => handleTargetPick(target)}>
                      {target}
                    </MenuItem>
                  ))}
                </Menu>
              </Stack>
            )}
          </Stack>
        </Grid>
        <Grid size={{xs: 12, lg: scimCoreUserType ? 5 : 12}} sx={{minWidth: 0}}>
          <ScimPayloadPreview payload={payload} />
        </Grid>
      </Grid>
    </Stack>
  );
}
