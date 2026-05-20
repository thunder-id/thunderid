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

import {EmbeddedFlowComponent, EmbeddedFlowComponentType} from '@thunderid/browser';
import {ReactElement} from 'react';
import {AdapterProps} from '../../../../../models/adapters';
import CheckboxInput from '../../../../adapters/CheckboxInput';
import DateInput from '../../../../adapters/DateInput';
import DividerComponent from '../../../../adapters/DividerComponent';
import EmailInput from '../../../../adapters/EmailInput';
import FacebookButton from '../../../../adapters/FacebookButton';
// eslint-disable-next-line import/no-cycle
import FormContainer from '../../../../adapters/FormContainer';
import GitHubButton from '../../../../adapters/GitHubButton';
import GoogleButton from '../../../../adapters/GoogleButton';
import ImageComponent from '../../../../adapters/ImageComponent';
import LinkedInButton from '../../../../adapters/LinkedInButton';
import MicrosoftButton from '../../../../adapters/MicrosoftButton';
import NumberInput from '../../../../adapters/NumberInput';
import PasswordInput from '../../../../adapters/PasswordInput';
import SelectInput from '../../../../adapters/SelectInput';
import SignInWithEthereumButton from '../../../../adapters/SignInWithEthereumButton';
import ButtonComponent from '../../../../adapters/SubmitButton';
import TelephoneInput from '../../../../adapters/TelephoneInput';
import TextInput from '../../../../adapters/TextInput';
import Typography from '../../../../adapters/Typography';

/**
 * Creates the appropriate recovery component based on the component type.
 */
export const createRecoveryComponent = ({component, onSubmit, ...rest}: AdapterProps): ReactElement => {
  switch (component.type) {
    case EmbeddedFlowComponentType.Typography:
      return <Typography component={component} onSubmit={onSubmit} {...rest} />;

    case EmbeddedFlowComponentType.Input: {
      // Determine input type based on variant or config
      const inputVariant: string = component.variant?.toUpperCase();
      const inputType: string = (component.config['type'] as string)?.toLowerCase();

      if (inputVariant === 'EMAIL' || inputType === 'email') {
        return <EmailInput component={component} onSubmit={onSubmit} {...rest} />;
      }

      if (inputVariant === 'PASSWORD' || inputType === 'password') {
        return <PasswordInput component={component} onSubmit={onSubmit} {...rest} />;
      }

      if (inputVariant === 'TELEPHONE' || inputType === 'tel') {
        return <TelephoneInput component={component} onSubmit={onSubmit} {...rest} />;
      }

      if (inputVariant === 'NUMBER' || inputType === 'number') {
        return <NumberInput component={component} onSubmit={onSubmit} {...rest} />;
      }

      if (inputVariant === 'DATE' || inputType === 'date') {
        return <DateInput component={component} onSubmit={onSubmit} {...rest} />;
      }

      if (inputVariant === 'CHECKBOX' || inputType === 'checkbox') {
        return <CheckboxInput component={component} onSubmit={onSubmit} {...rest} />;
      }

      return <TextInput component={component} onSubmit={onSubmit} {...rest} />;
    }

    case EmbeddedFlowComponentType.Button: {
      const buttonVariant: string | undefined = component.variant?.toUpperCase();
      const buttonText: string = (component.config['text'] as string) || (component.config['label'] as string) || '';

      // TODO: The connection type should come as metadata.
      if (buttonVariant === 'SOCIAL') {
        if (buttonText.toLowerCase().includes('google')) {
          return (
            <GoogleButton onClick={(): any => onSubmit(component, {})} {...rest}>
              {buttonText}
            </GoogleButton>
          );
        }

        if (buttonText.toLowerCase().includes('github')) {
          return (
            <GitHubButton onClick={(): any => onSubmit(component, {})} {...rest}>
              {buttonText}
            </GitHubButton>
          );
        }

        if (buttonText.toLowerCase().includes('microsoft')) {
          return (
            <MicrosoftButton onClick={(): any => onSubmit(component, {})} {...rest}>
              {buttonText}
            </MicrosoftButton>
          );
        }

        if (buttonText.toLowerCase().includes('facebook')) {
          return (
            <FacebookButton onClick={(): any => onSubmit(component, {})} {...rest}>
              {buttonText}
            </FacebookButton>
          );
        }

        if (buttonText.toLowerCase().includes('linkedin')) {
          return (
            <LinkedInButton onClick={(): any => onSubmit(component, {})} {...rest}>
              {buttonText}
            </LinkedInButton>
          );
        }

        if (buttonText.toLowerCase().includes('ethereum')) {
          return (
            <SignInWithEthereumButton onClick={(): any => onSubmit(component, {})} {...rest}>
              {buttonText}
            </SignInWithEthereumButton>
          );
        }
      }

      // Use the generic ButtonComponent for all other button variants
      // It will handle PRIMARY, SECONDARY, TEXT, SOCIAL mappings internally
      return <ButtonComponent component={component} onSubmit={onSubmit} {...rest} />;
    }

    case EmbeddedFlowComponentType.Form:
      return <FormContainer component={component} onSubmit={onSubmit} {...rest} />;

    case EmbeddedFlowComponentType.Select:
      return <SelectInput component={component} onSubmit={onSubmit} {...rest} />;

    case EmbeddedFlowComponentType.Divider:
      return <DividerComponent component={component} onSubmit={onSubmit} {...rest} />;

    case EmbeddedFlowComponentType.Image:
      return <ImageComponent component={component} onSubmit={onSubmit} {...rest} />;

    default:
      return <div />;
  }
};

/**
 * Convenience function that creates the appropriate recovery component from flow component data.
 */
export const createRecoveryOptionFromComponent = (
  component: EmbeddedFlowComponent,
  formValues: Record<string, string>,
  touchedFields: Record<string, boolean>,
  formErrors: Record<string, string>,
  isLoading: boolean,
  isFormValid: boolean,
  onInputChange: (name: string, value: string) => void,
  options?: {
    buttonClassName?: string;
    inputClassName?: string;
    key?: string | number;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>) => void;
    size?: 'small' | 'medium' | 'large';
    variant?: any;
  },
): ReactElement =>
  createRecoveryComponent({
    component,
    formErrors,
    formValues,
    isFormValid,
    isLoading,
    onInputChange,
    touchedFields,
    ...options,
  });

/**
 * Processes an array of components and renders them as React elements for recovery flow.
 */
export const renderRecoveryComponents = (
  components: EmbeddedFlowComponent[],
  formValues: Record<string, string>,
  touchedFields: Record<string, boolean>,
  formErrors: Record<string, string>,
  isLoading: boolean,
  isFormValid: boolean,
  onInputChange: (name: string, value: string) => void,
  options?: {
    buttonClassName?: string;
    inputClassName?: string;
    onSubmit?: (component: EmbeddedFlowComponent, data?: Record<string, any>) => void;
    size?: 'small' | 'medium' | 'large';
    variant?: any;
  },
): ReactElement[] =>
  components
    .map((component: any, index: any) =>
      createRecoveryOptionFromComponent(
        component,
        formValues,
        touchedFields,
        formErrors,
        isLoading,
        isFormValid,
        onInputChange,
        {
          ...options,
          // Use component id as key, fallback to index
          key: component.id || index,
        },
      ),
    )
    .filter(Boolean);
