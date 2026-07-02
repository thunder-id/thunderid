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

import {I18nTextInput} from '@thunderid/components';
import {useResolveDisplayName} from '@thunderid/hooks';
import {
  Alert,
  Box,
  Button,
  Checkbox,
  Chip,
  Collapse,
  FormControl,
  FormControlLabel,
  FormLabel,
  IconButton,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronRight, Info, Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import AttributeLibraryPanel from './AttributeLibraryPanel';
import type {Attribute, SchemaPropertyInput, UIPropertyType} from '../../types/user-types';
import {attributeToProperty} from '../../utils/attributeToProperty';
import {invalidateI18nCache} from '../../utils/invalidateI18nCache';

const supportsUnique = (type: UIPropertyType): boolean => type === 'string' || type === 'number' || type === 'enum';

const supportsCredential = (type: UIPropertyType): boolean => type === 'string' || type === 'number';

/**
 * Props for the {@link SchemaPropertyEditor} component.
 *
 * @public
 */
export interface SchemaPropertyEditorProps {
  properties: SchemaPropertyInput[];
  onPropertiesChange: (properties: SchemaPropertyInput[]) => void;
  enumInput: Record<string, string>;
  onEnumInputChange: (enumInput: Record<string, string>) => void;
  userTypeName?: string;
  disabled?: boolean;
  /** Rendered in the right column after the property rows (e.g. the display-attribute selector). */
  footer?: ReactNode;
}

/**
 * Two-panel schema builder shared by the create wizard and the edit "Schema" tab:
 * a left attribute library and a right list of properties rendered as collapsible
 * rows. Properties can be seeded from the library or added as blank custom
 * properties. For library-seeded properties the name/type/unique/credential all
 * stay locked to the definition; custom properties are fully editable.
 *
 * @public
 */
export default function SchemaPropertyEditor({
  properties,
  onPropertiesChange,
  enumInput,
  onEnumInputChange,
  userTypeName = undefined,
  disabled = false,
  footer = undefined,
}: SchemaPropertyEditorProps): JSX.Element {
  const {t} = useTranslation();
  const {resolveDisplayName} = useResolveDisplayName({handlers: {t}});
  const [expandedIds, setExpandedIds] = useState<Record<string, boolean>>({});

  const toggleExpanded = (id: string): void => {
    setExpandedIds((prev) => ({...prev, [id]: !prev[id]}));
  };

  const nextId = (): string => String(properties.reduce((max, p) => Math.max(max, Number(p.id) || 0), 0) + 1);

  const handleAddAttribute = (attribute: Attribute): void => {
    onPropertiesChange([...properties, attributeToProperty(attribute, nextId())]);
  };

  const handleAddCustomProperty = (): void => {
    const id = nextId();
    const newProperty: SchemaPropertyInput = {
      id,
      name: '',
      displayName: '',
      type: 'string',
      required: false,
      unique: false,
      credential: false,
      enum: [],
      regex: '',
      custom: true,
    };
    onPropertiesChange([...properties, newProperty]);
    setExpandedIds((prev) => ({...prev, [id]: true}));
  };

  const handleRemoveProperty = (id: string): void => {
    onPropertiesChange(properties.filter((prop) => prop.id !== id));
    const newEnumInput = {...enumInput};
    delete newEnumInput[id];
    onEnumInputChange(newEnumInput);
  };

  const handlePropertyChange = <K extends keyof SchemaPropertyInput>(
    id: string,
    field: K,
    value: SchemaPropertyInput[K],
  ): void => {
    onPropertiesChange(
      properties.map((prop) =>
        prop.id === id
          ? {
              ...prop,
              [field]: value,
              // Clear unique when credential is enabled.
              ...(field === 'credential' && value && {unique: false}),
              // Reset type-specific fields when the type changes.
              ...(field === 'type' && {
                enum: (value as UIPropertyType) === 'enum' ? prop.enum : [],
                regex: '',
                unique: supportsUnique(value as UIPropertyType) ? prop.unique : false,
                credential: supportsCredential(value as UIPropertyType) ? prop.credential : false,
              }),
            }
          : prop,
      ),
    );
  };

  const handleAddEnumValue = (propertyId: string): void => {
    const inputValue = enumInput[propertyId]?.trim();
    if (!inputValue) return;

    const target = properties.find((prop) => prop.id === propertyId);
    if (target?.enum.includes(inputValue)) return;

    onPropertiesChange(
      properties.map((prop) => (prop.id === propertyId ? {...prop, enum: [...prop.enum, inputValue]} : prop)),
    );
    onEnumInputChange({...enumInput, [propertyId]: ''});
  };

  const handleRemoveEnumValue = (propertyId: string, enumValue: string): void => {
    onPropertiesChange(
      properties.map((prop) =>
        prop.id === propertyId ? {...prop, enum: prop.enum.filter((val) => val !== enumValue)} : prop,
      ),
    );
  };

  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: {xs: '1fr', md: '300px 1fr'},
        gap: 3,
        alignItems: 'start',
      }}
    >
      <AttributeLibraryPanel
        existingNames={properties.map((p) => p.name)}
        onAdd={handleAddAttribute}
        disabled={disabled}
      />

      <Stack direction="column" spacing={3}>
        {properties.length === 0 && (
          <Paper variant="outlined" sx={{px: 3, py: 4, borderRadius: 2, borderStyle: 'dashed', textAlign: 'center'}}>
            <Typography variant="body2" color="text.secondary">
              {t('userTypes:attributes.emptyHint', 'Select attributes from the left to add them to this user type.')}
            </Typography>
          </Paper>
        )}

        {properties.map((property) => (
          <Paper
            key={property.id}
            variant="outlined"
            sx={{
              borderRadius: 2,
              overflow: 'hidden',
              transition: 'border-color 0.2s',
              '&:hover': {borderColor: 'primary.main'},
            }}
          >
            {/* Row header - click to expand the configuration */}
            <Box
              role="button"
              tabIndex={0}
              aria-label={
                property.displayName
                  ? resolveDisplayName(property.displayName)
                  : property.name || t('userTypes:newAttribute', 'New attribute')
              }
              aria-expanded={expandedIds[property.id] ?? false}
              onClick={() => toggleExpanded(property.id)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  toggleExpanded(property.id);
                }
              }}
              sx={{
                display: 'flex',
                alignItems: 'center',
                gap: 1,
                px: 2,
                py: 1.5,
                cursor: 'pointer',
                userSelect: 'none',
              }}
            >
              {expandedIds[property.id] ? <ChevronDown size={18} /> : <ChevronRight size={18} />}
              <Typography
                variant="subtitle2"
                sx={{fontWeight: 600, color: property.name ? 'text.primary' : 'text.secondary'}}
              >
                {property.displayName
                  ? resolveDisplayName(property.displayName)
                  : property.name || t('userTypes:newAttribute', 'New attribute')}
              </Typography>
              {property.required && <Chip label={t('common:form.required', 'Required')} size="small" color="primary" />}
              {property.unique && <Chip label={t('userTypes:unique', 'Unique')} size="small" variant="outlined" />}
              {property.credential && (
                <Chip label={t('userTypes:credential', 'Credential')} size="small" variant="outlined" />
              )}
              <Box sx={{flexGrow: 1}} />
              {!disabled && (
                <Tooltip title={t('userTypes:removeProperty', 'Remove property')}>
                  <IconButton
                    size="small"
                    color="error"
                    aria-label={t('userTypes:removeProperty', 'Remove property')}
                    onClick={(e) => {
                      e.stopPropagation();
                      handleRemoveProperty(property.id);
                    }}
                  >
                    <Trash2 size={16} />
                  </IconButton>
                </Tooltip>
              )}
            </Box>

            <Collapse in={expandedIds[property.id] ?? false} unmountOnExit>
              <Stack spacing={2} sx={{px: 3, pb: 3, pt: 1}}>
                <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', md: '1fr 1fr'}, gap: 2}}>
                  <FormControl fullWidth>
                    <FormLabel>{t('userTypes:propertyName', 'Name')}</FormLabel>
                    <TextField
                      value={property.name}
                      onChange={(e) => handlePropertyChange(property.id, 'name', e.target.value)}
                      placeholder={t('userTypes:propertyNamePlaceholder', 'e.g., email, age, address')}
                      size="small"
                      disabled={disabled || !property.custom}
                    />
                  </FormControl>

                  <FormControl fullWidth>
                    <FormLabel>{t('userTypes:propertyType', 'Type')}</FormLabel>
                    <Select
                      value={property.type}
                      onChange={(e) => handlePropertyChange(property.id, 'type', e.target.value as UIPropertyType)}
                      size="small"
                      disabled={disabled || !property.custom}
                    >
                      <MenuItem value="string">{t('userTypes:types.string', 'String')}</MenuItem>
                      <MenuItem value="number">{t('userTypes:types.number', 'Number')}</MenuItem>
                      <MenuItem value="boolean">{t('userTypes:types.boolean', 'Boolean')}</MenuItem>
                      <MenuItem value="enum">{t('userTypes:types.enum', 'Enum')}</MenuItem>
                    </Select>
                  </FormControl>
                </Box>

                <I18nTextInput
                  label={t('userTypes:displayName', 'Display Name')}
                  value={property.displayName}
                  onChange={(newValue: string) => handlePropertyChange(property.id, 'displayName', newValue)}
                  placeholder={t('userTypes:displayNamePlaceholder', 'e.g., First Name')}
                  onTranslationCreated={invalidateI18nCache}
                  labels={{
                    triggerTooltip: t('userTypes:displayNameI18n.tooltip', 'Configure translation'),
                    popoverTitle: t('userTypes:displayNameI18n.title', 'Translation'),
                    createTitle: t('userTypes:displayNameI18n.createTitle', 'Create New Translation'),
                    createTooltip: t('userTypes:displayNameI18n.createTooltip', 'Create a new translation key'),
                    languageLabel: t('userTypes:displayNameI18n.language', 'Language'),
                    keyLabel: t('userTypes:displayNameI18n.i18nKey', 'Translation Key'),
                    selectKeyPlaceholder: t('userTypes:displayNameI18n.selectKey', 'Select a translation key'),
                    valueLabel: t('userTypes:displayNameI18n.translationValue', 'Translation Value'),
                    resolvedValueLabel: t('userTypes:displayNameI18n.resolvedValue', 'Resolved value'),
                    keyRequiredError: t('userTypes:displayNameI18n.keyRequired', 'Translation key is required'),
                    valueRequiredError: t('userTypes:displayNameI18n.valueRequired', 'Translation value is required'),
                    invalidKeyFormatError: t(
                      'userTypes:displayNameI18n.invalidKeyFormat',
                      'Key may only contain letters, numbers, dots, hyphens, and underscores',
                    ),
                    cancelLabel: t('common:cancel', 'Cancel'),
                    createLabel: t('common:create', 'Create'),
                    closeLabel: t('common:close', 'Close'),
                    unknownError: t('common:errors.unknown', 'An unknown error occurred'),
                  }}
                  defaultNewKey={
                    userTypeName?.trim() && property.name.trim()
                      ? `${userTypeName.trim()}.${property.name.trim()}`
                      : undefined
                  }
                />

                {/* Checkbox options with info tooltips */}
                <Box sx={{display: 'flex', gap: 3}}>
                  <Tooltip
                    title={t('userTypes:tooltips.required', 'This field must be provided when creating a user')}
                    placement="top"
                    arrow
                  >
                    <FormControlLabel
                      control={
                        <Checkbox
                          checked={property.required}
                          onChange={(e) => handlePropertyChange(property.id, 'required', e.target.checked)}
                          disabled={disabled}
                        />
                      }
                      label={
                        <Stack direction="row" alignItems="center" spacing={0.5}>
                          <span>{t('common:form.required', 'Required')}</span>
                          <Info size={14} color="inherit" />
                        </Stack>
                      }
                    />
                  </Tooltip>
                  {supportsUnique(property.type) && (
                    <Tooltip
                      title={t('userTypes:tooltips.unique', 'Each user must have a distinct value for this field')}
                      placement="top"
                      arrow
                    >
                      <FormControlLabel
                        control={
                          <Checkbox
                            checked={property.unique}
                            disabled={disabled || !property.custom || property.credential}
                            onChange={(e) => handlePropertyChange(property.id, 'unique', e.target.checked)}
                          />
                        }
                        label={
                          <Stack direction="row" alignItems="center" spacing={0.5}>
                            <span>{t('userTypes:unique', 'Unique')}</span>
                            <Info size={14} color="inherit" />
                          </Stack>
                        }
                      />
                    </Tooltip>
                  )}
                  {supportsCredential(property.type) && (
                    <Tooltip
                      title={t(
                        'userTypes:tooltips.credential',
                        'Values will be hashed and not returned in API responses',
                      )}
                      placement="top"
                      arrow
                    >
                      <FormControlLabel
                        control={
                          <Checkbox
                            checked={property.credential}
                            disabled={disabled || !property.custom}
                            onChange={(e) => handlePropertyChange(property.id, 'credential', e.target.checked)}
                          />
                        }
                        label={
                          <Stack direction="row" alignItems="center" spacing={0.5}>
                            <span>{t('userTypes:credential', 'Credential')}</span>
                            <Info size={14} color="inherit" />
                          </Stack>
                        }
                      />
                    </Tooltip>
                  )}
                </Box>

                {/* Credential indicator */}
                {property.credential && (
                  <Alert severity="info" variant="outlined">
                    {t(
                      'userTypes:credentialHint',
                      'This field will be treated as a secret. Values will be hashed and cannot be retrieved.',
                    )}
                  </Alert>
                )}

                {/* String: regex pattern */}
                {property.type === 'string' && (
                  <FormControl fullWidth>
                    <FormLabel>{t('userTypes:regexPattern', 'Regular Expression Pattern (Optional)')}</FormLabel>
                    <TextField
                      value={property.regex}
                      onChange={(e) => handlePropertyChange(property.id, 'regex', e.target.value)}
                      placeholder={t('userTypes:regexPlaceholder', 'e.g., ^[a-zA-Z0-9]+$')}
                      size="small"
                      disabled={disabled}
                    />
                  </FormControl>
                )}

                {/* Enum: value input + chips */}
                {property.type === 'enum' && (
                  <FormControl fullWidth>
                    <FormLabel>{t('userTypes:enumValues', 'Allowed Values (Enum)')}</FormLabel>
                    <Box sx={{display: 'flex', gap: 1, mb: 1}}>
                      <TextField
                        value={enumInput[property.id] ?? ''}
                        onChange={(e) => onEnumInputChange({...enumInput, [property.id]: e.target.value})}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') {
                            e.preventDefault();
                            handleAddEnumValue(property.id);
                          }
                        }}
                        placeholder={t('userTypes:enumPlaceholder', 'Add value and press Enter')}
                        size="small"
                        fullWidth
                        disabled={disabled}
                      />
                      <Button
                        variant="outlined"
                        size="small"
                        onClick={() => handleAddEnumValue(property.id)}
                        disabled={disabled}
                      >
                        {t('common:actions.add', 'Add')}
                      </Button>
                    </Box>
                    {property.enum.length > 0 && (
                      <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                        {property.enum.map((val) => (
                          <Chip
                            key={val}
                            label={val}
                            onDelete={disabled ? undefined : () => handleRemoveEnumValue(property.id, val)}
                            size="small"
                          />
                        ))}
                      </Stack>
                    )}
                  </FormControl>
                )}
              </Stack>
            </Collapse>
          </Paper>
        ))}

        {!disabled && (
          <Button
            variant="outlined"
            startIcon={<Plus size={16} />}
            onClick={handleAddCustomProperty}
            fullWidth
            sx={{py: 1.5, borderStyle: 'dashed', '&:hover': {borderStyle: 'dashed'}}}
          >
            {t('userTypes:attributes.addCustom', 'Add Custom Attribute')}
          </Button>
        )}

        {footer}
      </Stack>
    </Box>
  );
}
