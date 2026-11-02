// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {isI18nTemplatePattern, isMetaTemplatePattern} from '@thunderid/utils';
import {
  Autocomplete,
  type AutocompleteRenderInputParams,
  Box,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  InputAdornment,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {SquareFunction} from '@wso2/oxygen-ui-icons-react';
import startCase from 'lodash-es/startCase';
import {useEffect, useMemo, useState, type ChangeEvent, type ReactElement, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import DynamicValuePopover from './DynamicValuePopover';
import useResourceFieldError from '../../hooks/useResourceFieldError';
import {ResourceTypes, type Resource} from '../../models/resources';

/**
 * Props interface of {@link TextPropertyField}
 */
export interface TextPropertyFieldPropsInterface {
  /**
   * The resource associated with the property.
   */
  resource: Resource;
  /**
   * The key of the property.
   */
  propertyKey: string;
  /**
   * The value of the property.
   */
  propertyValue: string;
  /**
   * The event handler for the property change.
   * @param propertyKey - The key of the property.
   * @param newValue - The new value of the property.
   * @param resource - The resource associated with the property.
   */
  onChange: (propertyKey: string, newValue: string, resource: Resource, debounce?: boolean) => void;
  /**
   * Known values offered as autocomplete suggestions. The field stays free text,
   * so any value outside this list is still accepted.
   */
  suggestions?: string[];
  /**
   * Whether the field offers i18n/meta template insertion. Structural values such as
   * layout properties take literal values only.
   */
  supportsDynamicValue?: boolean;
  /**
   * Additional props.
   */
  [key: string]: unknown;
}

/**
 * Text property field component for rendering text input fields.
 *
 * @param props - Props injected to the component.
 * @returns The TextPropertyField component.
 */
function TextPropertyField({
  resource,
  propertyKey,
  propertyValue,
  onChange,
  suggestions = undefined,
  supportsDynamicValue = true,
  ...rest
}: TextPropertyFieldPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {resolve} = useTemplateLiteralResolver();
  const [isDynamicValuePopoverOpen, setIsDynamicValuePopoverOpen] = useState<boolean>(false);
  const [localValue, setLocalValue] = useState<string>(propertyValue);
  const [iconButtonEl, setIconButtonEl] = useState<HTMLButtonElement | null>(null);

  /**
   * Sync local state when propertyValue changes from external sources.
   */
  useEffect(() => {
    setLocalValue(propertyValue);
  }, [propertyValue]);

  /**
   * Check if the property value matches any dynamic value pattern (i18n or meta).
   */
  const isDynamic: boolean = useMemo(
    () => isI18nTemplatePattern(propertyValue) || isMetaTemplatePattern(propertyValue),
    [propertyValue],
  );

  /**
   * Check specifically for i18n pattern to resolve and display a preview.
   */
  const isI18nPattern: boolean = useMemo(() => isI18nTemplatePattern(propertyValue), [propertyValue]);

  /**
   * Resolve the i18n value if the pattern is detected.
   */
  const resolvedI18nValue: string = useMemo(
    () => (isI18nPattern ? (resolve(propertyValue, {t}) ?? '') : ''),
    [propertyValue, isI18nPattern, t, resolve],
  );

  /**
   * Get the error message for the text property field.
   */
  const errorMessage: string = useResourceFieldError(resource?.id, propertyKey);

  /**
   * Handles the toggle of the dynamic value popover.
   */
  const handleDynamicValueToggle = () => {
    setIsDynamicValuePopoverOpen(!isDynamicValuePopoverOpen);
  };

  /**
   * Handles the closing of the dynamic value popover.
   */
  const handleDynamicValueClose = () => {
    setIsDynamicValuePopoverOpen(false);
  };

  const isIdField = propertyKey === 'id';

  // A step's id is the node's identity: renaming it remounts this field, so it
  // commits on blur/Enter instead of per keystroke. A rejected rename leaves the
  // resource untouched and the field falls back to the current id.
  const commitsOnBlur = isIdField && resource.resourceType === ResourceTypes.Step;

  // Ids are identifiers and structural values map straight to CSS, so neither
  // offers dynamic/i18n value insertion.
  const offersDynamicValue = !isIdField && supportsDynamicValue;

  const dynamicValueAdornment = !offersDynamicValue ? null : (
    <InputAdornment position="end">
      <Tooltip title={t('flows:core.elements.textPropertyField.tooltip.configureDynamicValue')}>
        <IconButton
          ref={setIconButtonEl}
          onClick={handleDynamicValueToggle}
          size="small"
          edge="end"
          color={isDynamic ? 'primary' : 'default'}
        >
          <SquareFunction size={16} />
        </IconButton>
      </Tooltip>
    </InputAdornment>
  );

  const dynamicValueSx = isDynamic
    ? {
        '& .MuiOutlinedInput-root': {
          backgroundColor: 'rgba(var(--mui-palette-primary-mainChannel) / 0.1)',
          '& fieldset': {
            borderColor: 'primary.main',
          },
          '&:hover fieldset': {
            borderColor: 'primary.dark',
          },
          '&.Mui-focused fieldset': {
            borderColor: 'primary.main',
          },
        },
      }
    : undefined;

  const placeholder: string = t('flows:core.elements.textPropertyField.placeholder', {
    propertyName: startCase(propertyKey),
  });

  const textField: ReactElement = (
    <TextField
      fullWidth
      id={`${resource.id}-${propertyKey}`}
      value={localValue}
      error={!!errorMessage}
      onChange={(e: ChangeEvent<HTMLInputElement>) => {
        setLocalValue(e.target.value);
        if (!commitsOnBlur) {
          onChange(propertyKey, e.target.value, resource, true);
        }
      }}
      onBlur={() => {
        if (!commitsOnBlur || localValue === propertyValue) {
          return;
        }
        onChange(propertyKey, localValue, resource);
        // A successful rename remounts this field (its key carries the id), so
        // this reset only takes effect when the rename is rejected — reverting
        // the field to the actual id, since no prop change will sync it.
        setLocalValue(propertyValue);
      }}
      onKeyDown={(e) => {
        if (commitsOnBlur && e.key === 'Enter') {
          (e.target as HTMLInputElement).blur();
        }
      }}
      placeholder={placeholder}
      sx={dynamicValueSx}
      InputProps={dynamicValueAdornment ? {endAdornment: dynamicValueAdornment} : undefined}
      {...rest}
    />
  );

  return (
    <Box>
      <FormControl fullWidth>
        <FormLabel htmlFor={`${resource.id}-${propertyKey}`}>{startCase(propertyKey)}</FormLabel>
        {suggestions ? (
          <Autocomplete
            freeSolo
            options={suggestions}
            value={localValue}
            inputValue={localValue}
            onChange={(_event: SyntheticEvent, newValue: string | null) => {
              const nextValue: string = newValue ?? '';
              setLocalValue(nextValue);
              onChange(propertyKey, nextValue, resource);
            }}
            onInputChange={(_event: SyntheticEvent, newValue: string, reason: string) => {
              if (reason !== 'input' && reason !== 'clear') {
                return;
              }
              setLocalValue(newValue);
              onChange(propertyKey, newValue, resource, true);
            }}
            renderInput={(params: AutocompleteRenderInputParams) => {
              const {InputProps: acInputProps, ...restParams} = params;
              return (
                <TextField
                  {...restParams}
                  fullWidth
                  // Keep the id the FormLabel points at, otherwise the label targets
                  // MUI's generated Autocomplete input id and loses click-to-focus
                  // and its accessible name.
                  id={`${resource.id}-${propertyKey}`}
                  error={!!errorMessage}
                  placeholder={placeholder}
                  sx={dynamicValueSx}
                  InputProps={{
                    ...acInputProps,
                    endAdornment: (
                      <>
                        {dynamicValueAdornment}
                        {acInputProps?.endAdornment}
                      </>
                    ),
                  }}
                  {...rest}
                />
              );
            }}
          />
        ) : (
          textField
        )}
      </FormControl>
      {errorMessage && <FormHelperText error>{errorMessage}</FormHelperText>}
      {isI18nPattern && resolvedI18nValue && (
        // The resolved text is a preview, not a field. Keeping it to a single caption
        // line preserves the form's vertical rhythm instead of interrupting it with a
        // card per templated property.
        <Tooltip title={t('flows:core.elements.textPropertyField.resolvedValue')}>
          <Typography
            variant="caption"
            color="text.secondary"
            sx={{
              display: 'block',
              mt: 0.5,
              pl: 0.25,
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
          >
            {resolvedI18nValue}
          </Typography>
        </Tooltip>
      )}
      {offersDynamicValue && (
        <DynamicValuePopover
          open={isDynamicValuePopoverOpen}
          anchorEl={iconButtonEl}
          propertyKey={propertyKey}
          onClose={handleDynamicValueClose}
          value={propertyValue}
          onChange={(newValue: string) => onChange(propertyKey, newValue, resource)}
        />
      )}
    </Box>
  );
}

export default TextPropertyField;
