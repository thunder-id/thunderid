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

import {FC, FormEvent} from 'react';
import {AdapterProps} from '../../models/adapters';
// eslint-disable-next-line import/no-cycle
import {createSignUpComponent} from '../presentation/auth/SignUp/v1/SignUpOptionFactory';

/**
 * Form container component that renders child components.
 */
const FormContainer: FC<AdapterProps> = (props: AdapterProps) => {
  const {component} = props;

  // If the form has child components, render them wrapped in a form element
  if (component.components && component.components.length > 0) {
    const handleFormSubmit: (e: FormEvent) => void = (e: FormEvent): void => {
      e.preventDefault();

      // Find submit button in child components and trigger its submission
      const submitButton: any = component.components?.find(
        (child: any) =>
          child.type === 'BUTTON' &&
          (child.variant === 'PRIMARY' || child.variant === 'SECONDARY' || child.config?.type === 'submit'),
      );

      if (submitButton && props.onSubmit) {
        props.onSubmit(submitButton, props.formValues);
      }
    };

    return (
      <form key={component.id} onSubmit={handleFormSubmit} style={{display: 'flex', flexDirection: 'column'}}>
        {component.components.map((childComponent: any) =>
          createSignUpComponent({
            ...props,
            component: childComponent,
          }),
        )}
      </form>
    );
  }

  // Empty form container
  return <div key={component.id} />;
};

export default FormContainer;
