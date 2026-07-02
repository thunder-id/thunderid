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

import type {Attribute, AttributeDataType, SchemaPropertyInput, UIPropertyType} from '../types/user-types';

/**
 * Map an attribute data type to the schema builder's UI property type.
 * The UI has no dedicated date type, so dates are represented as strings.
 */
function mapDataType(dataType: AttributeDataType): UIPropertyType {
  switch (dataType) {
    case 'number':
      return 'number';
    case 'boolean':
      return 'boolean';
    default:
      return 'string';
  }
}

/**
 * Build a schema property input from a selected attribute. The property
 * name is fixed to the attribute id; the rest of the definition is
 * pre-filled from the baseline definition.
 *
 * @param attribute - The selected attribute.
 * @param id - A unique local id for the form row.
 */
export function attributeToProperty(attribute: Attribute, id: string): SchemaPropertyInput {
  return {
    id,
    name: attribute.id,
    displayName: attribute.displayName ?? '',
    type: mapDataType(attribute.dataType),
    required: attribute.required,
    // Credential properties cannot also be unique.
    unique: attribute.credential ? false : attribute.unique,
    credential: attribute.credential,
    enum: [],
    regex: attribute.regex ?? '',
    // Seeded from the library: name/type/unique/credential stay locked to the definition.
    custom: false,
  };
}
