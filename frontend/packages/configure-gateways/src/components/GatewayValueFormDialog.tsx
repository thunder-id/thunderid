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
import {
  GATEWAY_VALUE_DESCRIPTION_MAX_LENGTH,
  GATEWAY_VALUE_MAX_LENGTH,
  GATEWAY_VALUE_NAME_MAX_LENGTH,
} from '../constants/gateway-values';
import {
  validateGatewayValueDescription,
  validateGatewayValueName,
  validateGatewayValueValue,
} from '../utils/validateGatewayValue';

export interface GatewayValueFormValues {
  name: string;
  value: string;
  description: string;
}

export interface GatewayValueFormDialogProps {
  /** Distinguishes the field ids of a variable form from a secret form. */
  idPrefix: string;
  title: string;
  description?: string;
  /** False when editing, where the name is fixed. */
  nameEditable: boolean;
  initialValues: GatewayValueFormValues;
  valueLabel: string;
  valueHint?: string;
  /** A secret's value is entered like a password and never shown. */
  secretValue?: boolean;
  submitLabel: string;
  submittingLabel: string;
  isPending: boolean;
  /** The resolved message of a failed save, shown beside the submit control. */
  error?: string;
  /** Called on every edit, so a stale save error can be cleared. */
  onEdit: () => void;
  onSubmit: (values: GatewayValueFormValues) => void;
  onClose: () => void;
}

/**
 * Adds or edits a variable or a secret, checking it against the rules the gateway's store applies
 * before it is sent.
 */
export default function GatewayValueFormDialog({
  idPrefix,
  title,
  description = undefined,
  nameEditable,
  initialValues,
  valueLabel,
  valueHint = undefined,
  secretValue = false,
  submitLabel,
  submittingLabel,
  isPending,
  error = undefined,
  onEdit,
  onSubmit,
  onClose,
}: GatewayValueFormDialogProps): JSX.Element {
  const {t} = useTranslation();
  const [values, setValues] = useState<GatewayValueFormValues>(initialValues);
  const [touched, setTouched] = useState<Partial<Record<keyof GatewayValueFormValues, boolean>>>({});

  const nameProblem = nameEditable ? validateGatewayValueName(values.name) : undefined;
  const valueProblem = validateGatewayValueValue(values.value);
  const descriptionProblem = validateGatewayValueDescription(values.description);
  const isValid = !nameProblem && !valueProblem && !descriptionProblem;

  let nameError: string | undefined;
  if (touched.name && nameProblem === 'required') {
    nameError = t('gateways:values.form.name.required', 'A name is required.');
  } else if (nameProblem === 'tooLong') {
    nameError = t('gateways:values.form.name.tooLong', 'A name can be at most {{max}} characters.', {
      max: GATEWAY_VALUE_NAME_MAX_LENGTH,
    });
  } else if (nameProblem === 'pattern') {
    nameError = t(
      'gateways:values.form.name.pattern',
      'Use only letters, digits and underscores, and start with a letter or an underscore.',
    );
  }

  let valueError: string | undefined;
  if (touched.value && valueProblem === 'required') {
    valueError = t('gateways:values.form.value.required', 'A value is required.');
  } else if (valueProblem === 'tooLong') {
    valueError = t('gateways:values.form.value.tooLong', 'A value can be at most {{max}} characters.', {
      max: GATEWAY_VALUE_MAX_LENGTH,
    });
  }

  const descriptionError =
    descriptionProblem === 'tooLong'
      ? t('gateways:values.form.description.tooLong', 'A description can be at most {{max}} characters.', {
          max: GATEWAY_VALUE_DESCRIPTION_MAX_LENGTH,
        })
      : undefined;

  const handleChange = (field: keyof GatewayValueFormValues, next: string): void => {
    onEdit();
    setValues((prev) => ({...prev, [field]: next}));
    setTouched((prev) => ({...prev, [field]: true}));
  };

  const handleClose = (): void => {
    if (isPending) return;
    onClose();
  };

  const handleSubmit = (): void => {
    if (!isValid || isPending) return;
    onSubmit(values);
  };

  return (
    <Dialog open onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Stack spacing={3} sx={{pt: 1}}>
          {description && (
            <Typography variant="body2" color="text.secondary">
              {description}
            </Typography>
          )}
          <FormControl fullWidth required={nameEditable}>
            <FormLabel htmlFor={`${idPrefix}-name-input`}>{t('gateways:values.form.name.label', 'Name')}</FormLabel>
            <TextField
              id={`${idPrefix}-name-input`}
              fullWidth
              value={values.name}
              disabled={!nameEditable}
              onChange={(e) => handleChange('name', e.target.value)}
              placeholder={t('gateways:values.form.name.placeholder', 'e.g. PAYMENTS_CLIENT_ID')}
              error={Boolean(nameError)}
              helperText={nameError}
              slotProps={{htmlInput: {sx: {fontFamily: 'monospace'}}}}
            />
          </FormControl>
          <FormControl fullWidth required>
            <FormLabel htmlFor={`${idPrefix}-value-input`}>{valueLabel}</FormLabel>
            <TextField
              id={`${idPrefix}-value-input`}
              fullWidth
              type={secretValue ? 'password' : 'text'}
              autoComplete={secretValue ? 'new-password' : 'off'}
              multiline={!secretValue}
              maxRows={secretValue ? undefined : 6}
              value={values.value}
              onChange={(e) => handleChange('value', e.target.value)}
              error={Boolean(valueError)}
              helperText={valueError ?? valueHint}
            />
          </FormControl>
          <FormControl fullWidth>
            <FormLabel htmlFor={`${idPrefix}-description-input`}>
              {t('gateways:values.form.description.label', 'Description (optional)')}
            </FormLabel>
            <TextField
              id={`${idPrefix}-description-input`}
              fullWidth
              value={values.description}
              onChange={(e) => handleChange('description', e.target.value)}
              error={Boolean(descriptionError)}
              helperText={descriptionError}
            />
          </FormControl>
          {error && <Alert severity="error">{error}</Alert>}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button variant="outlined" onClick={handleClose} disabled={isPending}>
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button variant="contained" onClick={handleSubmit} disabled={!isValid || isPending}>
          {isPending ? submittingLabel : submitLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}
