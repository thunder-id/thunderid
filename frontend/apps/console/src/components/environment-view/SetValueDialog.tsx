// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  FormControl,
  FormLabel,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useSetStoredValue} from './useEnvironmentData';
import {fromListText, toListText, type ValueReference} from './values';

export interface SetValueDialogProps {
  gatewayId: string;
  environmentName: string;
  reference: ValueReference;
  /** Whether the value holds a list's items. */
  list: boolean;
  /** What the gateway holds now, for a variable; a secret's value is never read back. */
  current?: string;
  /** The value's description, kept as it is. */
  description?: string;
  onClose: () => void;
}

/**
 * Sets a value an environment's gateway fills its configuration with. A variable starts from what
 * the gateway holds, a list's one item per line; a secret starts empty, as its value is never read.
 */
export default function SetValueDialog({
  gatewayId,
  environmentName,
  reference,
  list,
  current = undefined,
  description = undefined,
  onClose,
}: SetValueDialogProps): JSX.Element {
  const {t} = useTranslation();
  const secret = reference.kind === 'secret';
  const [text, setText] = useState<string>(() => (current === undefined ? '' : list ? toListText(current) : current));
  const setValue = useSetStoredValue(gatewayId, reference.kind);
  const value = list ? fromListText(text) : text;
  const empty = list ? value === '[]' : text === '';

  return (
    <Dialog open onClose={setValue.isPending ? undefined : onClose} maxWidth="sm" fullWidth>
      <DialogTitle>
        {secret
          ? t('common:environment.value.setSecret', 'Set secret {{name}}', {name: reference.name})
          : t('common:environment.value.setVariable', 'Set variable {{name}}', {name: reference.name})}
      </DialogTitle>
      <DialogContent>
        <Stack spacing={2} sx={{mt: 1}}>
          <Typography variant="body2" color="text.secondary">
            {t(
              'common:environment.value.setDescription',
              'Sets the value {{environment}} fills this field with. The rest of the configuration is not changed.',
              {environment: environmentName},
            )}
          </Typography>
          <FormControl fullWidth>
            <FormLabel htmlFor="environment-value">{t('common:environment.value.label', 'Value')}</FormLabel>
            <TextField
              id="environment-value"
              type={secret ? 'password' : 'text'}
              multiline={list}
              minRows={list ? 3 : undefined}
              value={text}
              onChange={(event) => setText(event.target.value)}
              helperText={list ? t('common:environment.value.listHint', 'One per line.') : undefined}
            />
          </FormControl>
          {setValue.error && (
            <Alert severity="error">
              {t('common:environment.value.error', 'The value could not be set. Please try again.')}
            </Alert>
          )}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button variant="outlined" onClick={onClose} disabled={setValue.isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button
          variant="contained"
          disabled={empty || setValue.isPending}
          onClick={() => setValue.mutate({name: reference.name, value, description}, {onSuccess: onClose})}
        >
          {setValue.isPending ? t('common:status.saving', 'Saving...') : t('common:actions.save', 'Save')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
