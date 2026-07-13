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

import {useGetUserType, useGetUserTypes} from '@thunderid/configure-user-types';
import {
  Autocomplete,
  Box,
  Button,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  MenuItem,
  Select,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {Plus, Share2, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useEffect, useMemo, useRef, useState} from 'react';
import {useTranslation} from 'react-i18next';
import type {AttributeConfiguration} from '../models/connection';
import {
  type AttributeMappingFormState,
  flattenUserTypeAttributes,
  fromAttributeConfiguration,
  toAttributeConfiguration,
} from '../utils/attributeConfiguration';

interface MappingRow {
  key: number;
  externalAttribute: string;
  localAttribute: string;
}

interface AttributeMappingSectionProps {
  initialConfig?: AttributeConfiguration;
  onChange: (config: AttributeConfiguration | undefined, valid: boolean) => void;
}

export default function AttributeMappingSection({
  initialConfig = undefined,
  onChange,
}: AttributeMappingSectionProps): JSX.Element {
  const {t} = useTranslation('connections');

  const [initialState] = useState<AttributeMappingFormState>(() => fromAttributeConfiguration(initialConfig));
  const keyCounter = useRef(initialState.rows.length);
  const [userType, setUserType] = useState<string>(initialState.userType);
  const [rows, setRows] = useState<MappingRow[]>(() => initialState.rows.map((row, index) => ({key: index, ...row})));

  const userTypesQuery = useGetUserTypes();
  const userTypeList = useMemo(() => userTypesQuery.data?.types ?? [], [userTypesQuery.data]);
  const selectedUserTypeId: string | undefined = userTypeList.find((type) => type.name === userType)?.id;
  const userTypeDetail = useGetUserType(selectedUserTypeId);

  const localAttributeOptions: string[] = useMemo(
    () => flattenUserTypeAttributes(userTypeDetail.data?.schema),
    [userTypeDetail.data],
  );

  const hasContentRows: boolean = rows.some(
    (row) => row.externalAttribute.trim() !== '' || row.localAttribute.trim() !== '',
  );
  const userTypeMissing: boolean = userType === '' && hasContentRows;
  const valid = !userTypeMissing;

  useEffect(() => {
    const config = toAttributeConfiguration({
      userType,
      rows: rows.map((row) => ({externalAttribute: row.externalAttribute, localAttribute: row.localAttribute})),
    });
    onChange(config, valid);
  }, [userType, rows, valid, onChange]);

  const addRow = (): void => {
    const key = keyCounter.current;
    keyCounter.current += 1;
    setRows((prev) => [...prev, {key, externalAttribute: '', localAttribute: ''}]);
  };
  const removeRow = (key: number): void => {
    setRows((prev) => prev.filter((row) => row.key !== key));
  };
  const updateRow = (key: number, patch: Partial<MappingRow>): void => {
    setRows((prev) => prev.map((row) => (row.key === key ? {...row, ...patch} : row)));
  };

  // Ensure the stored user type is selectable even if it is not in the fetched list.
  const userTypeNames: string[] = useMemo(() => {
    const names = userTypeList.map((type) => type.name);
    return userType && !names.includes(userType) ? [userType, ...names] : names;
  }, [userTypeList, userType]);

  return (
    <Stack direction="column" spacing={2.5} data-testid="attribute-mapping-section">
      <FormControl fullWidth error={userTypeMissing}>
        <FormLabel htmlFor="attribute-mapping-user-type">{t('attributeMapping.userType.label')}</FormLabel>
        <Select
          id="attribute-mapping-user-type"
          displayEmpty
          value={userType}
          onChange={(e) => setUserType(e.target.value)}
          renderValue={(value) => (value ? value : t('attributeMapping.userType.placeholder'))}
          data-testid="attribute-mapping-user-type-select"
        >
          {userTypeNames.map((name) => (
            <MenuItem key={name} value={name}>
              {name}
            </MenuItem>
          ))}
        </Select>
        <FormHelperText>
          {userTypeMissing ? t('attributeMapping.userTypeRequired') : t('attributeMapping.userType.helper')}
        </FormHelperText>
      </FormControl>

      <Stack direction="column" spacing={1.5}>
        {rows.length === 0 && (
          <Box
            sx={{border: '1px dashed', borderColor: 'divider', borderRadius: 2, p: 4, textAlign: 'center'}}
            data-testid="attribute-mapping-empty"
          >
            <Stack direction="column" spacing={1} alignItems="center">
              <Box
                sx={{
                  width: 44,
                  height: 44,
                  borderRadius: 2,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  bgcolor: 'action.hover',
                }}
              >
                <Share2 size={18} />
              </Box>
              <Typography variant="subtitle2">{t('attributeMapping.empty.title')}</Typography>
              <Typography variant="body2" color="text.secondary" sx={{maxWidth: 420}}>
                {t('attributeMapping.empty.description')}
              </Typography>
            </Stack>
          </Box>
        )}
        {rows.map((row) => (
          <Stack key={row.key} direction="row" spacing={1.5} alignItems="flex-end">
            <FormControl fullWidth>
              <FormLabel htmlFor={`attribute-mapping-external-${row.key}`}>
                {t('attributeMapping.externalAttribute.label')}
              </FormLabel>
              <TextField
                id={`attribute-mapping-external-${row.key}`}
                fullWidth
                placeholder={t('attributeMapping.externalAttribute.placeholder')}
                value={row.externalAttribute}
                onChange={(e) => updateRow(row.key, {externalAttribute: e.target.value})}
              />
            </FormControl>
            <FormControl fullWidth>
              <FormLabel>{t('attributeMapping.localAttribute.label')}</FormLabel>
              <Autocomplete
                fullWidth
                freeSolo
                options={localAttributeOptions}
                inputValue={row.localAttribute}
                onInputChange={(_event, value) => updateRow(row.key, {localAttribute: value})}
                renderInput={(params) => (
                  <TextField {...params} placeholder={t('attributeMapping.localAttribute.placeholder')} />
                )}
              />
            </FormControl>
            <IconButton
              onClick={() => removeRow(row.key)}
              aria-label="remove attribute mapping"
              data-testid={`attribute-mapping-remove-${row.key}`}
            >
              <Trash2 size={16} />
            </IconButton>
          </Stack>
        ))}
        <Box>
          <Button
            variant="outlined"
            size="small"
            startIcon={<Plus size={16} />}
            onClick={addRow}
            data-testid="attribute-mapping-add"
          >
            {t('attributeMapping.add')}
          </Button>
        </Box>
      </Stack>
    </Stack>
  );
}
