/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {SettingsCard} from '@thunderid/components';
import {useGetUserType, useGetUserTypes} from '@thunderid/configure-user-types';
import {
  Autocomplete,
  Box,
  Button,
  Divider,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {Link2, Plus, Share2, Trash2, User} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useEffect, useMemo, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import type {AttributeConfiguration} from '../models/connection';
import {
  flattenUserTypeAttributes,
  fromAttributeConfiguration,
  toAttributeConfiguration,
} from '../utils/attributeConfiguration';

interface KeyedRow {
  key: number;
  externalAttribute: string;
  localAttribute: string;
}

interface KeyedGroup {
  key: number;
  userType: string;
  rows: KeyedRow[];
}

interface KeyedValue {
  key: number;
  value: string;
  userType: string;
}

const EMPTY_VALUE_MAPPING: KeyedValue[] = [];

interface KeyedLink {
  key: number;
  value: string;
}

interface AttributeMappingSectionProps {
  initialConfig?: AttributeConfiguration;
  onChange: (config: AttributeConfiguration | undefined, valid: boolean) => void;
}

/** Options for a user-type Select, ensuring a stored value is selectable even if absent from the list. */
function withStoredOption(names: string[], value: string): string[] {
  return value && !names.includes(value) ? [value, ...names] : names;
}

/** A mapping row is incomplete when exactly one of its two sides is filled (it is dropped on save). */
function rowIncomplete(row: {externalAttribute: string; localAttribute: string}): boolean {
  return (row.externalAttribute.trim() !== '') !== (row.localAttribute.trim() !== '');
}

/** A value-mapping entry is incomplete when exactly one of its two sides is filled. */
function valueEntryIsIncomplete(entry: {value: string; userType: string}): boolean {
  return (entry.value.trim() !== '') !== (entry.userType.trim() !== '');
}

/**
 * Editable attribute-mapping profile for a single user type. Rendered as its own component so the
 * per-group local-attribute options can be fetched from the selected user type's schema (a hook
 * cannot be called per item in a list).
 */
function MappingGroupEditor({
  group,
  userTypeNames,
  otherUsedUserTypes,
  userTypeIdByName,
  canRemove,
  showUserTypeError,
  onUserTypeChange,
  onAddRow,
  onRemoveRow,
  onUpdateRow,
  onRemoveGroup,
}: {
  group: KeyedGroup;
  userTypeNames: string[];
  otherUsedUserTypes: string[];
  userTypeIdByName: Map<string, string>;
  canRemove: boolean;
  showUserTypeError: boolean;
  onUserTypeChange: (userType: string) => void;
  onAddRow: () => void;
  onRemoveRow: (rowKey: number) => void;
  onUpdateRow: (rowKey: number, patch: Partial<KeyedRow>) => void;
  onRemoveGroup: () => void;
}): JSX.Element {
  const {t} = useTranslation('connections');
  const userTypeDetail = useGetUserType(userTypeIdByName.get(group.userType));
  const localAttributeOptions: string[] = useMemo(
    () => flattenUserTypeAttributes(userTypeDetail.data?.schema),
    [userTypeDetail.data],
  );
  const options: string[] = useMemo(() => {
    const usedElsewhere = new Set(otherUsedUserTypes);
    return withStoredOption(
      userTypeNames.filter((name) => !usedElsewhere.has(name)),
      group.userType,
    );
  }, [userTypeNames, otherUsedUserTypes, group.userType]);
  const singleUserType: boolean = userTypeNames.length === 1;
  const lastRow = group.rows[group.rows.length - 1];
  const lastRowIncomplete: boolean =
    lastRow !== undefined && (lastRow.externalAttribute.trim() === '' || lastRow.localAttribute.trim() === '');

  return (
    <Box
      sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, p: 2}}
      data-testid={`attribute-mapping-group-${group.key}`}
    >
      {(!singleUserType || canRemove) && (
        <Stack direction="row" spacing={2} alignItems="flex-end" sx={{mb: 2}}>
          {!singleUserType && (
            <FormControl sx={{minWidth: 220}} error={showUserTypeError}>
              <FormLabel sx={{mb: 0.75}} htmlFor={`attribute-mapping-group-user-type-${group.key}`}>
                {t('attributeMapping.mappings.userType')}
              </FormLabel>
              <Select
                id={`attribute-mapping-group-user-type-${group.key}`}
                displayEmpty
                value={group.userType}
                onChange={(e) => onUserTypeChange(e.target.value)}
                renderValue={(value) => (value ? value : t('attributeMapping.userType.placeholder'))}
                data-testid={`attribute-mapping-group-user-type-select-${group.key}`}
              >
                {options.map((name) => (
                  <MenuItem key={name} value={name}>
                    {name}
                  </MenuItem>
                ))}
              </Select>
              {showUserTypeError && <FormHelperText>{t('attributeMapping.mappings.userTypeRequired')}</FormHelperText>}
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
              data-testid={`attribute-mapping-group-remove-${group.key}`}
            >
              {t('attributeMapping.mappings.remove')}
            </Button>
          )}
        </Stack>
      )}

      <Stack direction="column" spacing={1.5}>
        {group.rows.length > 0 && (
          <Stack direction="row" spacing={1.5}>
            <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
              {t('attributeMapping.externalAttribute.label')}
            </Typography>
            <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
              {t('attributeMapping.localAttribute.label')}
            </Typography>
            <Box sx={{width: 40}} />
          </Stack>
        )}
        {group.rows.map((row) => {
          const incomplete = rowIncomplete(row);
          const hasContent = row.externalAttribute.trim() !== '' || row.localAttribute.trim() !== '';
          const canDeleteRow = hasContent || group.rows.length > 1;
          return (
            <Stack key={row.key} direction="row" spacing={1.5} alignItems="center">
              <TextField
                fullWidth
                error={incomplete && row.externalAttribute.trim() === ''}
                placeholder={t('attributeMapping.externalAttribute.placeholder')}
                value={row.externalAttribute}
                onChange={(e) => onUpdateRow(row.key, {externalAttribute: e.target.value})}
                inputProps={{'aria-label': t('attributeMapping.externalAttribute.label')}}
              />
              <Autocomplete
                fullWidth
                freeSolo
                options={localAttributeOptions}
                inputValue={row.localAttribute}
                onInputChange={(_event, value) => onUpdateRow(row.key, {localAttribute: value})}
                renderInput={(params) => (
                  <TextField
                    {...params}
                    error={incomplete && row.localAttribute.trim() === ''}
                    placeholder={t('attributeMapping.localAttribute.placeholder')}
                    inputProps={{...params.inputProps, 'aria-label': t('attributeMapping.localAttribute.label')}}
                  />
                )}
              />
              {canDeleteRow ? (
                <IconButton
                  onClick={() => onRemoveRow(row.key)}
                  aria-label="remove attribute mapping"
                  data-testid={`attribute-mapping-remove-${group.key}-${row.key}`}
                >
                  <Trash2 size={16} />
                </IconButton>
              ) : (
                <Box sx={{width: 40}} />
              )}
            </Stack>
          );
        })}
        <Box>
          <Button
            variant="outlined"
            size="small"
            startIcon={<Plus size={16} />}
            onClick={onAddRow}
            disabled={lastRowIncomplete}
            data-testid={`attribute-mapping-add-${group.key}`}
          >
            {t('attributeMapping.add')}
          </Button>
        </Box>
      </Stack>
    </Box>
  );
}

export default function AttributeMappingSection({
  initialConfig = undefined,
  onChange,
}: AttributeMappingSectionProps): JSX.Element {
  const {t} = useTranslation('connections');

  const [initialKeyed] = useState(() => {
    const state = fromAttributeConfiguration(initialConfig);
    let seq = 0;
    const nk = (): number => {
      seq += 1;
      return seq;
    };
    return {
      defaultUserType: state.defaultUserType,
      resolveDynamic: state.resolveDynamic,
      externalAttribute: state.externalAttribute,
      valueMapping: state.valueMapping.map((entry) => ({key: nk(), ...entry})),
      groups:
        state.groups.length > 0
          ? state.groups.map((group) => ({
              key: nk(),
              userType: group.userType,
              rows: group.rows.map((row) => ({key: nk(), ...row})),
            }))
          : [{key: nk(), userType: '', rows: [{key: nk(), externalAttribute: '', localAttribute: ''}]}],
      groupsWasEmpty: state.groups.length === 0,
      linking: state.linking.length > 0 ? state.linking.map((value) => ({key: nk(), value})) : [{key: nk(), value: ''}],
      linkingWasEmpty: state.linking.length === 0,
      seq,
    };
  });
  const keyCounter = useRef(initialKeyed.seq);
  const nextKey = (): number => {
    keyCounter.current += 1;
    return keyCounter.current;
  };

  const [defaultUserType, setDefaultUserType] = useState<string>(initialKeyed.defaultUserType);
  const [resolveDynamic, setResolveDynamic] = useState<boolean>(initialKeyed.resolveDynamic);
  const [externalAttribute, setExternalAttribute] = useState<string>(initialKeyed.externalAttribute);
  const [valueMappingEnabled, setValueMappingEnabled] = useState<boolean>(initialKeyed.valueMapping.length > 0);
  const [valueMapping, setValueMapping] = useState<KeyedValue[]>(initialKeyed.valueMapping);
  const [groups, setGroups] = useState<KeyedGroup[]>(initialKeyed.groups);
  const [linking, setLinking] = useState<KeyedLink[]>(initialKeyed.linking);

  const userTypesQuery = useGetUserTypes();
  const userTypeList = useMemo(() => userTypesQuery.data?.types ?? [], [userTypesQuery.data]);
  const userTypeNames: string[] = useMemo(() => userTypeList.map((type) => type.name), [userTypeList]);
  const userTypeIdByName: Map<string, string> = useMemo(
    () => new Map(userTypeList.map((type) => [type.name, type.id])),
    [userTypeList],
  );
  const canResolveDynamic: boolean = userTypeList.length > 1;
  const showResolutionSection: boolean = userTypeList.length !== 1;
  const effectiveResolveDynamic: boolean = canResolveDynamic && resolveDynamic;

  // A fresh connection with no attribute config yet: once the sole user type is known, seed the
  // default and starter group with it (their selectors are hidden when there's only one) so mappings
  // the admin enters resolve to it instead of being dropped for a missing user type. Adjusted during
  // render (React's documented pattern for reacting to a changed value) rather than via an effect, to
  // avoid the extra render pass a setState-in-effect would cost.
  const wasUnconfigured: boolean =
    initialKeyed.defaultUserType === '' &&
    initialKeyed.groupsWasEmpty &&
    initialKeyed.linkingWasEmpty &&
    !initialKeyed.resolveDynamic;
  // Sentinel `null` (rather than the initial userTypeList) so the check below still evaluates on the
  // very first render — e.g. when the list is already available synchronously from a warm query cache.
  const [seenUserTypeList, setSeenUserTypeList] = useState<typeof userTypeList | null>(null);
  const [autoFilled, setAutoFilled] = useState(false);
  if (!autoFilled && wasUnconfigured && userTypeList !== seenUserTypeList) {
    setSeenUserTypeList(userTypeList);
    if (userTypeList.length === 1) {
      setAutoFilled(true);
      const onlyUserType = userTypeList[0].name;
      setDefaultUserType(onlyUserType);
      setGroups((prev) => prev.map((group, index) => (index === 0 ? {...group, userType: onlyUserType} : group)));
    }
  }

  const groupHasContent = (group: KeyedGroup): boolean =>
    group.rows.some((row) => row.externalAttribute.trim() !== '' || row.localAttribute.trim() !== '');
  const anyGroupHasContent: boolean = groups.some(groupHasContent);
  const groupUserTypeMissing: boolean = groups.some((group) => group.userType.trim() === '' && groupHasContent(group));
  const groupRowIncomplete: boolean = groups.some((group) => group.rows.some(rowIncomplete));
  const defaultMissing: boolean = defaultUserType.trim() === '' && (anyGroupHasContent || effectiveResolveDynamic);
  const externalMissing: boolean = effectiveResolveDynamic && externalAttribute.trim() === '';
  // Entries typed while the "Value Mapping" toggle is off don't count — they're preserved so toggling
  // back on restores them, but they aren't active until the admin explicitly re-enables the toggle.
  // Value mappings are optional even when dynamic resolution is on: every identity resolves to the
  // default until mappings are added, so only half-filled entries (not their absence) are invalid.
  const effectiveValueMapping: KeyedValue[] = valueMappingEnabled ? valueMapping : EMPTY_VALUE_MAPPING;
  const valueEntryIncomplete: boolean = effectiveResolveDynamic && effectiveValueMapping.some(valueEntryIsIncomplete);
  const valid: boolean =
    !groupUserTypeMissing && !groupRowIncomplete && !defaultMissing && !externalMissing && !valueEntryIncomplete;

  useEffect(() => {
    const hasContent: boolean =
      anyGroupHasContent || effectiveResolveDynamic || linking.some((entry) => entry.value.trim() !== '');
    // On a fresh connection with a single user type the default is auto-derived and its field hidden,
    // so don't persist a default-only config the admin never configured (which would dirty the form
    // just by opening it). Only suppress for truly unconfigured connections — an existing default-only
    // config must still round-trip unchanged.
    const emitDefault: string = wasUnconfigured && userTypeList.length === 1 && !hasContent ? '' : defaultUserType;
    const config = toAttributeConfiguration({
      defaultUserType: emitDefault,
      resolveDynamic: effectiveResolveDynamic,
      externalAttribute,
      valueMapping: effectiveValueMapping.map((entry) => ({value: entry.value, userType: entry.userType})),
      groups: groups.map((group) => ({
        userType: group.userType,
        rows: group.rows.map((row) => ({externalAttribute: row.externalAttribute, localAttribute: row.localAttribute})),
      })),
      linking: linking.map((entry) => entry.value),
    });
    onChange(config, valid);
  }, [
    defaultUserType,
    effectiveResolveDynamic,
    externalAttribute,
    effectiveValueMapping,
    groups,
    linking,
    valid,
    anyGroupHasContent,
    userTypeList,
    wasUnconfigured,
    onChange,
  ]);

  const defaultOptions: string[] = useMemo(
    () => withStoredOption(userTypeNames, defaultUserType),
    [userTypeNames, defaultUserType],
  );

  const hasUnusedUserType = (usedTypes: string[]): boolean => {
    const used = new Set(usedTypes.filter((name) => name.trim() !== ''));
    return userTypeNames.some((name) => !used.has(name));
  };
  const lastValueIsEmpty: boolean =
    valueMapping.length > 0 && valueMapping[valueMapping.length - 1].value.trim() === '';
  const showAddValue: boolean = userTypeList.length > 1;
  const showAddUserType: boolean = hasUnusedUserType(groups.map((group) => group.userType));
  const lastLinkIsEmpty: boolean = linking.length > 0 && linking[linking.length - 1].value.trim() === '';

  // Value-mapping handlers.
  const addValue = (): void =>
    setValueMapping((prev) => [
      ...prev,
      {key: nextKey(), value: '', userType: userTypeList.length === 1 ? userTypeList[0].name : ''},
    ]);
  const removeValue = (key: number): void => setValueMapping((prev) => prev.filter((entry) => entry.key !== key));
  const updateValue = (key: number, patch: Partial<KeyedValue>): void =>
    setValueMapping((prev) => prev.map((entry) => (entry.key === key ? {...entry, ...patch} : entry)));
  const toggleValueMapping = (enabled: boolean): void => {
    setValueMappingEnabled(enabled);
    if (enabled && valueMapping.length === 0) {
      addValue();
    }
  };

  // Group / row handlers.
  const addGroup = (): void =>
    setGroups((prev) => [
      ...prev,
      {key: nextKey(), userType: '', rows: [{key: nextKey(), externalAttribute: '', localAttribute: ''}]},
    ]);
  const removeGroup = (key: number): void => setGroups((prev) => prev.filter((group) => group.key !== key));
  const updateGroupType = (key: number, userType: string): void =>
    setGroups((prev) => prev.map((group) => (group.key === key ? {...group, userType} : group)));
  const addRow = (groupKey: number): void =>
    setGroups((prev) =>
      prev.map((group) =>
        group.key === groupKey
          ? {...group, rows: [...group.rows, {key: nextKey(), externalAttribute: '', localAttribute: ''}]}
          : group,
      ),
    );
  const removeRow = (groupKey: number, rowKey: number): void =>
    setGroups((prev) =>
      prev.map((group) =>
        group.key === groupKey ? {...group, rows: group.rows.filter((row) => row.key !== rowKey)} : group,
      ),
    );
  const updateRow = (groupKey: number, rowKey: number, patch: Partial<KeyedRow>): void =>
    setGroups((prev) =>
      prev.map((group) =>
        group.key === groupKey
          ? {...group, rows: group.rows.map((row) => (row.key === rowKey ? {...row, ...patch} : row))}
          : group,
      ),
    );

  // Account-linking handlers.
  const addLink = (): void => setLinking((prev) => [...prev, {key: nextKey(), value: ''}]);
  const removeLink = (key: number): void => setLinking((prev) => prev.filter((entry) => entry.key !== key));
  const updateLink = (key: number, value: string): void =>
    setLinking((prev) => prev.map((entry) => (entry.key === key ? {...entry, value} : entry)));

  const iconBox = (icon: JSX.Element): JSX.Element => (
    <Box
      sx={{
        width: 30,
        height: 30,
        borderRadius: 1.5,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        bgcolor: 'action.hover',
        color: 'primary.main',
      }}
    >
      {icon}
    </Box>
  );

  return (
    <Stack direction="column" spacing={3} data-testid="attribute-mapping-section">
      {/* Section 1 — user type resolution (hidden when there's only one user type to resolve to) */}
      {showResolutionSection && (
        <SettingsCard
          title={t('attributeMapping.resolution.title')}
          description={t('attributeMapping.resolution.description')}
          titleIcon={iconBox(<User size={16} />)}
        >
          <Stack direction="column" spacing={3.5}>
            {canResolveDynamic && (
              <Stack direction="row" spacing={2} alignItems="center" justifyContent="space-between">
                <Typography variant="subtitle2">{t('attributeMapping.resolution.dynamic.label')}</Typography>
                <Switch
                  checked={resolveDynamic}
                  onChange={(e) => setResolveDynamic(e.target.checked)}
                  inputProps={{'aria-label': t('attributeMapping.resolution.dynamic.label')}}
                />
              </Stack>
            )}

            {effectiveResolveDynamic && (
              <>
                <FormControl sx={{maxWidth: 360}}>
                  <FormLabel sx={{mb: 0.75}} htmlFor="attribute-mapping-resolution-external">
                    {t('attributeMapping.externalAttribute.label')}
                  </FormLabel>
                  <TextField
                    id="attribute-mapping-resolution-external"
                    placeholder={t('attributeMapping.resolution.externalAttribute.placeholder')}
                    value={externalAttribute}
                    onChange={(e) => setExternalAttribute(e.target.value)}
                  />
                  <FormHelperText>{t('attributeMapping.resolution.externalAttribute.helper')}</FormHelperText>
                </FormControl>

                <Box>
                  <Stack direction="row" spacing={2} alignItems="center" justifyContent="space-between">
                    <Typography variant="body2" color="text.secondary" fontWeight={600}>
                      {t('attributeMapping.resolution.valueMapping.title')}
                    </Typography>
                    <Switch
                      checked={valueMappingEnabled}
                      onChange={(e) => toggleValueMapping(e.target.checked)}
                      slotProps={{
                        input: {role: 'switch', 'aria-label': t('attributeMapping.resolution.valueMapping.enable')},
                      }}
                    />
                  </Stack>
                  <Typography variant="body2" color="text.secondary" sx={{mt: 0.25, mb: valueMappingEnabled ? 2 : 0}}>
                    {t('attributeMapping.resolution.valueMapping.hint')}
                  </Typography>
                  {valueMappingEnabled && (
                    <Stack direction="column" spacing={1.5}>
                      {valueMapping.length > 0 && (
                        <Stack direction="row" spacing={1.5}>
                          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
                            {t('attributeMapping.resolution.valueMapping.externalValue')}
                          </Typography>
                          <Typography variant="caption" color="text.secondary" fontWeight={600} sx={{flex: 1}}>
                            {t('attributeMapping.resolution.valueMapping.localUserType')}
                          </Typography>
                          <Box sx={{width: 40}} />
                        </Stack>
                      )}
                      {valueMapping.map((entry) => {
                        const incomplete = valueEntryIsIncomplete(entry);
                        const hasContent = entry.value.trim() !== '' || entry.userType.trim() !== '';
                        const canDeleteEntry = hasContent || valueMapping.length > 1;
                        return (
                          <Stack key={entry.key} direction="row" spacing={1.5} alignItems="center">
                            <TextField
                              fullWidth
                              error={incomplete && entry.value.trim() === ''}
                              placeholder={t('attributeMapping.resolution.valueMapping.valuePlaceholder')}
                              value={entry.value}
                              onChange={(e) => updateValue(entry.key, {value: e.target.value})}
                              inputProps={{'aria-label': t('attributeMapping.resolution.valueMapping.externalValue')}}
                            />
                            <Select
                              fullWidth
                              displayEmpty
                              error={incomplete && entry.userType.trim() === ''}
                              value={entry.userType}
                              onChange={(e) => updateValue(entry.key, {userType: e.target.value})}
                              renderValue={(value) => (value ? value : t('attributeMapping.userType.placeholder'))}
                              inputProps={{'aria-label': t('attributeMapping.resolution.valueMapping.localUserType')}}
                            >
                              {withStoredOption(userTypeNames, entry.userType).map((name) => (
                                <MenuItem key={name} value={name}>
                                  {name}
                                </MenuItem>
                              ))}
                            </Select>
                            {canDeleteEntry ? (
                              <IconButton
                                onClick={() => removeValue(entry.key)}
                                aria-label="remove value mapping"
                                data-testid={`attribute-mapping-value-remove-${entry.key}`}
                              >
                                <Trash2 size={16} />
                              </IconButton>
                            ) : (
                              <Box sx={{width: 40}} />
                            )}
                          </Stack>
                        );
                      })}
                      {showAddValue && (
                        <Box>
                          <Button
                            variant="outlined"
                            size="small"
                            startIcon={<Plus size={16} />}
                            onClick={addValue}
                            disabled={lastValueIsEmpty}
                            data-testid="attribute-mapping-value-add"
                          >
                            {t('attributeMapping.resolution.addValue')}
                          </Button>
                        </Box>
                      )}
                    </Stack>
                  )}
                </Box>
              </>
            )}

            <FormControl sx={{maxWidth: 360}} error={defaultMissing}>
              <FormLabel sx={{mb: 0.75}} htmlFor="attribute-mapping-default-user-type">
                {t('attributeMapping.userType.label')}
              </FormLabel>
              <Select
                id="attribute-mapping-default-user-type"
                displayEmpty
                value={defaultUserType}
                onChange={(e) => setDefaultUserType(e.target.value)}
                renderValue={(value) => (value ? value : t('attributeMapping.userType.placeholder'))}
                data-testid="attribute-mapping-default-user-type-select"
              >
                {defaultOptions.map((name) => (
                  <MenuItem key={name} value={name}>
                    {name}
                  </MenuItem>
                ))}
              </Select>
              {(defaultMissing || effectiveResolveDynamic) && (
                <FormHelperText>
                  {defaultMissing
                    ? t('attributeMapping.userTypeRequired')
                    : t('attributeMapping.resolution.default.helperFallback')}
                </FormHelperText>
              )}
            </FormControl>
          </Stack>
        </SettingsCard>
      )}

      {/* Section 2 — attribute mappings by user type */}
      <SettingsCard
        title={t('attributeMapping.mappings.title')}
        description={t('attributeMapping.mappings.description')}
        titleIcon={iconBox(<Share2 size={16} />)}
      >
        <Stack direction="column" spacing={2}>
          {groups.map((group) => (
            <MappingGroupEditor
              key={group.key}
              group={group}
              userTypeNames={userTypeNames}
              otherUsedUserTypes={groups
                .filter((other) => other.key !== group.key)
                .map((other) => other.userType)
                .filter((userType) => userType.trim() !== '')}
              userTypeIdByName={userTypeIdByName}
              canRemove={groups.length > 1}
              showUserTypeError={group.userType.trim() === '' && groupHasContent(group)}
              onUserTypeChange={(userType) => updateGroupType(group.key, userType)}
              onAddRow={() => addRow(group.key)}
              onRemoveRow={(rowKey) => removeRow(group.key, rowKey)}
              onUpdateRow={(rowKey, patch) => updateRow(group.key, rowKey, patch)}
              onRemoveGroup={() => removeGroup(group.key)}
            />
          ))}
          {showAddUserType && (
            <Box>
              <Button
                variant="outlined"
                size="small"
                startIcon={<Plus size={16} />}
                onClick={addGroup}
                data-testid="attribute-mapping-add-user-type"
              >
                {t('attributeMapping.mappings.addUserType')}
              </Button>
            </Box>
          )}
        </Stack>
      </SettingsCard>

      {/* Section 3 — account linking */}
      <SettingsCard
        title={t('attributeMapping.linking.title')}
        description={t('attributeMapping.linking.description')}
        titleIcon={iconBox(<Link2 size={16} />)}
      >
        <Stack direction="column" spacing={1.5}>
          <Typography variant="body2" color="text.secondary" fontWeight={600}>
            {linking.length > 1 ? t('attributeMapping.linking.labelCombo') : t('attributeMapping.linking.label')}
          </Typography>
          {linking.map((entry, index) => {
            const canDeleteLink = entry.value.trim() !== '' || linking.length > 1;
            return (
              <Stack key={entry.key} direction="column" spacing={1.5}>
                {index > 0 && (
                  <Stack direction="row" spacing={1} alignItems="center">
                    <Typography variant="caption" color="primary.main" fontWeight={700}>
                      {t('attributeMapping.linking.and')}
                    </Typography>
                    <Divider sx={{flex: 1}} />
                  </Stack>
                )}
                <Stack direction="row" spacing={1.5} alignItems="center">
                  <TextField
                    fullWidth
                    placeholder={t('attributeMapping.linking.placeholder')}
                    value={entry.value}
                    onChange={(e) => updateLink(entry.key, e.target.value)}
                    inputProps={{'aria-label': t('attributeMapping.linking.label')}}
                  />
                  {canDeleteLink ? (
                    <IconButton
                      onClick={() => removeLink(entry.key)}
                      aria-label="remove account linking attribute"
                      data-testid={`attribute-mapping-link-remove-${entry.key}`}
                    >
                      <Trash2 size={16} />
                    </IconButton>
                  ) : (
                    <Box sx={{width: 40}} />
                  )}
                </Stack>
              </Stack>
            );
          })}
          <Box>
            <Button
              variant="outlined"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={addLink}
              disabled={lastLinkIsEmpty}
              data-testid="attribute-mapping-link-add"
            >
              {t('attributeMapping.linking.addAttribute')}
            </Button>
          </Box>
        </Stack>
      </SettingsCard>
    </Stack>
  );
}
