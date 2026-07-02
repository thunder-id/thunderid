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

import {Box, Button, FormLabel, Paper, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ATTRIBUTES from '../../constants/attributes';
import type {Attribute} from '../../types/user-types';

/**
 * Props for the {@link AttributeLibraryPanel} component.
 *
 * @public
 */
export interface AttributeLibraryPanelProps {
  /** Property names already present in the schema; matching attributes are hidden. */
  existingNames: string[];
  /** Called when the admin picks an attribute to add to the schema. */
  onAdd: (attribute: Attribute) => void;
  disabled?: boolean;
}

/**
 * Side panel listing the static, front-end-only library of predefined attributes.
 * Clicking an attribute adds it to the schema; attributes already in the schema
 * are hidden.
 *
 * @public
 */
export default function AttributeLibraryPanel({
  existingNames,
  onAdd,
  disabled = false,
}: AttributeLibraryPanelProps): JSX.Element {
  const {t} = useTranslation();

  const [search, setSearch] = useState('');

  const availableAttributes = useMemo(() => {
    const existing = new Set(existingNames);
    return ATTRIBUTES.filter((attr) => !existing.has(attr.id));
  }, [existingNames]);

  const filteredAttributes = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) {
      return availableAttributes;
    }
    return availableAttributes.filter(
      (attr) => attr.id.toLowerCase().includes(term) || (attr.displayName ?? '').toLowerCase().includes(term),
    );
  }, [availableAttributes, search]);

  return (
    <Paper
      variant="outlined"
      component="section"
      role="region"
      aria-label={t('userTypes:attributes.libraryTitle', 'Attributes')}
      sx={{p: 2, borderRadius: 2, position: {md: 'sticky'}, top: {md: 16}}}
    >
      <Stack spacing={1.5}>
        <FormLabel>{t('userTypes:attributes.libraryTitle', 'Attributes')}</FormLabel>

        <TextField
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t('userTypes:attributes.searchPlaceholder', 'Search attributes')}
          size="small"
          fullWidth
          disabled={disabled}
        />

        {availableAttributes.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            {t('userTypes:attributes.allAdded', 'All available attributes have been added.')}
          </Typography>
        ) : filteredAttributes.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            {t('userTypes:attributes.noResults', 'No attributes match your search.')}
          </Typography>
        ) : (
          <Box sx={{maxHeight: 420, overflowY: 'auto', pr: 0.5}}>
            {filteredAttributes.map((attr) => (
              <Button
                key={attr.id}
                variant="text"
                fullWidth
                disabled={disabled}
                onClick={() => onAdd(attr)}
                startIcon={<Plus size={16} />}
                sx={{justifyContent: 'flex-start', textTransform: 'none', px: 1}}
              >
                <Typography variant="body2" noWrap sx={{flex: 1, textAlign: 'left'}}>
                  {attr.displayName ?? attr.id}
                </Typography>
              </Button>
            ))}
          </Box>
        )}
      </Stack>
    </Paper>
  );
}
