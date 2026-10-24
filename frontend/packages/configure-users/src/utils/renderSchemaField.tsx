// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {resolveAttributeRegex} from '@thunderid/utils';
import {
  Box,
  Typography,
  TextField,
  FormLabel,
  FormControl,
  Select,
  MenuItem,
  Checkbox,
  FormControlLabel,
} from '@wso2/oxygen-ui';
import {Controller} from 'react-hook-form';
import type {Control, FieldErrors, Path} from 'react-hook-form';
import ArrayFieldInput from '../components/ArrayFieldInput';
import CredentialFieldInput from '../components/CredentialFieldInput';
import type {PropertyDefinition} from '../models/users';

/**
 * Helper function to render a form field based on the property definition
 *
 * @param fieldName - The name of the field in the schema
 * @param fieldDef - The property definition from the schema
 * @param control - React Hook Form control object
 * @param errors - Form validation errors
 * @param resolveDisplayName - Optional callback to resolve display name (handles plain strings and i18n patterns)
 * @returns A rendered form field component or null for unsupported types
 */
const renderSchemaField = <T extends Record<string, unknown>>(
  fieldName: string,
  fieldDef: PropertyDefinition,
  control: Control<T>,
  errors: FieldErrors<T>,
  resolveDisplayName?: (displayName: string) => string,
) => {
  const isRequired = fieldDef.required ?? false;

  let fieldLabel = fieldName;
  if (fieldDef.displayName) {
    const resolved = resolveDisplayName?.(fieldDef.displayName);
    fieldLabel = (resolved !== '' ? resolved : undefined) ?? fieldDef.displayName;
  }

  // String fields
  if (fieldDef.type === 'string' || fieldDef.type === 'email' || fieldDef.type === 'mobile') {
    const stringDef = fieldDef;

    // Render as Select dropdown if enum values are provided
    if (stringDef.enum && stringDef.enum.length > 0) {
      const enumOptions = stringDef.enum;
      return (
        <FormControl key={fieldName}>
          <FormLabel htmlFor={fieldName}>
            {fieldLabel}
            {isRequired && <span style={{color: 'red'}}> *</span>}
          </FormLabel>
          <Controller
            name={fieldName as Path<T>}
            control={control}
            rules={{
              required: isRequired ? `${fieldLabel} is required` : false,
            }}
            render={({field}) => (
              <Select
                {...field}
                value={field.value ?? ''}
                id={fieldName}
                fullWidth
                required={isRequired}
                error={!!errors[fieldName]}
                displayEmpty
              >
                <MenuItem value="">
                  <em>Select {fieldLabel}</em>
                </MenuItem>
                {enumOptions.map((option) => (
                  <MenuItem key={option} value={option}>
                    {option.charAt(0).toUpperCase() + option.slice(1)}
                  </MenuItem>
                ))}
              </Select>
            )}
          />
          {errors[fieldName] && (
            <Typography variant="caption" color="error" sx={{mt: 0.5, ml: 1.75}}>
              {errors[fieldName]?.message as string}
            </Typography>
          )}
        </FormControl>
      );
    }

    // Render as TextField for regular string fields
    // Determine validation pattern
    let validationPattern;
    const regex = resolveAttributeRegex(stringDef.type, stringDef.regex);
    if (regex) {
      validationPattern = {
        value: new RegExp(regex),
        message: `${fieldLabel} format is invalid`,
      };
    }

    return (
      <FormControl key={fieldName}>
        <FormLabel htmlFor={fieldName}>
          {fieldLabel}
          {isRequired && <span style={{color: 'red'}}> *</span>}
        </FormLabel>
        <Controller
          name={fieldName as Path<T>}
          control={control}
          rules={{
            required: isRequired ? `${fieldLabel} is required` : false,
            pattern: validationPattern,
          }}
          render={({field}) =>
            stringDef.credential ? (
              <CredentialFieldInput
                id={fieldName}
                name={field.name}
                value={(field.value as string) ?? ''}
                placeholder={`Enter ${fieldLabel.toLowerCase()}`}
                required={isRequired}
                error={!!errors[fieldName]}
                helperText={errors[fieldName]?.message as string}
                color={errors[fieldName] ? 'error' : 'primary'}
                onChange={field.onChange}
                onBlur={field.onBlur}
                inputRef={field.ref}
              />
            ) : (
              <TextField
                {...field}
                value={field.value ?? ''}
                id={fieldName}
                type={fieldDef.type === 'mobile' ? 'tel' : 'text'}
                slotProps={{htmlInput: {inputMode: fieldDef.type === 'email' ? 'email' : undefined}}}
                placeholder={`Enter ${fieldLabel.toLowerCase()}`}
                fullWidth
                required={isRequired}
                variant="outlined"
                error={!!errors[fieldName]}
                helperText={errors[fieldName]?.message as string}
                color={errors[fieldName] ? 'error' : 'primary'}
              />
            )
          }
        />
      </FormControl>
    );
  }

  // Number fields
  if (fieldDef.type === 'number') {
    const numberDef = fieldDef;
    return (
      <FormControl key={fieldName}>
        <FormLabel htmlFor={fieldName}>
          {fieldLabel}
          {isRequired && <span style={{color: 'red'}}> *</span>}
        </FormLabel>
        <Controller
          name={fieldName as Path<T>}
          control={control}
          rules={{
            required: isRequired ? `${fieldLabel} is required` : false,
          }}
          render={({field}) =>
            numberDef.credential ? (
              <CredentialFieldInput
                id={fieldName}
                name={field.name}
                value={String(field.value ?? '')}
                placeholder={`Enter ${fieldLabel.toLowerCase()}`}
                required={isRequired}
                error={!!errors[fieldName]}
                helperText={errors[fieldName]?.message as string}
                color={errors[fieldName] ? 'error' : 'primary'}
                onChange={(e) => {
                  const {value} = e.target;
                  const num = Number(value);
                  field.onChange(value && !Number.isNaN(num) ? num : '');
                }}
                onBlur={field.onBlur}
                inputRef={field.ref}
              />
            ) : (
              <TextField
                {...field}
                value={field.value ?? ''}
                id={fieldName}
                type="number"
                placeholder={`Enter ${fieldLabel.toLowerCase()}`}
                fullWidth
                required={isRequired}
                variant="outlined"
                error={!!errors[fieldName]}
                helperText={errors[fieldName]?.message as string}
                color={errors[fieldName] ? 'error' : 'primary'}
                onChange={(e) => {
                  const {value} = e.target;
                  field.onChange(value ? Number(value) : '');
                }}
              />
            )
          }
        />
      </FormControl>
    );
  }

  // Boolean fields
  if (fieldDef.type === 'boolean') {
    return (
      <FormControl key={fieldName}>
        <Controller
          name={fieldName as Path<T>}
          control={control}
          render={({field}) => (
            <Box sx={{display: 'flex', alignItems: 'center', py: 1}}>
              <FormControlLabel
                control={
                  <Checkbox
                    id={fieldName}
                    name={field.name}
                    checked={field.value === true}
                    onChange={(e) => field.onChange(e.target.checked)}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                }
                required={isRequired}
                label={fieldLabel}
                sx={{mb: 2}}
              />
            </Box>
          )}
        />
      </FormControl>
    );
  }

  // Array fields
  if (fieldDef.type === 'array') {
    return (
      <FormControl key={fieldName} fullWidth>
        <FormLabel htmlFor={fieldName}>
          {fieldLabel}
          {isRequired && <span style={{color: 'red'}}> *</span>}
        </FormLabel>
        <Controller
          name={fieldName as Path<T>}
          control={control}
          rules={{
            required: isRequired ? `${fieldLabel} is required` : false,
            validate: (value) => {
              if (isRequired && (!Array.isArray(value) || value.length === 0)) {
                return `${fieldLabel} must have at least one value`;
              }
              return true;
            },
          }}
          render={({field}) => {
            const fieldValue = Array.isArray(field.value) ? field.value : [];
            return (
              <Box>
                <ArrayFieldInput value={fieldValue} onChange={field.onChange} fieldLabel={fieldLabel} />
                {errors[fieldName] && (
                  <Typography variant="caption" color="error" sx={{mt: 0.5, ml: 1.75}}>
                    {errors[fieldName]?.message as string}
                  </Typography>
                )}
              </Box>
            );
          }}
        />
      </FormControl>
    );
  }

  // For unsupported types, return null
  return null;
};

export default renderSchemaField;
