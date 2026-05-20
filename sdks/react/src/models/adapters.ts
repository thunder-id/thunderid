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

import {EmbeddedFlowComponent, WithPreferences} from '@thunderid/browser';

/**
 * Props shared by all adapter components.
 */
export interface AdapterProps extends WithPreferences {
  /**
   * Custom CSS class name for buttons.
   */
  buttonClassName?: string;

  /**
   * The component configuration from the flow response.
   */
  component: EmbeddedFlowComponent;

  /**
   * Form validation errors.
   */
  formErrors: Record<string, string>;

  /**
   * Current form values.
   */
  formValues: Record<string, string>;

  /**
   * Custom CSS class name for form inputs.
   */
  inputClassName?: string;

  /**
   * Whether the form is valid.
   */
  isFormValid: boolean;

  /**
   * Whether the component is in loading state.
   */
  isLoading: boolean;

  /**
   * Callback function called when input values change.
   */
  onInputChange: (name: string, value: string) => void;

  onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>) => void;

  /**
   * Component size variant.
   */
  size?: 'small' | 'medium' | 'large';

  /**
   * Touched state for form fields.
   */
  touchedFields: Record<string, boolean>;

  /**
   * Component theme variant.
   */
  variant?: any;
}
