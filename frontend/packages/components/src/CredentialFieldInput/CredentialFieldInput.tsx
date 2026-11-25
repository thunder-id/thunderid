// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
