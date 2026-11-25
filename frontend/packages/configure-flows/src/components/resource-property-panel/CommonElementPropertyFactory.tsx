// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {loadOxygenIcons, OxygenIcon, parseStackItems, type OxygenIconSet} from '@thunderid/design';
import {useLogger} from '@thunderid/logger/react';
import {
  Autocomplete,
  type AutocompleteRenderInputParams,
  Box,
  FormControl,
  FormLabel,
  MenuItem,
  Select,
  TextField,
} from '@wso2/oxygen-ui';
import startCase from 'lodash-es/startCase';
import {type ReactElement, useState, useCallback} from 'react';
import {useTranslation} from 'react-i18next';
import CheckboxPropertyField from './CheckboxPropertyField';
import IconPickerListbox from './IconPickerListbox';
import RichTextActionFields from './rich-text/RichTextActionFields';
import RichTextWithTranslation from './rich-text/RichTextWithTranslation';
import TextPropertyField from './TextPropertyField';
import FlowBuilderElementConstants from '../../constants/FlowBuilderElementConstants';
import {ElementTypes} from '../../models/elements';
import type {Resource} from '../../models/resources';

const TEXT_ALIGN_VALUES = ['left', 'center', 'right', 'justify', 'inherit'] as const;

// Layout properties with a known value set. Rendered as free-text autocompletes so
// the common values are discoverable while custom CSS values stay allowed.
const LAYOUT_PROPERTY_SUGGESTIONS: Record<string, string[]> = {
  align: ['stretch', 'center', 'flex-start', 'flex-end', 'baseline', 'start', 'end'],
  direction: ['row', 'column', 'row-reverse', 'column-reverse'],
  justify: ['stretch', 'center', 'flex-start', 'flex-end', 'space-between', 'space-around', 'space-evenly'],
};

// Structural layout values map straight to CSS rather than being shown to end users,
// so they take literal values only: no i18n or meta template insertion.
const LAYOUT_PROPERTY_KEYS: string[] = ['align', 'direction', 'gap', 'items', 'justify'];

// A grid stack uses `direction` to pick the axis its slots run along. CSS Grid has no
// reverse auto-flow, so the reverse variants are only offered in flex mode.
const GRID_DIRECTION_VALUES: string[] = ['row', 'column'];

/**
 * Suggestions for a layout property, narrowed to the values the resource's current
 * layout mode actually honors.
 */
function getLayoutSuggestions(resource: Resource, propertyKey: string): string[] {
  const isGridStack: boolean =
    resource.type === ElementTypes.Stack &&
    parseStackItems((resource as Resource & {items?: string | number}).items) !== undefined;

  if (isGridStack && propertyKey === 'direction') {
    return GRID_DIRECTION_VALUES;
  }
  return LAYOUT_PROPERTY_SUGGESTIONS[propertyKey];
}

// ---------------------------------------------------------------------------
// IconPickerField — extracted so hooks are always called at the top level.
// ---------------------------------------------------------------------------

interface IconPickerFieldProps {
  resource: Resource;
  propertyKey: string;
  propertyValue: unknown;
  onChange: (propertyKey: string, newValue: unknown, resource: Resource, debounce?: boolean) => void;
}

function IconPickerField({resource, propertyKey, propertyValue, onChange}: IconPickerFieldProps): ReactElement {
  const logger = useLogger('ConfigureDetails');
  // The icon set is loaded on the first open of the picker; loadOxygenIcons caches it.
  const [loaded, setLoaded] = useState<OxygenIconSet | null>(null);

  const handleOpen = useCallback(() => {
    if (!loaded) {
      loadOxygenIcons()
        .then(setLoaded)
        .catch((error) => {
          logger.error('Failed to load icon module', {error});
        });
    }
  }, [loaded, logger]);

  const iconNames = loaded?.names ?? [];
  const totalCount = iconNames.length;

  // Only resolve the preview icon once the icon set is loaded.
  const previewIcon = loaded && typeof propertyValue === 'string' ? loaded.get(propertyValue) : undefined;

  return (
    <Box>
      <Autocomplete
        options={iconNames}
        // The picker lists one name per icon; a value saved under another name of an icon, such as
        // UserIcon, shows as the name the icon is listed under.
        value={typeof propertyValue === 'string' ? (loaded?.resolveName(propertyValue) ?? propertyValue) : null}
        loading={!loaded}
        onOpen={handleOpen}
        onChange={(_event: React.SyntheticEvent, newValue: string | null) => {
          onChange(propertyKey, newValue ?? '', resource);
        }}
        filterOptions={(options, {inputValue}) => {
          const query = inputValue.trim().toLowerCase();
          const filtered = query ? options.filter((n) => n.toLowerCase().includes(query)) : options;
          return filtered;
        }}
        // The listbox renders only the options in view (see IconPickerListbox), so every icon can be
        // listed. The keyboard only reaches rendered options, so it does not wrap around the list.
        slotProps={{listbox: {component: IconPickerListbox}}}
        disableListWrap
        noOptionsText={loaded ? 'No icons found' : 'Loading icons…'}
        renderInput={(params: AutocompleteRenderInputParams) => {
          const {InputProps: acInputProps, inputProps: acHtmlInputProps, InputLabelProps, ...restParams} = params;
          return (
            <TextField
              {...restParams}
              label={startCase(propertyKey)}
              size="small"
              placeholder={loaded ? `Search ${totalCount} icons…` : 'Loading icons…'}
              slotProps={{
                input: {
                  ...acInputProps,
                  startAdornment: previewIcon ? (
                    <Box sx={{display: 'flex', alignItems: 'center', pl: 0.5, pr: 0.5}}>
                      <OxygenIcon icon={previewIcon} size={16} />
                    </Box>
                  ) : (
                    acInputProps?.startAdornment
                  ),
                },
                htmlInput: acHtmlInputProps,
                inputLabel: InputLabelProps,
              }}
            />
          );
        }}
        renderOption={({key, ...props}: React.HTMLAttributes<HTMLLIElement> & {key: string}, option: string) => {
          const icon = loaded?.get(option);
          return (
            <li key={key} {...props}>
              <Box display="flex" alignItems="center" gap={1}>
                {icon && <OxygenIcon icon={icon} size={16} />}
                {option}
              </Box>
            </li>
          );
        }}
      />
    </Box>
  );
}

// ---------------------------------------------------------------------------
// CommonElementPropertyFactory — main factory
// ---------------------------------------------------------------------------

/**
 * Props interface of {@link CommonElementPropertyFactory}
 */
export interface CommonElementPropertyFactoryPropsInterface {
  resource: Resource;
  propertyKey: string;
  propertyValue: unknown;
  onChange: (propertyKey: string, newValue: unknown, resource: Resource, debounce?: boolean) => void;
  [key: string]: unknown;
}

/**
 * Factory to generate the common property configurator for the given element.
 *
 * @param props - Props injected to the component.
 * @returns The CommonElementPropertyFactory component.
 */
function CommonElementPropertyFactory({
  resource,
  propertyKey,
  propertyValue,
  onChange,
  ...rest
}: CommonElementPropertyFactoryPropsInterface): ReactElement | null {
  const {t} = useTranslation();
  if (propertyKey === 'label') {
    if (resource.type === ElementTypes.RichText) {
      return (
        <>
          <RichTextWithTranslation
            onChange={(html: string) => onChange(propertyKey, html, resource, true)}
            resource={resource}
            {...rest}
          />
          <RichTextActionFields resource={resource} onChange={onChange} />
        </>
      );
    }
  }

  if (resource.type === ElementTypes.Icon && propertyKey === 'name') {
    return (
      <IconPickerField
        resource={resource}
        propertyKey={propertyKey}
        propertyValue={propertyValue}
        onChange={onChange}
      />
    );
  }

  if (resource.type === ElementTypes.Text && propertyKey === 'align') {
    return (
      <FormControl fullWidth size="small">
        <FormLabel htmlFor={propertyKey}>{t('flows:core.elements.text.align.label')}</FormLabel>
        <Select
          id={propertyKey}
          value={typeof propertyValue === 'string' ? propertyValue : 'left'}
          onChange={(e) => onChange(propertyKey, e.target.value, resource)}
        >
          {TEXT_ALIGN_VALUES.map((value) => (
            <MenuItem key={value} value={value}>
              {t(`flows:core.elements.text.align.options.${value}`)}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
    );
  }

  const isLayoutProperty: boolean = LAYOUT_PROPERTY_KEYS.includes(propertyKey);

  // Own-property check: a bare index lookup also matches inherited members such as
  // `toString`, and element config keys are arbitrary.
  if (Object.hasOwn(LAYOUT_PROPERTY_SUGGESTIONS, propertyKey) && typeof propertyValue === 'string') {
    return (
      <TextPropertyField
        resource={resource}
        propertyKey={propertyKey}
        propertyValue={propertyValue}
        onChange={onChange}
        suggestions={getLayoutSuggestions(resource, propertyKey)}
        supportsDynamicValue={false}
        {...rest}
      />
    );
  }

  if (typeof propertyValue === 'boolean') {
    return (
      <CheckboxPropertyField
        resource={resource}
        propertyKey={propertyKey}
        propertyValue={propertyValue}
        onChange={onChange}
        {...rest}
      />
    );
  }

  if (typeof propertyValue === 'number') {
    return (
      <TextPropertyField
        resource={resource}
        propertyKey={propertyKey}
        propertyValue={String(propertyValue)}
        onChange={(key, value, res) => onChange(key, value !== '' ? Number(value) : 0, res, true)}
        supportsDynamicValue={!isLayoutProperty}
        type="number"
        slotProps={{htmlInput: {min: 0}}}
        {...rest}
      />
    );
  }

  if (typeof propertyValue === 'string') {
    return (
      <TextPropertyField
        resource={resource}
        propertyKey={propertyKey}
        propertyValue={propertyValue}
        onChange={onChange}
        {...rest}
      />
    );
  }

  if (resource.type === ElementTypes.Captcha) {
    return (
      <TextField
        fullWidth
        label="Provider"
        defaultValue={FlowBuilderElementConstants.DEFAULT_CAPTCHA_PROVIDER}
        slotProps={{
          htmlInput: {
            disabled: true,
            readOnly: true,
          },
        }}
        {...rest}
      />
    );
  }

  return null;
}

export default CommonElementPropertyFactory;
