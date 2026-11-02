// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Autocomplete, Box, Chip, FormControl, FormLabel, TextField} from '@wso2/oxygen-ui';
import {useState, type ReactElement, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import type {Resource} from '../../models/resources';

const parseClasses = (value: string): string[] => (value ?? '').split(/\s+/).filter(Boolean);

/**
 * Props interface of {@link ClassesPropertyField}
 */
export interface ClassesPropertyFieldPropsInterface {
  /**
   * The resource associated with the property.
   */
  resource: Resource;
  /**
   * The key of the property.
   */
  propertyKey: string;
  /**
   * Space-separated list of CSS class names currently configured.
   */
  propertyValue: string;
  /**
   * The event handler for the property change.
   * @param propertyKey - The key of the property.
   * @param newValue - The new space-separated class list.
   * @param resource - The resource associated with the property.
   */
  onChange: (propertyKey: string, newValue: string, resource: Resource, debounce?: boolean) => void;
}

/**
 * Property field for editing a component's CSS classes as a list of rows, each removable,
 * with an "Add" button to append a new one. The rows are joined into a single space-separated
 * string when persisted.
 *
 * @param props - Props injected to the component.
 * @returns The ClassesPropertyField component.
 */
function ClassesPropertyField({
  resource,
  propertyKey,
  propertyValue,
  onChange,
}: ClassesPropertyFieldPropsInterface): ReactElement {
  const {t} = useTranslation();
  const [classNames, setClassNames] = useState<string[]>(() => parseClasses(propertyValue));

  const commitClasses = (updated: string[]): void => {
    // Class names are whitespace separated, so a typed value cannot contain spaces.
    const normalized: string[] = updated.flatMap((entry: string) => entry.split(/\s+/).filter(Boolean));
    const deduped: string[] = [...new Set(normalized)];
    setClassNames(deduped);
    onChange(propertyKey, deduped.join(' '), resource);
  };

  return (
    <Box>
      <FormControl fullWidth>
        <FormLabel htmlFor={`${resource.id}-${propertyKey}`}>
          {t('flows:core.elements.classesPropertyField.label')}
        </FormLabel>
        {/* One tag input rather than a stack of text fields: classes are short tokens,
            so they read better as chips and the field stays a fixed height. */}
        <Autocomplete
          multiple
          freeSolo
          autoSelect
          clearOnBlur
          options={[] as string[]}
          value={classNames}
          onChange={(_event: SyntheticEvent, newValue: string[]) => commitClasses(newValue)}
          renderTags={(value: string[], getTagProps) =>
            value.map((option: string, index: number) => {
              const {key, ...tagProps} = getTagProps({index});
              return <Chip key={key} size="small" label={option} {...tagProps} />;
            })
          }
          renderInput={(params) => (
            <TextField
              {...params}
              id={`${resource.id}-${propertyKey}`}
              placeholder={
                classNames.length === 0 ? t('flows:core.elements.classesPropertyField.placeholder') : undefined
              }
            />
          )}
        />
      </FormControl>
    </Box>
  );
}

export default ClassesPropertyField;
