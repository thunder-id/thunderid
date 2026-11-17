// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Button,
  CircularProgress,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {ValueName} from './ValueCells';
import type {ValueRow} from './ValuesTable';
import {useSetStoredValue} from '../api/useStoredValues';
import type {ValueKind} from '../models/deploy';
import {fromListText, toListText} from '../utils/listValue';
import {isListRow} from '../utils/valueRows';

export interface ValueEditDialogProps {
  gatewayId: string;
  kind: ValueKind;
  /** The value to set; the dialog is open while there is one. */
  row: ValueRow | null;
  onClose: () => void;
}

/**
 * Sets one variable or secret on a gateway. A variable starts from what the gateway holds, a list one
 * item per line; a secret's value is never read back, so it starts empty and is replaced whole.
 */
export default function ValueEditDialog({gatewayId, kind, row, onClose}: ValueEditDialogProps): JSX.Element {
  return (
    <Dialog open={row !== null} onClose={onClose} maxWidth="sm" fullWidth>
      {row && <ValueEditForm key={row.name} gatewayId={gatewayId} kind={kind} row={row} onClose={onClose} />}
    </Dialog>
  );
}

function ValueEditForm({
  gatewayId,
  kind,
  row,
  onClose,
}: {
  gatewayId: string;
  kind: ValueKind;
  row: ValueRow;
  onClose: () => void;
}): JSX.Element {
  const {t} = useTranslation('deploy');
  const setValue = useSetStoredValue(gatewayId, kind);
  const secret = kind === 'secret';
  const list = !secret && isListRow(row);
  const [draft, setDraft] = useState(secret ? '' : toListText(row.stored?.value ?? ''));

  const save = async (): Promise<void> => {
    try {
      await setValue.mutateAsync({
        name: row.name,
        value: list ? fromListText(draft) : draft,
        description: row.stored?.description,
      });
      onClose();
    } catch {
      // The failure is shown in the dialog, from the mutation's own state.
    }
  };

  return (
    <>
      <DialogTitle>{t('values.setLabel', 'Set {{name}}', {name: row.name})}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{pt: 1}}>
          <ValueName row={row} />
          <TextField
            size="small"
            fullWidth
            type={secret ? 'password' : 'text'}
            autoComplete={secret ? 'new-password' : 'off'}
            multiline={list}
            minRows={list ? 2 : undefined}
            value={draft}
            onChange={(event) => {
              setDraft(event.target.value);
              setValue.reset();
            }}
            placeholder={secret ? t('values.secretPlaceholder', 'Enter a new value to replace') : undefined}
            helperText={list ? t('values.listHint', 'One per line.') : undefined}
            inputProps={{'aria-label': row.name}}
            slotProps={{input: {sx: {fontFamily: 'monospace', fontSize: 13}}}}
          />
          {setValue.isError && (
            <Alert severity="error">{t('values.saveFailed', 'The value could not be saved.')}</Alert>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('values.cancel', 'Cancel')}</Button>
        <Button
          variant="contained"
          onClick={() => void save()}
          disabled={setValue.isPending || (secret && draft === '')}
          startIcon={setValue.isPending ? <CircularProgress size={14} /> : undefined}
        >
          {t('values.save', 'Save')}
        </Button>
      </DialogActions>
    </>
  );
}
