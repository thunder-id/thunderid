// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useState, type JSX} from 'react';
import type {SchemaPropertyInput} from '../../../types/user-types';
import SchemaPropertyEditor from '../../shared/SchemaPropertyEditor';

export interface EditSchemaSettingsProps {
  properties: SchemaPropertyInput[];
  onPropertiesChange: (properties: SchemaPropertyInput[]) => void;
  userTypeHandle: string;
  disabled?: boolean;
}

/**
 * Schema settings tab content for the User Type edit page.
 * Displays the property editor cards for defining user type schema fields.
 */
export default function EditSchemaSettings({
  properties,
  onPropertiesChange,
  userTypeHandle,
  disabled = false,
}: EditSchemaSettingsProps): JSX.Element {
  const [enumInput, setEnumInput] = useState<Record<string, string>>({});

  return (
    <SchemaPropertyEditor
      properties={properties}
      onPropertiesChange={onPropertiesChange}
      enumInput={enumInput}
      onEnumInputChange={setEnumInput}
      userTypeHandle={userTypeHandle}
      disabled={disabled}
      isEditMode
    />
  );
}
