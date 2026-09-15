// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
import {emptyClaimRow, type ClaimNameError, type ClaimRow} from '../models/credential-claims';

export interface ClaimsEditorProps {
  claims: ClaimRow[];
  onChange: (claims: ClaimRow[]) => void;
  /** Invalid claim names keyed by row id, as returned by findClaimNameErrors. */
  nameErrors?: Record<string, ClaimNameError>;
}

/**
 * Per-claim editor for a credential configuration: one row per disclosed claim,
 * with the attribute name (the user-profile lookup key) and a wallet display name.
 */
export default function ClaimsEditor({claims, onChange, nameErrors = {}}: ClaimsEditorProps): JSX.Element {
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
                error={nameErrors[claim.id] !== undefined}
                helperText={nameErrors[claim.id] ? t(`claims.errors.${nameErrors[claim.id]}`) : undefined}
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
        variant="text"
        color="primary"
        startIcon={<Plus size={16} />}
        onClick={add}
        fullWidth
        sx={{
          py: 1.5,
          border: '1px dashed',
          borderColor: 'divider',
          '&:hover': {border: '1px dashed', borderColor: 'primary.main'},
        }}
      >
        {t('claims.add')}
      </Button>
    </Stack>
  );
}
