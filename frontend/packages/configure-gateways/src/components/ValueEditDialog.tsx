// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {getErrorMessage} from '@thunderid/utils';
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
import useGatewayErrorTranslator from '../hooks/useGatewayErrorTranslator';
import useValueActionLabel from '../hooks/useValueActionLabel';
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
 * item per line; a secret's value is never read back, so it starts empty and is replaced whole. While a
 * save is pending the dialog cannot be closed or edited, so the save that ends it is always the one for the
 * value it shows.
 */
export default function ValueEditDialog({gatewayId, kind, row, onClose}: ValueEditDialogProps): JSX.Element {
  const setValue = useSetStoredValue(gatewayId, kind);
  const done = (): void => {
    setValue.reset();
    onClose();
  };
  // Every way out but a successful save is closed while one is pending.
  const cancel = (): void => {
    if (!setValue.isPending) done();
  };
  return (
    <Dialog open={row !== null} onClose={cancel} maxWidth="sm" fullWidth>
      {row && (
        <ValueEditForm key={row.name} kind={kind} row={row} setValue={setValue} onCancel={cancel} onSaved={done} />
      )}
    </Dialog>
  );
}

function ValueEditForm({
  kind,
  row,
  setValue,
  onCancel,
  onSaved,
}: {
  kind: ValueKind;
  row: ValueRow;
  setValue: ReturnType<typeof useSetStoredValue>;
  onCancel: () => void;
  onSaved: () => void;
}): JSX.Element {
  const {t} = useTranslation('deploy');
  const tForErrors = useGatewayErrorTranslator();
  const secret = kind === 'secret';
  const list = !secret && isListRow(row);
  const stored = row.stored?.value ?? '';
  // Only a list is shown one item per line; any other value is edited as it is stored, so saving it
  // unchanged keeps it.
  const [draft, setDraft] = useState(() => {
    if (secret) return '';
    return list ? toListText(stored) : stored;
  });
  const {label: title} = useValueActionLabel(row, kind);

  const save = async (): Promise<void> => {
    try {
      await setValue.mutateAsync({
        name: row.name,
        value: list ? fromListText(draft) : draft,
        description: row.stored?.description,
      });
      onSaved();
    } catch {
      // The failure is shown in the dialog, from the mutation's own state.
    }
  };

  return (
    <>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{pt: 1}}>
          <ValueName row={row} />
          <TextField
            size="small"
            fullWidth
            disabled={setValue.isPending}
            type={secret ? 'password' : 'text'}
            autoComplete={secret ? 'new-password' : 'off'}
            multiline={list}
            minRows={list ? 2 : undefined}
            value={draft}
            onChange={(event) => {
              setDraft(event.target.value);
              if (setValue.isError) setValue.reset();
            }}
            placeholder={secret ? t('values.secretPlaceholder', 'Enter a new value to replace') : undefined}
            helperText={list ? t('values.listHint', 'One per line.') : undefined}
            inputProps={{'aria-label': row.name}}
            slotProps={{input: {sx: {fontFamily: 'monospace', fontSize: 13}}}}
          />
          {setValue.error && (
            <Alert severity="error">
              {getErrorMessage(setValue.error, tForErrors, 'deploy:values.saveFailed', 'The value could not be saved.')}
            </Alert>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onCancel} disabled={setValue.isPending}>
          {t('values.cancel', 'Cancel')}
        </Button>
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
