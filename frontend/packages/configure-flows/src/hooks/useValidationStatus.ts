// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useContext} from 'react';
import type {ValidationContextProps} from '../context/ValidationContext';
import {ValidationContext} from '../context/ValidationContext';
/**
 * Custom hook to access the validation status context.
 *
 * @returns The validation context props.
 */
const useValidationStatus = (): ValidationContextProps => {
  const context: ValidationContextProps = useContext(ValidationContext);

  if (!context) {
    throw new Error('useValidationStatus must be used within a ValidationProvider');
  }

  return context;
};

export default useValidationStatus;
