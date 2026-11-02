// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {
  PermissionCatalog,
  removePermissions,
  useGetResourceServers,
  type ResourcePermissions,
} from '@thunderid/configure-resource-servers';
import {
  Autocomplete,
  Box,
  Button,
  Chip,
  FormControl,
  FormLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  Switch,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import DelimiterField from './DelimiterField';
import useGetGroupsForMapping from '../api/useGetGroupsForMapping';
import useGetRolesForMapping from '../api/useGetRolesForMapping';
import type {
  AuthorizationDirectMapping,
  AuthorizationOperator,
  AuthorizationRuleMapping,
  AuthorizationTargetType,
  AuthorizationValueType,
} from '../models/connection';
import {
  defaultOperatorFor,
  fromAuthorizationMappings,
  type KeyedAuthorizationMapping,
  type KeyedAuthorizationValue,
  mappingIsAttempted,
  mappingIsComplete,
  newAuthorizationMapping,
  newValueRow,
  operatorsFor,
  ruleRowAttempted,
  ruleRowIsComplete,
  toAuthorizationMappings,
} from '../utils/authorizationMapping';

export interface AuthorizationMappingSectionProps {
  initialRuleConfig?: AuthorizationRuleMapping[];
  initialDirectConfig?: AuthorizationDirectMapping[];
  onRuleChange: (mappings: AuthorizationRuleMapping[] | undefined, valid: boolean) => void;
  onDirectChange: (mappings: AuthorizationDirectMapping[] | undefined, valid: boolean) => void;
}

const VALUE_TYPES: AuthorizationValueType[] = ['string', 'number', 'boolean', 'array'];
const TARGET_TYPES: AuthorizationTargetType[] = ['role', 'group', 'permission'];

// No search endpoint exists yet for roles, groups, or resource servers, so each picker fetches one
// page and filters client-side. 100 is the server's enforced maximum page size, not a UI choice.
const PICKER_PAGE_SIZE = 100;

interface PickerOption {
  id: string;
  name: string;
}

function targetTypeLabel(t: (key: string) => string, targetType: AuthorizationTargetType): string {
  return t(`authorizationRules.target.type.${targetType}`);
}

function RolesField({value, onChange}: {value: string[]; onChange: (ids: string[]) => void}): JSX.Element {
  const {t} = useTranslation('connections');
  const {data} = useGetRolesForMapping({limit: PICKER_PAGE_SIZE});
  const options: PickerOption[] = useMemo(() => (data?.roles ?? []).map((r) => ({id: r.id, name: r.name})), [data]);
  // A stored id absent from this page (beyond PICKER_PAGE_SIZE, or the page still loading) still needs
  // a selected entry, or the next onChange - which reports only what Autocomplete rendered - would
  // silently drop it.
  const selected = useMemo(
    () => value.map((id) => options.find((o) => o.id === id) ?? {id, name: id}),
    [value, options],
  );

  return (
    <Autocomplete
      multiple
      fullWidth
      options={options}
      value={selected}
      getOptionLabel={(o) => o.name}
      isOptionEqualToValue={(o, v) => o.id === v.id}
      onChange={(_event, newValue) => onChange(newValue.map((o) => o.id))}
      renderInput={(params) => (
        <TextField
          {...params}
          placeholder={t('authorizationRules.target.role.placeholder')}
          inputProps={{...params.inputProps, 'aria-label': t('authorizationRules.target.role.label')}}
        />
      )}
    />
  );
}

function GroupsField({value, onChange}: {value: string[]; onChange: (ids: string[]) => void}): JSX.Element {
  const {t} = useTranslation('connections');
  const {data} = useGetGroupsForMapping({limit: PICKER_PAGE_SIZE});
  const options: PickerOption[] = useMemo(() => (data?.groups ?? []).map((g) => ({id: g.id, name: g.name})), [data]);
  // A stored id absent from this page (beyond PICKER_PAGE_SIZE, or the page still loading) still needs
  // a selected entry, or the next onChange - which reports only what Autocomplete rendered - would
  // silently drop it.
  const selected = useMemo(
    () => value.map((id) => options.find((o) => o.id === id) ?? {id, name: id}),
    [value, options],
  );

  return (
    <Autocomplete
      multiple
      fullWidth
      options={options}
      value={selected}
      getOptionLabel={(o) => o.name}
      isOptionEqualToValue={(o, v) => o.id === v.id}
      onChange={(_event, newValue) => onChange(newValue.map((o) => o.id))}
      renderInput={(params) => (
        <TextField
          {...params}
          placeholder={t('authorizationRules.target.group.placeholder')}
          inputProps={{...params.inputProps, 'aria-label': t('authorizationRules.target.group.label')}}
        />
      )}
    />
  );
}

/** Permissions are picked from the full hierarchy (grouped by resource server, so a single rule can
 * span more than one server) and shown as removable chips beneath the tree. */
function PermissionsField({
  value,
  onChange,
}: {
  value: ResourcePermissions[];
  onChange: (value: ResourcePermissions[]) => void;
}): JSX.Element {
  const chips = value.flatMap((rp) =>
    rp.permissions.map((permission) => ({resourceServerId: rp.resourceServerId, permission})),
  );

  return (
    <Stack direction="column" spacing={1}>
      <PermissionCatalog selected={value} onChange={onChange} />
      {chips.length > 0 && (
        <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
          {chips.map((chip) => (
            <Chip
              key={`${chip.resourceServerId}:${chip.permission}`}
              size="small"
              label={chip.permission}
              onDelete={() => onChange(removePermissions(value, chip.resourceServerId, [chip.permission]))}
            />
          ))}
        </Stack>
      )}
    </Stack>
  );
}

/**
 * The generic rule-input piece for one value row: an operator picker (filtered to what `valueType`
 * allows) followed by the value being compared against. Boolean claims get a True/False picker rather
 * than freehand text, since there are only two meaningful values.
 */
function RuleValueFields({
  valueType,
  delimiter,
  operator,
  value,
  error = undefined,
  onChange,
}: {
  valueType: AuthorizationValueType;
  delimiter: string;
  operator: AuthorizationOperator;
  value: string;
  error?: boolean;
  onChange: (patch: {operator?: AuthorizationOperator; value?: string}) => void;
}): JSX.Element {
  const {t} = useTranslation('connections');
  const operators = operatorsFor(valueType, delimiter);

  return (
    <Stack direction="row" spacing={1.5} alignItems="center" sx={{flex: 1}}>
      <Select
        value={operator}
        onChange={(e) => onChange({operator: e.target.value as AuthorizationOperator})}
        inputProps={{'aria-label': t('authorizationRules.operator.label')}}
        sx={{minWidth: 190}}
      >
        {operators.map((op) => (
          <MenuItem key={op} value={op}>
            {t(`authorizationRules.operator.${op}`)}
          </MenuItem>
        ))}
      </Select>

      {valueType === 'boolean' ? (
        <Select
          fullWidth
          displayEmpty
          error={error}
          value={value}
          onChange={(e) => onChange({value: e.target.value})}
          renderValue={(v) =>
            v ? t(`authorizationRules.value.boolean.${v}`) : t('authorizationRules.value.placeholder')
          }
          inputProps={{'aria-label': t('authorizationRules.value.label')}}
        >
          <MenuItem value="true">{t('authorizationRules.value.boolean.true')}</MenuItem>
          <MenuItem value="false">{t('authorizationRules.value.boolean.false')}</MenuItem>
        </Select>
      ) : (
        <TextField
          fullWidth
          error={error}
          placeholder={t('authorizationRules.value.placeholder')}
          value={value}
          onChange={(e) => onChange({value: e.target.value})}
          inputProps={{'aria-label': t('authorizationRules.value.label')}}
        />
      )}
    </Stack>
  );
}

function ValueRow({
  claim,
  entry,
  valueType,
  delimiter,
  canRemove,
  onUpdateRule,
  onUpdateGrants,
  onRemoveValue,
}: {
  claim: string;
  entry: KeyedAuthorizationValue;
  valueType: AuthorizationValueType;
  delimiter: string;
  canRemove: boolean;
  onUpdateRule: (patch: {operator?: AuthorizationOperator; value?: string}) => void;
  onUpdateGrants: (patch: Partial<Pick<KeyedAuthorizationValue, 'roleIds' | 'groupIds' | 'permissions'>>) => void;
  onRemoveValue: () => void;
}): JSX.Element {
  const {t} = useTranslation('connections');
  // Flags both a rule missing its own grants, and a rule with a value and grants but whose owning
  // claim is blank — either way, toAuthorizationMappings would drop it silently on save.
  const incomplete = ruleRowAttempted(entry) && !ruleRowIsComplete(claim, entry);

  return (
    <Box sx={{border: '1px dashed', borderColor: 'divider', borderRadius: 1.5, p: 1.5}}>
      <Typography variant="caption" color="text.secondary" fontWeight={600}>
        {t('authorizationRules.value.label')}
      </Typography>
      <Stack direction="row" spacing={1.5} alignItems="center" sx={{mb: 1.5, mt: 0.5}}>
        <RuleValueFields
          valueType={valueType}
          delimiter={delimiter}
          operator={entry.operator}
          value={entry.value}
          error={incomplete}
          onChange={onUpdateRule}
        />
        {canRemove && (
          <IconButton onClick={onRemoveValue} aria-label={t('authorizationRules.value.remove')}>
            <Trash2 size={16} />
          </IconButton>
        )}
      </Stack>
      <Typography variant="caption" color="text.secondary" fontWeight={600}>
        {t('authorizationRules.target.sectionLabel')}
      </Typography>
      <Stack direction="column" spacing={1.5} sx={{mt: 0.5}}>
        <FormControl>
          <FormLabel sx={{mb: 0.75}}>{t('authorizationRules.target.type.role')}</FormLabel>
          <RolesField value={entry.roleIds} onChange={(roleIds) => onUpdateGrants({roleIds})} />
        </FormControl>
        <FormControl>
          <FormLabel sx={{mb: 0.75}}>{t('authorizationRules.target.type.group')}</FormLabel>
          <GroupsField value={entry.groupIds} onChange={(groupIds) => onUpdateGrants({groupIds})} />
        </FormControl>
        <FormControl>
          <FormLabel sx={{mb: 0.75}}>{t('authorizationRules.target.type.permission')}</FormLabel>
          <PermissionsField value={entry.permissions} onChange={(permissions) => onUpdateGrants({permissions})} />
        </FormControl>
      </Stack>
    </Box>
  );
}

function MappingRow({
  mapping,
  servers,
  onChange,
  onRemove,
}: {
  mapping: KeyedAuthorizationMapping;
  servers: PickerOption[];
  onChange: (patch: Partial<KeyedAuthorizationMapping>) => void;
  onRemove: () => void;
}): JSX.Element {
  const {t} = useTranslation('connections');

  /** Resets any rule left on an operator that `valueType`/`delimiter` no longer allows to that
   * combination's default operator, leaving every other rule untouched. */
  const reconcileOperators = (
    values: KeyedAuthorizationValue[],
    valueType: AuthorizationValueType,
    delimiter: string,
  ): KeyedAuthorizationValue[] => {
    const validOperators = operatorsFor(valueType, delimiter);
    const fallbackOperator = defaultOperatorFor(valueType, delimiter);
    return values.map((v) => (validOperators.includes(v.operator) ? v : {...v, operator: fallbackOperator}));
  };

  /** Changing valueType or delimiter can change which operators are valid (e.g. leaving "number",
   * switching to "array", or setting/clearing a delimiter on a "string" mapping): any rule left on an
   * operator that's no longer valid resets to the new state's default operator. Delimiter is also
   * forced back to "" whenever valueType isn't "string", since the backend rejects a non-empty
   * delimiter on any other value type. */
  const updateValueConfig = (patch: Partial<Pick<KeyedAuthorizationMapping, 'valueType' | 'delimiter'>>): void => {
    const merged = {...mapping, ...patch};
    const delimiter = merged.valueType === 'string' ? merged.delimiter : '';
    onChange({...patch, delimiter, values: reconcileOperators(merged.values, merged.valueType, delimiter)});
  };

  /** Toggling into Advanced can surface rules seeded (or left) on an operator that's invalid for the
   * mapping's current valueType/delimiter — e.g. a delimiter set while still in the simple/direct
   * mode, which is only ever validated against operators once a rule table exists to check it
   * against. Reconcile on every toggle, in both directions, so the operator picker is never left
   * showing a value it doesn't itself offer. */
  const toggleAdvanced = (isAdvanced: boolean): void =>
    onChange({isAdvanced, values: reconcileOperators(mapping.values, mapping.valueType, mapping.delimiter)});

  const updateRule = (valueKey: number, patch: {operator?: AuthorizationOperator; value?: string}): void =>
    onChange({values: mapping.values.map((v) => (v.key === valueKey ? {...v, ...patch} : v))});

  const updateGrants = (
    valueKey: number,
    patch: Partial<Pick<KeyedAuthorizationValue, 'roleIds' | 'groupIds' | 'permissions'>>,
  ): void => onChange({values: mapping.values.map((v) => (v.key === valueKey ? {...v, ...patch} : v))});

  const addValue = (): void =>
    onChange({values: [...mapping.values, newValueRow(defaultOperatorFor(mapping.valueType, mapping.delimiter))]});

  const removeValue = (valueKey: number): void => onChange({values: mapping.values.filter((v) => v.key !== valueKey)});

  return (
    <Box
      sx={{border: '1px solid', borderColor: 'divider', borderRadius: 2, p: 2}}
      data-testid={`authorization-mapping-${mapping.key}`}
    >
      <Stack direction="row" spacing={2} alignItems="flex-start" sx={{mb: 2}}>
        <FormControl sx={{flex: 1}}>
          <FormLabel sx={{mb: 0.75}}>{t('authorizationRules.claim.label')}</FormLabel>
          <TextField
            placeholder={t('authorizationRules.claim.placeholder')}
            value={mapping.claim}
            onChange={(e) => onChange({claim: e.target.value})}
            error={mappingIsAttempted(mapping) && mapping.claim.trim() === ''}
            inputProps={{'aria-label': t('authorizationRules.claim.label')}}
          />
        </FormControl>
        {mapping.isAdvanced && (
          <FormControl sx={{width: 160}}>
            <FormLabel sx={{mb: 0.75}}>{t('authorizationRules.valueType.label')}</FormLabel>
            <Select
              value={mapping.valueType}
              onChange={(e) => updateValueConfig({valueType: e.target.value as AuthorizationValueType})}
              inputProps={{'aria-label': t('authorizationRules.valueType.label')}}
            >
              {VALUE_TYPES.map((vt) => (
                <MenuItem key={vt} value={vt}>
                  {t(`authorizationRules.valueType.${vt}`)}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        )}
        {(!mapping.isAdvanced || mapping.valueType === 'string') && (
          <FormControl sx={{width: 220}}>
            <FormLabel sx={{mb: 0.75}}>{t('authorizationRules.delimiter.label')}</FormLabel>
            <DelimiterField
              value={mapping.delimiter}
              onChange={(delimiter) => (mapping.isAdvanced ? updateValueConfig({delimiter}) : onChange({delimiter}))}
            />
          </FormControl>
        )}
        <IconButton color="error" onClick={onRemove} aria-label={t('authorizationRules.mapping.remove')}>
          <Trash2 size={16} />
        </IconButton>
      </Stack>

      <Stack
        direction="row"
        spacing={2}
        alignItems="center"
        justifyContent="space-between"
        sx={{cursor: 'pointer'}}
        onClick={() => toggleAdvanced(!mapping.isAdvanced)}
      >
        <Typography variant="body2" fontWeight={600}>
          {t('authorizationMapping.advanced.toggle.label')}
        </Typography>
        <Switch
          checked={mapping.isAdvanced}
          onChange={(e) => toggleAdvanced(e.target.checked)}
          onClick={(e) => e.stopPropagation()}
          slotProps={{input: {role: 'switch', 'aria-label': t('authorizationMapping.advanced.toggle.label')}}}
        />
      </Stack>
      <Typography variant="caption" color="text.secondary" sx={{mb: 2, display: 'block'}}>
        {t('authorizationMapping.advanced.toggle.helper')}
      </Typography>

      {mapping.isAdvanced ? (
        <>
          <Typography variant="body2" color="text.secondary" fontWeight={600}>
            {t('authorizationRules.values.title')}
          </Typography>
          <Typography variant="caption" color="text.secondary" sx={{mb: 1, display: 'block'}}>
            {t('authorizationRules.values.helper')}
          </Typography>
          <Stack direction="column" spacing={1.5}>
            {mapping.values.map((entry) => (
              <ValueRow
                key={entry.key}
                claim={mapping.claim}
                entry={entry}
                valueType={mapping.valueType}
                delimiter={mapping.delimiter}
                canRemove={mapping.values.length > 1}
                onUpdateRule={(patch) => updateRule(entry.key, patch)}
                onUpdateGrants={(patch) => updateGrants(entry.key, patch)}
                onRemoveValue={() => removeValue(entry.key)}
              />
            ))}
          </Stack>
          <Button variant="text" size="small" startIcon={<Plus size={16} />} onClick={addValue} sx={{mt: 1.5}}>
            {t('authorizationRules.value.add')}
          </Button>
        </>
      ) : (
        <Stack direction="row" spacing={2}>
          <FormControl sx={{width: 200}}>
            <FormLabel sx={{mb: 0.75}}>{t('authorizationRules.targetType.label')}</FormLabel>
            <Select
              value={mapping.targetType}
              onChange={(e) => onChange({targetType: e.target.value as AuthorizationTargetType, resourceServerId: ''})}
              inputProps={{'aria-label': t('authorizationRules.targetType.label')}}
            >
              {TARGET_TYPES.map((type) => (
                <MenuItem key={type} value={type}>
                  {targetTypeLabel(t, type)}
                </MenuItem>
              ))}
            </Select>
          </FormControl>

          {mapping.targetType === 'permission' && (
            <FormControl sx={{flex: 1}}>
              <FormLabel sx={{mb: 0.75}}>{t('authorizationRules.target.resourceServer.label')}</FormLabel>
              <Select
                fullWidth
                displayEmpty
                value={mapping.resourceServerId}
                onChange={(e) => onChange({resourceServerId: e.target.value})}
                renderValue={(value) =>
                  servers.find((s) => s.id === value)?.name ?? t('authorizationRules.target.resourceServer.placeholder')
                }
                inputProps={{'aria-label': t('authorizationRules.target.resourceServer.label')}}
              >
                {servers.map((server) => (
                  <MenuItem key={server.id} value={server.id}>
                    {server.name}
                  </MenuItem>
                ))}
              </Select>
            </FormControl>
          )}
        </Stack>
      )}
    </Box>
  );
}

export default function AuthorizationMappingSection({
  initialRuleConfig = undefined,
  initialDirectConfig = undefined,
  onRuleChange,
  onDirectChange,
}: AuthorizationMappingSectionProps): JSX.Element {
  const {t} = useTranslation('connections');
  const {data: serversData} = useGetResourceServers({limit: PICKER_PAGE_SIZE});
  const servers: PickerOption[] = (serversData?.resourceServers ?? []).map((s) => ({id: s.id, name: s.name}));

  const [mappings, setMappings] = useState<KeyedAuthorizationMapping[]>(() =>
    fromAuthorizationMappings(initialDirectConfig, initialRuleConfig),
  );

  const emit = (next: KeyedAuthorizationMapping[]): void => {
    setMappings(next);
    // A row that's been touched but isn't complete on its active side is dropped silently by
    // toAuthorizationMappings rather than saved, so it must block save here too — otherwise the
    // visual error on an advanced row's value is the only sign anything was lost.
    const hasIncompleteRow = next.some((m) => mappingIsAttempted(m) && !mappingIsComplete(m));
    const {rules, direct} = toAuthorizationMappings(next);
    onRuleChange(rules, !hasIncompleteRow);
    onDirectChange(direct, !hasIncompleteRow);
  };

  const addMapping = (): void => emit([...mappings, newAuthorizationMapping()]);

  const updateMapping = (key: number, patch: Partial<KeyedAuthorizationMapping>): void =>
    emit(mappings.map((m) => (m.key === key ? {...m, ...patch} : m)));

  const removeMapping = (key: number): void => emit(mappings.filter((m) => m.key !== key));

  return (
    <SettingsCard title={t('authorizationMapping.title')} description={t('authorizationMapping.description')}>
      <Stack direction="column" spacing={2.5}>
        {mappings.map((mapping) => (
          <MappingRow
            key={mapping.key}
            mapping={mapping}
            servers={servers}
            onChange={(patch) => updateMapping(mapping.key, patch)}
            onRemove={() => removeMapping(mapping.key)}
          />
        ))}

        <Box>
          <Button variant="text" color="primary" size="small" startIcon={<Plus size={16} />} onClick={addMapping}>
            {t('authorizationRules.mapping.add')}
          </Button>
        </Box>
      </Stack>
    </SettingsCard>
  );
}
