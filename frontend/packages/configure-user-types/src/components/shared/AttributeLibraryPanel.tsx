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

import {Box, FormLabel, IconButton, Paper, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useMemo, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ATTRIBUTES from '../../constants/attributes';
import type {LibraryAttribute} from '../../types/user-types';

/**
 * Props for the {@link AttributeLibraryPanel} component.
 *
 * @public
 */
export interface AttributeLibraryPanelProps {
  /** Property names already present in the schema; matching attributes are hidden. */
  existingNames: string[];
  /** Called when the admin picks an attribute to add to the schema. */
  onAdd: (attribute: LibraryAttribute) => void;
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
    return ATTRIBUTES.filter((attr) => !existing.has(attr.name));
  }, [existingNames]);

  const filteredAttributes = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) {
      return availableAttributes;
    }
    return availableAttributes.filter(
      (attr) => attr.name.toLowerCase().includes(term) || attr.displayName.toLowerCase().includes(term),
    );
  }, [availableAttributes, search]);

  const title = t('userTypes:attributes.libraryTitle', 'Available Properties');

  return (
    <Paper
      variant="outlined"
      component="section"
      role="region"
      aria-label={title}
      sx={{p: 2, borderRadius: 2, position: {md: 'sticky'}, top: {md: 16}}}
    >
      <Stack spacing={1.5}>
        <FormLabel>{title}</FormLabel>

        <TextField
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t('userTypes:attributes.searchPlaceholder', 'Search properties')}
          size="small"
          fullWidth
          disabled={disabled}
        />

        {availableAttributes.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            {t('userTypes:attributes.allAdded', 'All available properties have been added.')}
          </Typography>
        ) : filteredAttributes.length === 0 ? (
          <Typography variant="body2" color="text.secondary">
            {t('userTypes:attributes.noResults', 'No properties match your search.')}
          </Typography>
        ) : (
          <Stack
            spacing={0.5}
            sx={{
              maxHeight: {xs: 280, md: 'min(520px, calc(100vh - 300px))'},
              overflowY: 'auto',
              pr: 0.5,
            }}
          >
            {filteredAttributes.map((attr) => (
              <Box
                key={attr.name}
                sx={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: 1,
                  px: 1.5,
                  py: 1,
                  borderRadius: 1,
                  transition: 'background-color 0.2s ease-in-out',
                  '&:hover': {backgroundColor: 'action.hover'},
                }}
              >
                <Typography
                  variant="body2"
                  fontWeight={500}
                  title={attr.displayName || attr.name}
                  sx={{
                    flex: 1,
                    minWidth: 0,
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                    color: 'text.primary',
                  }}
                >
                  {attr.displayName || attr.name}
                </Typography>
                <IconButton
                  size="small"
                  disabled={disabled}
                  onClick={() => onAdd(attr)}
                  aria-label={attr.displayName || attr.name}
                  sx={{
                    flexShrink: 0,
                    height: 28,
                    width: 28,
                    border: '1px solid',
                    borderColor: 'divider',
                    borderRadius: 1,
                    backgroundColor: 'action.selected',
                    '&:hover': {
                      backgroundColor: 'primary.main',
                      borderColor: 'primary.main',
                      color: 'primary.contrastText',
                    },
                    '&.Mui-disabled': {
                      backgroundColor: 'action.disabledBackground',
                      borderColor: 'divider',
                    },
                  }}
                >
                  <Plus size={14} />
                </IconButton>
              </Box>
            ))}
          </Stack>
        )}
      </Stack>
    </Paper>
  );
}
