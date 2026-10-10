// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {I18nTextInput} from '@thunderid/components';
import {
  Box,
  Stack,
  Button,
  Paper,
  FormControl,
  FormLabel,
  Select,
  MenuItem,
  TextField,
  Checkbox,
  FormControlLabel,
  Chip,
  IconButton,
  Tooltip,
  Alert,
  Dialog,
  DialogTitle,
  DialogContent,
  DialogContentText,
  DialogActions,
} from '@wso2/oxygen-ui';
import {Trash2, Plus, Info} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {SchemaPropertyInput, PropertyType} from '../../../models/property-definition';
import {invalidateI18nCache} from '../../../utils/invalidateI18nCache';

export interface EditSchemaSettingsProps {
  properties: SchemaPropertyInput[];
  onPropertiesChange: (properties: SchemaPropertyInput[]) => void;
  agentTypeHandle: string;
}

/**
 * Schema settings tab content for the Agent Type edit page.
 * Displays the property editor cards for defining agent type schema fields.
 */
export default function EditSchemaSettings({
  properties,
  onPropertiesChange,
  agentTypeHandle,
}: EditSchemaSettingsProps): JSX.Element {
  const {t} = useTranslation();
  const [enumInput, setEnumInput] = useState<Record<string, string>>({});
  const [credentialRemoveDialogOpen, setCredentialRemoveDialogOpen] = useState(false);
  const [pendingCredentialRemoveId, setPendingCredentialRemoveId] = useState<string | null>(null);

  const handlePropertyChange = <K extends keyof SchemaPropertyInput>(
    propertyId: string,
    field: K,
    value: SchemaPropertyInput[K],
  ): void => {
    onPropertiesChange(
      properties.map((prop) =>
        prop.id === propertyId
          ? {
              ...prop,
              [field]: value,
              ...(field === 'type' && {
                enum: (value as string) === 'enum' ? prop.enum : [],
                regex: '',
                unique:
                  (value as string) === 'string' || (value as string) === 'number' || (value as string) === 'enum'
                    ? prop.unique
                    : false,
                credential: (value as string) === 'string' || (value as string) === 'number' ? prop.credential : false,
              }),
            }
          : prop,
      ),
    );
  };

  const handleRemoveProperty = (propertyId: string): void => {
    onPropertiesChange(properties.filter((prop) => prop.id !== propertyId));
    const newEnumInput = {...enumInput};
    delete newEnumInput[propertyId];
    setEnumInput(newEnumInput);
  };

  const handleAddProperty = (): void => {
    const maxId = properties.reduce((max, p) => Math.max(max, Number(p.id) || 0), 0);
    const newProperty: SchemaPropertyInput = {
      id: String(maxId + 1),
      name: '',
      displayName: '',
      type: 'string',
      required: false,
      unique: false,
      credential: false,
      enum: [],
      regex: '',
    };
    onPropertiesChange([...properties, newProperty]);
  };

  const handleAddEnumValue = (propertyId: string): void => {
    const inputValue = enumInput[propertyId]?.trim();
    if (!inputValue) return;

    const target = properties.find((p) => p.id === propertyId);
    if (target?.enum.includes(inputValue)) return;

    onPropertiesChange(
      properties.map((prop) => (prop.id === propertyId ? {...prop, enum: [...prop.enum, inputValue]} : prop)),
    );
    setEnumInput({...enumInput, [propertyId]: ''});
  };

  const handleRemoveEnumValue = (propertyId: string, enumValue: string): void => {
    onPropertiesChange(
      properties.map((prop) =>
        prop.id === propertyId ? {...prop, enum: prop.enum.filter((val) => val !== enumValue)} : prop,
      ),
    );
  };

  return (
    <Box>
      {properties.map((property) => (
        <Paper
          key={property.id}
          variant="outlined"
          sx={{
            position: 'relative',
            p: 3,
            mb: 2,
            borderRadius: 2,
            transition: 'border-color 0.2s',
            '&:hover': {borderColor: 'primary.main'},
            '&:hover .property-delete-btn': {opacity: 1},
          }}
        >
          {/* Remove button - visible on hover */}
          {properties.length > 1 && (
            <Tooltip title={t('agentTypes:removeProperty', 'Remove property')}>
              <IconButton
                className="property-delete-btn"
                size="small"
                color="error"
                onClick={() => handleRemoveProperty(property.id)}
                sx={{position: 'absolute', top: 8, right: 8, opacity: 0, transition: 'opacity 0.2s'}}
              >
                <Trash2 size={16} />
              </IconButton>
            </Tooltip>
          )}

          <Stack spacing={2}>
            <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', md: '1fr 1fr'}, gap: 2}}>
              <FormControl fullWidth>
                <FormLabel>{t('agentTypes:propertyName')}</FormLabel>
                <TextField
                  value={property.name}
                  onChange={(e) => handlePropertyChange(property.id, 'name', e.target.value)}
                  placeholder={t('agentTypes:propertyNamePlaceholder', 'e.g., email, age, address')}
                  size="small"
                />
              </FormControl>

              <FormControl fullWidth>
                <FormLabel>{t('agentTypes:propertyType', 'Type')}</FormLabel>
                <Select
                  value={property.type}
                  onChange={(e) => handlePropertyChange(property.id, 'type', e.target.value as PropertyType)}
                  size="small"
                >
                  <MenuItem value="string">{t('agentTypes:types.string', 'String')}</MenuItem>
                  <MenuItem value="number">{t('agentTypes:types.number', 'Number')}</MenuItem>
                  <MenuItem value="boolean">{t('agentTypes:types.boolean', 'Boolean')}</MenuItem>
                  <MenuItem value="enum">{t('agentTypes:types.enum', 'Enum')}</MenuItem>
                </Select>
              </FormControl>
            </Box>

            <I18nTextInput
              label={t('agentTypes:displayName', 'Display Name')}
              value={property.displayName}
              onChange={(newValue: string) => handlePropertyChange(property.id, 'displayName', newValue)}
              placeholder={t('agentTypes:displayNamePlaceholder', 'e.g., First Name')}
              onTranslationCreated={invalidateI18nCache}
              labels={{
                triggerTooltip: t('agentTypes:displayNameI18n.tooltip', 'Configure translation'),
                popoverTitle: t('agentTypes:displayNameI18n.title', 'Translation'),
                createTitle: t('agentTypes:displayNameI18n.createTitle', 'Create New Translation'),
                createTooltip: t('agentTypes:displayNameI18n.createTooltip', 'Create a new translation key'),
                languageLabel: t('agentTypes:displayNameI18n.language', 'Language'),
                keyLabel: t('agentTypes:displayNameI18n.i18nKey', 'Translation Key'),
                selectKeyPlaceholder: t('agentTypes:displayNameI18n.selectKey', 'Select a translation key'),
                valueLabel: t('agentTypes:displayNameI18n.translationValue', 'Translation Value'),
                resolvedValueLabel: t('agentTypes:displayNameI18n.resolvedValue', 'Resolved value'),
                keyRequiredError: t('agentTypes:displayNameI18n.keyRequired', 'Translation key is required'),
                valueRequiredError: t('agentTypes:displayNameI18n.valueRequired', 'Translation value is required'),
                invalidKeyFormatError: t(
                  'agentTypes:displayNameI18n.invalidKeyFormat',
                  'Key may only contain letters, numbers, dots, hyphens, and underscores',
                ),
                cancelLabel: t('common:cancel', 'Cancel'),
                createLabel: t('common:create', 'Create'),
                closeLabel: t('common:close', 'Close'),
                unknownError: t('common:errors.unknown', 'An unknown error occurred'),
              }}
              defaultNewKey={
                agentTypeHandle.trim() && property.name.trim()
                  ? `${agentTypeHandle.trim()}.${property.name.trim()}`
                  : undefined
              }
            />

            {/* Checkbox options with info tooltips */}
            <Box sx={{display: 'flex', gap: 3}}>
              <Tooltip
                title={t('agentTypes:tooltips.required', 'This field must be provided when creating a user')}
                placement="top"
                arrow
              >
                <FormControlLabel
                  control={
                    <Checkbox
                      checked={property.required}
                      onChange={(e) => handlePropertyChange(property.id, 'required', e.target.checked)}
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
              {(property.type === 'string' || property.type === 'number' || property.type === 'enum') && (
                <Tooltip
                  title={t('agentTypes:tooltips.unique', 'Each user must have a distinct value for this field')}
                  placement="top"
                  arrow
                >
                  <FormControlLabel
                    control={
                      <Checkbox
                        checked={property.unique}
                        disabled={property.credential}
                        onChange={(e) => handlePropertyChange(property.id, 'unique', e.target.checked)}
                      />
                    }
                    label={
                      <Stack direction="row" alignItems="center" spacing={0.5}>
                        <span>{t('agentTypes:unique', 'Unique')}</span>
                        <Info size={14} color="inherit" />
                      </Stack>
                    }
                  />
                </Tooltip>
              )}
              {(property.type === 'string' || property.type === 'number') && (
                <Tooltip
                  title={t('agentTypes:tooltips.credential', 'Values will be hashed and not returned in API responses')}
                  placement="top"
                  arrow
                >
                  <FormControlLabel
                    control={
                      <Checkbox
                        checked={property.credential}
                        onChange={({target: {checked}}) => {
                          if (!checked) {
                            setPendingCredentialRemoveId(property.id);
                            setCredentialRemoveDialogOpen(true);
                            return;
                          }
                          onPropertiesChange(
                            properties.map((prop) =>
                              prop.id === property.id ? {...prop, credential: checked, unique: false} : prop,
                            ),
                          );
                        }}
                      />
                    }
                    label={
                      <Stack direction="row" alignItems="center" spacing={0.5}>
                        <span>{t('agentTypes:credential', 'Credential')}</span>
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
                  'agentTypes:credentialHint',
                  'This field will be treated as a secret. Values will be hashed and cannot be retrieved.',
                )}
              </Alert>
            )}

            {/* String: regex pattern */}
            {property.type === 'string' && (
              <FormControl fullWidth>
                <FormLabel>{t('agentTypes:regexPattern', 'Regular Expression Pattern (Optional)')}</FormLabel>
                <TextField
                  value={property.regex}
                  onChange={(e) => handlePropertyChange(property.id, 'regex', e.target.value)}
                  placeholder={t('agentTypes:regexPlaceholder', 'e.g., ^[a-zA-Z0-9]+$')}
                  size="small"
                />
              </FormControl>
            )}

            {/* Enum: value input + chips */}
            {property.type === 'enum' && (
              <FormControl fullWidth>
                <FormLabel>{t('agentTypes:enumValues', 'Allowed Values (Enum)')}</FormLabel>
                <Box sx={{display: 'flex', gap: 1, mb: 1}}>
                  <TextField
                    value={enumInput[property.id] ?? ''}
                    onChange={(e) => setEnumInput({...enumInput, [property.id]: e.target.value})}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') {
                        e.preventDefault();
                        handleAddEnumValue(property.id);
                      }
                    }}
                    placeholder={t('agentTypes:enumPlaceholder', 'Add value and press Enter')}
                    size="small"
                    fullWidth
                  />
                  <Button variant="outlined" size="small" onClick={() => handleAddEnumValue(property.id)}>
                    {t('common:actions.add', 'Add')}
                  </Button>
                </Box>
                {property.enum.length > 0 && (
                  <Stack direction="row" spacing={1} flexWrap="wrap" useFlexGap>
                    {property.enum.map((val) => (
                      <Chip
                        key={val}
                        label={val}
                        onDelete={() => handleRemoveEnumValue(property.id, val)}
                        size="small"
                      />
                    ))}
                  </Stack>
                )}
              </FormControl>
            )}
          </Stack>
        </Paper>
      ))}

      {/* Add Property Button */}
      <Button
        variant="text"
        color="primary"
        startIcon={<Plus size={16} />}
        onClick={handleAddProperty}
        fullWidth
        sx={{
          py: 1.5,
          mb: 2,
          border: '1px dashed',
          borderColor: 'divider',
          '&:hover': {border: '1px dashed', borderColor: 'primary.main'},
        }}
      >
        {t('agentTypes:addProperty', 'Add Property')}
      </Button>

      {/* Credential Removal Confirmation Dialog */}
      <Dialog
        open={credentialRemoveDialogOpen}
        onClose={() => {
          setCredentialRemoveDialogOpen(false);
          setPendingCredentialRemoveId(null);
        }}
      >
        <DialogTitle>{t('agentTypes:removeCredentialDialog.title', 'Remove Credential Flag')}</DialogTitle>
        <DialogContent>
          <DialogContentText>
            {t(
              'agentTypes:removeCredentialDialog.description',
              'Removing the credential flag will cause this field to no longer be hashed or protected. Existing hashed values may become inaccessible. Are you sure you want to proceed?',
            )}
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button
            onClick={() => {
              setCredentialRemoveDialogOpen(false);
              setPendingCredentialRemoveId(null);
            }}
          >
            {t('common:actions.cancel', 'Cancel')}
          </Button>
          <Button
            color="warning"
            variant="contained"
            onClick={() => {
              if (pendingCredentialRemoveId) {
                onPropertiesChange(
                  properties.map((prop) =>
                    prop.id === pendingCredentialRemoveId ? {...prop, credential: false} : prop,
                  ),
                );
              }
              setCredentialRemoveDialogOpen(false);
              setPendingCredentialRemoveId(null);
            }}
          >
            {t('agentTypes:removeCredentialDialog.confirm', 'Remove Credential')}
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
