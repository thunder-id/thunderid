/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {useState, useCallback, FormEvent} from 'react';

/**
 * Generic form field configuration
 */
export interface FormField {
  initialValue?: string;
  name: string;
  required?: boolean;
  validator?: (value: string) => string | null;
}

/**
 * Form validation result
 */
export interface ValidationResult {
  errors: Record<string, string>;
  isValid: boolean;
}

/**
 * Configuration for the useForm hook
 */
export interface UseFormConfig<T extends Record<string, string>> {
  /**
   * Form field definitions
   */
  fields?: FormField[];
  /**
   * Initial form values
   */
  initialValues?: Partial<T>;
  /**
   * Custom required field validation message
   */
  requiredMessage?: string;
  /**
   * Whether to validate on blur (default: true)
   */
  validateOnBlur?: boolean;
  /**
   * Whether to validate on change (default: false)
   */
  validateOnChange?: boolean;
  /**
   * Global form validator function
   */
  validator?: (values: T) => Record<string, string>;
}

/**
 * Return type for the useForm hook
 */
export interface UseFormReturn<T extends Record<string, string>> {
  /**
   * Clear all errors
   */
  clearErrors: () => void;
  /**
   * Current validation errors
   */
  errors: Record<keyof T, string>;
  /**
   * Get field props for easy integration with form components
   */
  getFieldProps: (name: keyof T) => {
    error: string | undefined;
    name: keyof T;
    onBlur: () => void;
    onChange: (value: string) => void;
    required: boolean;
    touched: boolean;
    value: string;
  };
  /**
   * Handle form submission
   */
  handleSubmit: (onSubmit: (values: T) => void | Promise<void>) => (e?: FormEvent) => Promise<void>;
  /**
   * Whether the form has been submitted
   */
  isSubmitted: boolean;
  /**
   * Whether the form is currently valid
   */
  isValid: boolean;
  /**
   * Reset the form to initial values
   */
  reset: () => void;
  /**
   * Set a field error
   */
  setError: (name: keyof T, error: string) => void;
  /**
   * Set multiple field errors
   */
  setErrors: (errors: Partial<Record<keyof T, string>>) => void;
  /**
   * Mark a field as touched
   */
  setTouched: (name: keyof T, touched?: boolean) => void;
  /**
   * Mark multiple fields as touched
   */
  setTouchedFields: (touched: Partial<Record<keyof T, boolean>>) => void;
  /**
   * Set a single field value
   */
  setValue: (name: keyof T, value: string) => void;
  /**
   * Set multiple field values
   */
  setValues: (values: Partial<T>) => void;
  /**
   * Mark all fields as touched
   */
  touchAllFields: () => void;
  /**
   * Fields that have been touched by the user
   */
  touched: Record<keyof T, boolean>;
  /**
   * Validate a single field
   */
  validateField: (name: keyof T) => string | null;
  /**
   * Validate all fields
   */
  validateForm: () => ValidationResult;
  /**
   * Current form values
   */
  values: T;
}

/**
 * Generic form hook that provides comprehensive form state management and validation.
 *
 * @template T - The type of form values (must extend Record<string, string>)
 * @param config - Configuration options for the form
 * @returns Form state and methods
 *
 * @example
 * ```tsx
 * interface LoginForm {
 *   username: string;
 *   password: string;
 * }
 *
 * const {
 *   values,
 *   touched,
 *   errors,
 *   isValid,
 *   setValue,
 *   handleSubmit,
 *   getFieldProps
 * } = useForm<LoginForm>({
 *   initialValues: { username: '', password: '' },
 *   fields: [
 *     { name: 'username', required: true },
 *     { name: 'password', required: true }
 *   ]
 * });
 *
 * const onSubmit = handleSubmit((values) => {
 *   console.log('Form submitted:', values);
 * });
 *
 * return (
 *   <form onSubmit={onSubmit}>
 *     <input {...getFieldProps('username')} />
 *     <input {...getFieldProps('password')} type="password" />
 *     <button type="submit" disabled={!isValid}>Submit</button>
 *   </form>
 * );
 * ```
 */
export const useForm = <T extends Record<string, string>>(config: UseFormConfig<T> = {}): UseFormReturn<T> => {
  const {
    initialValues = {} as T,
    fields = [],
    validator,
    validateOnChange = false,
    validateOnBlur = true,
    requiredMessage = 'This field is required',
  } = config;

  // Initialize form state
  const [values, setFormValues] = useState<T>({...initialValues} as T);
  const [touched, setFormTouched] = useState<Record<keyof T, boolean>>({} as Record<keyof T, boolean>);
  const [errors, setFormErrors] = useState<Record<keyof T, string>>({} as Record<keyof T, string>);
  const [isSubmitted, setIsSubmitted] = useState(false);

  // Get field configuration by name
  const getFieldConfig: (name: keyof T) => FormField | undefined = useCallback(
    (name: keyof T): FormField | undefined => fields.find((field: FormField) => field.name === name),
    [fields],
  );

  // Validate a single field
  const validateField: (name: keyof T) => string | null = useCallback(
    (name: keyof T): string | null => {
      const value: string = values[name] || '';
      const fieldConfig: FormField | undefined = getFieldConfig(name);

      // Check required validation
      if (fieldConfig?.required && (!value || value.trim() === '')) {
        return requiredMessage;
      }

      // Run custom field validator
      if (fieldConfig?.validator) {
        const fieldError: string | null = fieldConfig.validator(value);
        if (fieldError) return fieldError;
      }

      return null;
    },
    [values, getFieldConfig, requiredMessage],
  );

  // Validate the entire form
  const validateForm: () => ValidationResult = useCallback((): ValidationResult => {
    const newErrors: Record<keyof T, string> = {} as Record<keyof T, string>;

    // Validate individual fields
    fields.forEach((field: FormField) => {
      const error: string | null = validateField(field.name as keyof T);
      if (error) {
        newErrors[field.name as keyof T] = error;
      }
    });

    // Run global validator if provided
    if (validator) {
      const globalErrors: Record<string, string> = validator(values);
      Object.keys(globalErrors).forEach((key: string) => {
        if (globalErrors[key]) {
          newErrors[key as keyof T] = globalErrors[key];
        }
      });
    }

    return {
      errors: newErrors,
      isValid: Object.keys(newErrors).length === 0,
    };
  }, [fields, validateField, validator, values]);

  // Check if form is currently valid
  const isValid: boolean = Object.keys(errors).length === 0;

  // Set a single field value
  const setValue: (name: keyof T, value: string) => void = useCallback(
    (name: keyof T, value: string): void => {
      setFormValues((prev: T) => ({
        ...prev,
        [name]: value,
      }));

      // Validate on change if enabled
      if (validateOnChange) {
        const error: string | null = validateField(name);
        setFormErrors((prev: Record<keyof T, string>) => {
          const newErrors: Record<keyof T, string> = {...prev};
          if (error) {
            newErrors[name] = error;
          } else {
            delete newErrors[name];
          }
          return newErrors;
        });
      }
    },
    [validateField, validateOnChange],
  );

  // Set multiple field values
  const setValues: (newValues: Partial<T>) => void = useCallback((newValues: Partial<T>): void => {
    setFormValues((prev: T) => ({
      ...prev,
      ...newValues,
    }));
  }, []);

  // Mark a field as touched
  const setTouched: (name: keyof T, isTouched?: boolean) => void = useCallback(
    (name: keyof T, isTouched = true): void => {
      setFormTouched((prev: Record<keyof T, boolean>) => ({
        ...prev,
        [name]: isTouched,
      }));

      // Validate on blur if enabled and field is touched
      if (validateOnBlur && isTouched) {
        const error: string | null = validateField(name);
        setFormErrors((prev: Record<keyof T, string>) => {
          const newErrors: Record<keyof T, string> = {...prev};
          if (error) {
            newErrors[name] = error;
          } else {
            delete newErrors[name];
          }
          return newErrors;
        });
      }
    },
    [validateField, validateOnBlur],
  );

  // Set multiple touched fields
  const setTouchedFields: (touchedFields: Partial<Record<keyof T, boolean>>) => void = useCallback(
    (touchedFields: Partial<Record<keyof T, boolean>>): void => {
      setFormTouched((prev: Record<keyof T, boolean>) => ({
        ...prev,
        ...touchedFields,
      }));
    },
    [],
  );

  // Mark all fields as touched
  const touchAllFields: () => void = useCallback((): void => {
    const allTouched: Record<keyof T, boolean> = fields.reduce(
      (acc: Record<keyof T, boolean>, field: FormField) => {
        acc[field.name as keyof T] = true;
        return acc;
      },
      {} as Record<keyof T, boolean>,
    );

    setFormTouched(allTouched);

    // Validate all fields
    const validation: ValidationResult = validateForm();
    setFormErrors(validation.errors as Record<keyof T, string>);
  }, [fields, validateForm]);

  // Set a field error
  const setError: (name: keyof T, error: string) => void = useCallback((name: keyof T, error: string): void => {
    setFormErrors((prev: Record<keyof T, string>) => ({
      ...prev,
      [name]: error,
    }));
  }, []);

  // Set multiple field errors
  const setErrors: (newErrors: Partial<Record<keyof T, string>>) => void = useCallback(
    (newErrors: Partial<Record<keyof T, string>>): void => {
      setFormErrors((prev: Record<keyof T, string>) => ({
        ...prev,
        ...newErrors,
      }));
    },
    [],
  );

  // Clear all errors
  const clearErrors: () => void = useCallback((): void => {
    setFormErrors({} as Record<keyof T, string>);
  }, []);

  // Reset form to initial state
  const reset: () => void = useCallback((): void => {
    setFormValues({...initialValues} as T);
    setFormTouched({} as Record<keyof T, boolean>);
    setFormErrors({} as Record<keyof T, string>);
    setIsSubmitted(false);
  }, [initialValues]);

  // Handle form submission
  const handleSubmit: (onSubmit: (values: T) => void | Promise<void>) => (e?: FormEvent) => Promise<void> = useCallback(
    (onSubmit: (values: T) => void | Promise<void>) =>
      async (e?: FormEvent): Promise<void> => {
        if (e) {
          e.preventDefault();
        }

        setIsSubmitted(true);
        touchAllFields();

        const validation: ValidationResult = validateForm();

        if (validation.isValid) {
          await onSubmit(values);
        }
      },
    [values, touchAllFields, validateForm],
  );

  // Get field props for easy integration
  const getFieldProps: (name: keyof T) => any = useCallback(
    (name: keyof T) => {
      const fieldConfig: FormField | undefined = getFieldConfig(name);

      return {
        error: touched[name] ? errors[name] : undefined,
        name,
        onBlur: (): void => setTouched(name, true),
        onChange: (value: string): void => setValue(name, value),
        required: fieldConfig?.required || false,
        touched: touched[name] || false,
        value: values[name] || '',
      };
    },
    [values, errors, touched, setValue, setTouched, getFieldConfig],
  );

  return {
    clearErrors,
    errors,
    getFieldProps,
    handleSubmit,
    isSubmitted,
    isValid,
    reset,
    setError,
    setErrors,
    setTouched,
    setTouchedFields,
    setValue,
    setValues,
    touchAllFields,
    touched,
    validateField,
    validateForm,
    values,
  };
};

export default useForm;
