// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button, Dialog, DialogActions, DialogContent, DialogTitle, Stack, Typography} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props interface of {@link VariableChangeDialog}
 */
export interface VariableChangeDialogProps {
  /**
   * The label of the button that goes ahead with the change.
   */
  confirmLabel: string;
  /**
   * What the change does, one sentence each.
   */
  details: string[];
  onClose: () => void;
  onConfirm: () => void;
  open: boolean;
  title: string;
}

/**
 * Asks for a go-ahead before a change to a variable that takes something else with it.
 *
 * @param props - Props injected to the component.
 * @returns The VariableChangeDialog component.
 */
function VariableChangeDialog({
  confirmLabel,
  details,
  onClose,
  onConfirm,
  open,
  title,
}: VariableChangeDialogProps): ReactElement {
  const {t} = useTranslation();

  return (
    <Dialog
      open={open}
      onClose={onClose}
      maxWidth="xs"
      fullWidth
      // In a dark console the dialog is nearly the shade of the canvas behind it, so an outline sets it apart.
      slotProps={{paper: {sx: {border: 1, borderColor: 'divider'}}}}
      data-testid="variable-change-dialog"
    >
      <DialogTitle>{title}</DialogTitle>
      <DialogContent>
        <Stack gap={1}>
          {details.map((detail: string) => (
            <Typography key={detail} variant="body2">
              {detail}
            </Typography>
          ))}
        </Stack>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('common:actions.cancel', 'Cancel')}</Button>
        <Button onClick={onConfirm} color="error" variant="contained" data-testid="variable-change-confirm">
          {confirmLabel}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

export default VariableChangeDialog;
