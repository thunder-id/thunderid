// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, IconButton, Typography} from '@wso2/oxygen-ui';
import {Check, Copy, Eye, EyeOff} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState} from 'react';

export interface FormField {
  label: string;
  value: string;
  isPassword?: boolean;
  highlight?: boolean;
  readOnly?: boolean;
}

export default function FormFieldsBlock({fields}: {fields: FormField[]}): JSX.Element {
  const [copiedLabel, setCopiedLabel] = useState<string | null>(null);
  const [visiblePasswords, setVisiblePasswords] = useState<Set<string>>(new Set());

  const handleCopy = (label: string, value: string): void => {
    void navigator.clipboard.writeText(value).then(() => {
      setCopiedLabel(label);
      setTimeout(() => setCopiedLabel(null), 2000);
    });
  };

  const togglePasswordVisibility = (label: string): void => {
    setVisiblePasswords((prev) => {
      const next = new Set(prev);
      if (next.has(label)) {
        next.delete(label);
      } else {
        next.add(label);
      }
      return next;
    });
  };

  return (
    <Box sx={{border: '1px solid', borderColor: 'divider', borderRadius: 1.5, overflow: 'hidden'}}>
      {fields.map((f, i) => {
        const isVisible = !f.isPassword || visiblePasswords.has(f.label);
        return (
          <Box
            key={f.label}
            sx={{
              display: 'flex',
              alignItems: 'center',
              gap: 2,
              px: 2,
              py: f.highlight ? 1 : 0.75,
              borderTop: i === 0 ? 'none' : '1px solid',
              borderColor: 'divider',
              bgcolor: f.highlight ? 'action.selected' : 'transparent',
            }}
          >
            <Typography variant="caption" color="text.secondary" sx={{minWidth: 100, flexShrink: 0}}>
              {f.label}
            </Typography>
            {f.highlight ? (
              <Box
                sx={{
                  flex: 1,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 1,
                  px: 1.25,
                  py: 0.5,
                  borderRadius: 1,
                  bgcolor: 'background.paper',
                  border: '1px solid',
                  borderColor: 'primary.light',
                  minWidth: 0,
                }}
              >
                <Typography
                  variant="body2"
                  fontFamily="monospace"
                  color="primary.main"
                  fontWeight={600}
                  sx={{flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap'}}
                >
                  {f.value}
                </Typography>
              </Box>
            ) : (
              <Typography variant="body2" fontFamily="monospace" sx={{flex: 1}}>
                {isVisible ? f.value : '••••••••'}
              </Typography>
            )}
            {!f.readOnly && (
              <Box sx={{display: 'flex', alignItems: 'center', gap: 0.5, flexShrink: 0}}>
                {f.isPassword && (
                  <IconButton
                    size="small"
                    aria-label={isVisible ? 'Hide password' : 'Show password'}
                    onClick={() => togglePasswordVisibility(f.label)}
                    sx={{color: 'text.secondary'}}
                  >
                    {isVisible ? <EyeOff size={13} /> : <Eye size={13} />}
                  </IconButton>
                )}
                <IconButton
                  size="small"
                  aria-label={`Copy ${f.label}`}
                  onClick={() => handleCopy(f.label, f.value)}
                  sx={{
                    color: copiedLabel === f.label ? 'success.main' : f.highlight ? 'primary.main' : 'text.secondary',
                  }}
                >
                  {copiedLabel === f.label ? <Check size={13} /> : <Copy size={13} />}
                </IconButton>
              </Box>
            )}
          </Box>
        );
      })}
    </Box>
  );
}
