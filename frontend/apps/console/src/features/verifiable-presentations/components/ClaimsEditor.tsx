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

import {
  Box,
  Button,
  Chip,
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  MenuItem,
  Paper,
  Select,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX, type KeyboardEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {emptyClaimRow, type ClaimRequirement, type ClaimRow} from '../models/claims';

export interface ClaimsEditorProps {
  claims: ClaimRow[];
  onChange: (claims: ClaimRow[]) => void;
}

/**
 * Unified per-claim editor: one row per requested claim, with its requirement
 * (mandatory/optional), an optional value constraint, and a subject-derivation
 * toggle — instead of separate comma lists for each.
 */
export default function ClaimsEditor({claims, onChange}: ClaimsEditorProps): JSX.Element {
  const {t} = useTranslation('verifiable-presentations');
  const [valueInput, setValueInput] = useState<Record<string, string>>({});

  const update = (id: string, patch: Partial<ClaimRow>): void =>
    onChange(claims.map((c) => (c.id === id ? {...c, ...patch} : c)));
  const add = (): void => onChange([...claims, emptyClaimRow()]);
  const remove = (id: string): void => onChange(claims.filter((c) => c.id !== id));

  const addValue = (claim: ClaimRow): void => {
    const value = (valueInput[claim.id] ?? '').trim();
    if (value === '' || claim.values.includes(value)) {
      return;
    }
    update(claim.id, {values: [...claim.values, value]});
    setValueInput((prev) => ({...prev, [claim.id]: ''}));
  };
  const removeValue = (claim: ClaimRow, value: string): void =>
    update(claim.id, {values: claim.values.filter((v) => v !== value)});

  return (
    <Stack spacing={2}>
      {claims.length === 0 && (
        <Typography variant="body2" color="text.secondary">
          {t('claims.empty')}
        </Typography>
      )}

      {claims.map((claim) => (
        <Paper
          key={claim.id}
          variant="outlined"
          sx={{
            px: 3,
            py: 3,
            borderRadius: 2,
            position: 'relative',
            transition: 'border-color 0.2s',
            '&:hover': {borderColor: 'primary.main'},
            '&:hover .claim-delete-btn': {opacity: 1},
          }}
        >
          <Tooltip title={t('claims.remove')}>
            <IconButton
              className="claim-delete-btn"
              size="small"
              color="error"
              onClick={(): void => remove(claim.id)}
              sx={{position: 'absolute', top: 8, right: 8, opacity: 0, transition: 'opacity 0.2s'}}
            >
              <Trash2 size={16} />
            </IconButton>
          </Tooltip>

          <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', sm: '1fr 220px'}, gap: 2}}>
            <FormControl>
              <FormLabel>{t('claims.name')}</FormLabel>
              <TextField
                size="small"
                value={claim.name}
                placeholder="given_name"
                onChange={(e): void => update(claim.id, {name: e.target.value})}
              />
            </FormControl>
            <FormControl>
              <FormLabel>{t('claims.requirement')}</FormLabel>
              <Select
                size="small"
                value={claim.requirement}
                onChange={(e): void => update(claim.id, {requirement: e.target.value as ClaimRequirement})}
              >
                <MenuItem value="mandatory">{t('claims.mandatory')}</MenuItem>
                <MenuItem value="optional">{t('claims.optional')}</MenuItem>
              </Select>
            </FormControl>
          </Box>
          <FormHelperText>{t('claims.nameHint')}</FormHelperText>

          <FormControl fullWidth sx={{mt: 2}}>
            <FormLabel>{t('claims.values')}</FormLabel>
            <Box sx={{display: 'flex', gap: 1}}>
              <TextField
                size="small"
                fullWidth
                value={valueInput[claim.id] ?? ''}
                placeholder={t('claims.valuesPlaceholder')}
                onChange={(e): void => setValueInput((prev) => ({...prev, [claim.id]: e.target.value}))}
                onKeyDown={(e: KeyboardEvent): void => {
                  if (e.key === 'Enter') {
                    e.preventDefault();
                    addValue(claim);
                  }
                }}
              />
              <Button variant="outlined" onClick={(): void => addValue(claim)}>
                {t('common:actions.add')}
              </Button>
            </Box>
            {claim.values.length > 0 && (
              <Box sx={{mt: 1.5, display: 'flex', flexWrap: 'wrap', gap: 1}}>
                {claim.values.map((value) => (
                  <Chip key={value} label={value} size="small" onDelete={(): void => removeValue(claim, value)} />
                ))}
              </Box>
            )}
            <FormHelperText>{t('claims.valuesHint')}</FormHelperText>
          </FormControl>
        </Paper>
      ))}

      <Button
        variant="outlined"
        startIcon={<Plus size={16} />}
        onClick={add}
        fullWidth
        sx={{py: 1.5, borderStyle: 'dashed', '&:hover': {borderStyle: 'dashed'}}}
      >
        {t('claims.add')}
      </Button>
    </Stack>
  );
}
