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

export interface GoogleButtonProps extends WithPreferences {
  /**
   * Whether the component is in loading state.
   */
  isLoading?: boolean;
}

/**
 * Google Sign-In Button Component.
 * Handles authentication with Google identity provider.
 */
const GoogleButton: FC<GoogleButtonProps & HTMLAttributes<HTMLButtonElement>> = ({
  isLoading,
  preferences,
  children,
  ...rest
}: GoogleButtonProps & HTMLAttributes<HTMLButtonElement>) => {
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
        <svg width="18" height="18" viewBox="0 0 67.91 67.901" xmlns="http://www.w3.org/2000/svg">
          <g transform="translate(-0.001 -0.001)">
            <path
              d="M15.049,160.965l-2.364,8.824-8.639.183a34.011,34.011,0,0,1-.25-31.7h0l7.691,1.41,3.369,7.645a20.262,20.262,0,0,0,.19,13.642Z"
              transform="translate(0 -119.93)"
              fill="#fbbb00"
            />
            <path
              d="M294.24,208.176A33.939,33.939,0,0,1,282.137,241h0l-9.687-.494-1.371-8.559a20.235,20.235,0,0,0,8.706-10.333H261.628V208.176Z"
              transform="translate(-226.93 -180.567)"
              fill="#518ef8"
            />
            <path
              d="M81.668,328.8h0a33.962,33.962,0,0,1-51.161-10.387l11-9.006a20.192,20.192,0,0,0,29.1,10.338Z"
              transform="translate(-26.463 -268.374)"
              fill="#28b446"
            />
            <path
              d="M80.451,7.816l-11,9A20.19,20.19,0,0,0,39.686,27.393l-11.06-9.055h0A33.959,33.959,0,0,1,80.451,7.816Z"
              transform="translate(-24.828)"
              fill="#f14336"
            />
          </g>
        </svg>
      }
    >
      {children ?? t('elements.buttons.google.text')}
    </Button>
  );
};

export default GoogleButton;
