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

import {Chip, Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {AVAILABLE_CONNECTION_CATEGORIES} from '../config/connectionVendorMeta';
import type {ConnectionCategory} from '../models/connection';

export type CategoryFilterValue = ConnectionCategory | 'all';

interface ConnectionCategoryFiltersProps {
  selected: CategoryFilterValue;
  onSelect: (value: CategoryFilterValue) => void;
}

export default function ConnectionCategoryFilters({selected, onSelect}: ConnectionCategoryFiltersProps): JSX.Element {
  const {t} = useTranslation('connections');

  const values: CategoryFilterValue[] = ['all', ...AVAILABLE_CONNECTION_CATEGORIES];

  return (
    <Stack
      direction="row"
      spacing={1}
      alignItems="center"
      flexWrap="wrap"
      useFlexGap
      data-testid="connection-category-filters"
    >
      {values.map((value) => (
        <Chip
          key={value}
          label={t(`categories.${value}`)}
          color={selected === value ? 'primary' : 'default'}
          variant={selected === value ? 'filled' : 'outlined'}
          onClick={() => onSelect(value)}
          sx={{borderRadius: '20px', cursor: 'pointer'}}
        />
      ))}
    </Stack>
  );
}
