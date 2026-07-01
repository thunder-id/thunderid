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
  FormControl,
  FormHelperText,
  FormLabel,
  IconButton,
  Paper,
  Stack,
  TextField,
  Tooltip,
  Typography,
} from '@wso2/oxygen-ui';
import {Plus, Trash2} from '@wso2/oxygen-ui-icons-react';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {emptyClaimRow, type ClaimRow} from '../models/claims';

export interface ClaimsEditorProps {
  claims: ClaimRow[];
  onChange: (claims: ClaimRow[]) => void;
}

/**
 * Per-claim editor for a credential configuration: one row per disclosed claim,
 * with the attribute name (the user-profile lookup key) and a wallet display name.
 */
export default function ClaimsEditor({claims, onChange}: ClaimsEditorProps): JSX.Element {
  const {t} = useTranslation('verifiable-credentials');

  const update = (id: string, patch: Partial<ClaimRow>): void =>
    onChange(claims.map((c) => (c.id === id ? {...c, ...patch} : c)));
  const add = (): void => onChange([...claims, emptyClaimRow()]);
  const remove = (id: string): void => onChange(claims.filter((c) => c.id !== id));

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

          <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', sm: '1fr 1fr'}, gap: 2}}>
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
              <FormLabel>{t('claims.displayName')}</FormLabel>
              <TextField
                size="small"
                value={claim.displayName}
                placeholder="Given Name"
                onChange={(e): void => update(claim.id, {displayName: e.target.value})}
              />
            </FormControl>
          </Box>
          <FormHelperText>{t('claims.nameHint')}</FormHelperText>
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
