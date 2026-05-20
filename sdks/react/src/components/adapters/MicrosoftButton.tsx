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

import {WithPreferences} from '@thunderid/browser';
import {FC, HTMLAttributes} from 'react';
import useTranslation from '../../hooks/useTranslation';
import Button from '../primitives/Button/Button';

export interface MicrosoftButtonProps extends WithPreferences {
  /**
   * Whether the component is in loading state.
   */
  isLoading?: boolean;
}

/**
 * Microsoft Sign-In Button Component.
 * Handles authentication with Microsoft identity provider.
 */
const MicrosoftButton: FC<MicrosoftButtonProps & HTMLAttributes<HTMLButtonElement>> = ({
  isLoading,
  preferences,
  children,
  ...rest
}: MicrosoftButtonProps & HTMLAttributes<HTMLButtonElement>) => {
  const {t} = useTranslation(preferences?.i18n);

  return (
    <Button
      {...rest}
      fullWidth
      type="button"
      color="secondary"
      variant="solid"
      disabled={isLoading}
      startIcon={
        <svg width="14" height="14" viewBox="0 0 23 23" xmlns="http://www.w3.org/2000/svg">
          <path fill="#f3f3f3" d="M0 0h23v23H0z" />
          <path fill="#f35325" d="M1 1h10v10H1z" />
          <path fill="#81bc06" d="M12 1h10v10H12z" />
          <path fill="#05a6f0" d="M1 12h10v10H1z" />
          <path fill="#ffba08" d="M12 12h10v10H12z" />
        </svg>
      }
    >
      {children ?? t('elements.buttons.microsoft.text')}
    </Button>
  );
};

export default MicrosoftButton;
