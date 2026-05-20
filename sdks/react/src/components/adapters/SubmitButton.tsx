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

import {FC} from 'react';
import {AdapterProps} from '../../models/adapters';
import Button from '../primitives/Button/Button';
import Spinner from '../primitives/Spinner/Spinner';

/**
 * Button component for sign-up forms that handles all button variants.
 */
const ButtonComponent: FC<AdapterProps> = ({
  component,
  isLoading,
  isFormValid,
  buttonClassName,
  onSubmit,
  size = 'medium',
}: AdapterProps) => {
  const config: Record<string, unknown> = component.config || {};
  const buttonText: string = (config['text'] as string) || (config['label'] as string) || 'Continue';
  const buttonType: string = (config['type'] as string) || 'submit';
  const componentVariant: string = component.variant?.toUpperCase() || 'PRIMARY';

  // Map component variants to Button primitive props
  const getButtonProps = (): {color: 'primary' | 'secondary'; variant: 'solid' | 'text' | 'outline'} => {
    switch (componentVariant) {
      case 'PRIMARY':
        return {color: 'primary' as const, variant: 'solid' as const};
      case 'SECONDARY':
        return {color: 'secondary' as const, variant: 'solid' as const};
      case 'TEXT':
        return {color: 'primary' as const, variant: 'text' as const};
      case 'SOCIAL':
      case 'OUTLINED':
        return {color: 'primary' as const, variant: 'outline' as const};
      default:
        return {color: 'primary' as const, variant: 'solid' as const};
    }
  };

  const {variant, color} = getButtonProps();

  const handleClick = (): void => {
    if (onSubmit && buttonType !== 'submit') {
      onSubmit(component);
    }
  };

  return (
    <Button
      key={component.id}
      type={buttonType === 'submit' ? 'submit' : 'button'}
      variant={variant}
      color={color}
      size={size}
      disabled={isLoading || (buttonType === 'submit' && !isFormValid)}
      onClick={buttonType !== 'submit' ? handleClick : undefined}
      className={buttonClassName}
      style={{width: '100%'}}
    >
      {isLoading ? <Spinner size="small" /> : buttonText}
    </Button>
  );
};

export default ButtonComponent;
