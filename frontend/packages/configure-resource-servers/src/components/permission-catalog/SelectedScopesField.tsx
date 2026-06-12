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

import {FormControl, IconButton, InputAdornment, TextField, Tooltip} from '@wso2/oxygen-ui';
import {Check, Copy} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {ResourcePermissions} from '../../models/resource-server';

export interface SelectedScopesFieldProps {
  /** Permissions currently selected, grouped by resource server. */
  selected: ResourcePermissions[];
}

export default function SelectedScopesField({selected}: SelectedScopesFieldProps): JSX.Element {
  const {t} = useTranslation();
  const [copied, setCopied] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout>>(undefined);

  useEffect(() => () => clearTimeout(timerRef.current), []);

  const scopes = selected.flatMap((entry) => entry.permissions).join(' ');

  const handleCopy = (): void => {
    navigator.clipboard
      .writeText(scopes)
      .then(() => {
        setCopied(true);
        clearTimeout(timerRef.current);
        timerRef.current = setTimeout(() => setCopied(false), 1500);
      })
      .catch(() => {
        /* clipboard unavailable; no-op */
      });
  };

  return (
    <FormControl fullWidth>
      <TextField
        id="permission-catalog-scopes"
        size="small"
        value={scopes}
        placeholder={t('resourceServers:permissionCatalog.scopes.placeholder', 'No permissions selected')}
        InputProps={{
          readOnly: true,
          sx: {fontFamily: 'monospace'},
          endAdornment: (
            <InputAdornment position="end">
              <Tooltip
                title={copied ? t('resourceServers:permissionCatalog.scopes.copied', 'Copied') : ''}
                open={copied}
              >
                <span>
                  <IconButton
                    size="small"
                    disabled={scopes === ''}
                    onClick={handleCopy}
                    aria-label={t('resourceServers:permissionCatalog.scopes.copy', 'Copy scopes')}
                  >
                    {copied ? <Check size={16} /> : <Copy size={16} />}
                  </IconButton>
                </span>
              </Tooltip>
            </InputAdornment>
          ),
        }}
      />
    </FormControl>
  );
}
