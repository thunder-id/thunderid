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

import {Box, FormControl, FormLabel, IconButton, Stack, TextField, Tooltip} from '@wso2/oxygen-ui';
import {Plus, Trash} from '@wso2/oxygen-ui-icons-react';
import {useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import PanelActionButton from './PanelActionButton';
import type {Resource} from '../../models/resources';

const parseClasses = (value: string): string[] => {
  const classes = (value ?? '').split(/\s+/).filter(Boolean);
  return classes.length > 0 ? classes : [''];
};

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

  const commitClasses = (updated: string[], debounce?: boolean): void => {
    setClassNames(updated);
    onChange(propertyKey, updated.join(' '), resource, debounce);
  };

  const handleAdd = (): void => {
    commitClasses([...classNames, '']);
  };

  const handleRemove = (index: number): void => {
    commitClasses(classNames.filter((_, i) => i !== index));
  };

  const handleChange = (index: number, value: string): void => {
    commitClasses(
      classNames.map((className, i) => (i === index ? value : className)),
      true,
    );
  };

  return (
    <Box>
      <FormControl fullWidth>
        <FormLabel htmlFor={`${resource.id}-${propertyKey}`}>
          {t('flows:core.elements.classesPropertyField.label')}
        </FormLabel>

        <Stack spacing={2} id={`${resource.id}-${propertyKey}`}>
          {classNames.map((className, index) => (
            // eslint-disable-next-line react/no-array-index-key
            <Stack key={index} direction="row" spacing={1} alignItems="flex-start">
              <TextField
                fullWidth
                value={className}
                onChange={(e) => handleChange(index, e.target.value)}
                placeholder={t('flows:core.elements.classesPropertyField.placeholder')}
              />
              {classNames.length > 1 && (
                <Tooltip title={t('common:actions.delete')}>
                  <IconButton onClick={() => handleRemove(index)} color="error">
                    <Trash size={20} />
                  </IconButton>
                </Tooltip>
              )}
            </Stack>
          ))}

          <Box>
            <PanelActionButton startIcon={<Plus size={16} />} onClick={handleAdd}>
              {t('flows:core.elements.classesPropertyField.addClass')}
            </PanelActionButton>
          </Box>
        </Stack>
      </FormControl>
    </Box>
  );
}

export default ClassesPropertyField;
