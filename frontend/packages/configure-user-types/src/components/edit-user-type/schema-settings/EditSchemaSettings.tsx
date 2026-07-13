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

import {useState, type JSX} from 'react';
import type {SchemaPropertyInput} from '../../../types/user-types';
import SchemaPropertyEditor from '../../shared/SchemaPropertyEditor';

export interface EditSchemaSettingsProps {
  properties: SchemaPropertyInput[];
  onPropertiesChange: (properties: SchemaPropertyInput[]) => void;
  userTypeName: string;
  disabled?: boolean;
}

/**
 * Schema settings tab content for the User Type edit page.
 * Displays the property editor cards for defining user type schema fields.
 */
export default function EditSchemaSettings({
  properties,
  onPropertiesChange,
  userTypeName,
  disabled = false,
}: EditSchemaSettingsProps): JSX.Element {
  const [enumInput, setEnumInput] = useState<Record<string, string>>({});

  return (
    <SchemaPropertyEditor
      properties={properties}
      onPropertiesChange={onPropertiesChange}
      enumInput={enumInput}
      onEnumInputChange={setEnumInput}
      userTypeName={userTypeName}
      disabled={disabled}
      isEditMode
    />
  );
}
