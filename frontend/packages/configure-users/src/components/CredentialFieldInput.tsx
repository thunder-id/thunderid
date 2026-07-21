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

import {IconButton, InputAdornment, TextField} from '@wso2/oxygen-ui';
import {Eye, EyeClosed} from '@wso2/oxygen-ui-icons-react';
import {useState} from 'react';

interface CredentialFieldInputProps {
  id: string;
  value: string;
  placeholder: string;
  required: boolean;
  error: boolean;
  helperText?: string;
  color: 'error' | 'primary';
  onChange: (e: React.ChangeEvent<HTMLInputElement>) => void;
  onBlur?: () => void;
  inputRef: React.Ref<HTMLInputElement>;
  name: string;
  ariaLabel?: string;
}

function CredentialFieldInput({
  id,
  value,
  placeholder,
  required,
  error,
  helperText = undefined,
  color,
  onChange,
  onBlur = undefined,
  inputRef,
  name,
  ariaLabel = undefined,
}: CredentialFieldInputProps) {
  const [showPassword, setShowPassword] = useState(false);

  return (
    <TextField
      id={id}
      name={name}
      value={value}
      type={showPassword ? 'text' : 'password'}
      placeholder={placeholder}
      fullWidth
      required={required}
      variant="outlined"
      error={error}
      helperText={helperText}
      color={color}
      onChange={onChange}
      onBlur={onBlur}
      inputRef={inputRef}
      slotProps={{
        htmlInput: {'aria-label': ariaLabel},
        input: {
          endAdornment: (
            <InputAdornment position="end">
              <IconButton
                aria-label={showPassword ? 'hide password' : 'show password'}
                onClick={() => setShowPassword((prev) => !prev)}
                edge="end"
              >
                {showPassword ? <EyeClosed /> : <Eye />}
              </IconButton>
            </InputAdornment>
          ),
        },
      }}
    />
  );
}

export default CredentialFieldInput;
