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

import {Box, ButtonBase, useTheme} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import useIsDarkMode from '../../hooks/useIsDarkMode';

interface BlogCategoryFilterProps {
  categories: string[];
  active: string;
  onChange: (value: string) => void;
}

export default function BlogCategoryFilter({categories, active, onChange}: BlogCategoryFilterProps): JSX.Element {
  const theme = useTheme();
  const isLight = !useIsDarkMode();
  const options = ['All', ...categories];

  return (
    <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 0.75}}>
      {options.map((option) => {
        const isActive = active === option;
        return (
          <ButtonBase
            key={option}
            aria-pressed={isActive}
            onClick={() => onChange(option)}
            sx={{
              px: 1.75,
              py: 0.75,
              borderRadius: '999px',
              fontSize: '12.5px',
              fontWeight: isActive ? 600 : 500,
              border: '1px solid',
              userSelect: 'none',
              transition: 'all 0.15s ease',
              borderColor: isActive ? 'rgba(54,136,255,0.5)' : isLight ? 'rgba(0,0,0,0.1)' : 'rgba(255,255,255,0.1)',
              bgcolor: isActive ? 'rgba(54,136,255,0.12)' : 'transparent',
              color: isActive ? theme.vars?.palette.primary.main : isLight ? 'rgba(0,0,0,0.5)' : 'rgba(255,255,255,0.5)',
            }}
          >
            {option}
          </ButtonBase>
        );
      })}
    </Box>
  );
}
