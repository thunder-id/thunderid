// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Box, Button, Paper, Stack, Typography} from '@wso2/oxygen-ui';

export interface UnsavedChangesBarProps {
  /** Label for the unsaved changes message. */
  message: string;
  /** Label for the reset button. */
  resetLabel: string;
  /** Label for the save button in idle state. */
  saveLabel: string;
  /** Label for the save button while saving is in progress. */
  savingLabel: string;
  /** Whether a save operation is currently in progress. */
  isSaving: boolean;
  /** Whether the save button should be disabled (e.g. due to validation errors). */
  saveDisabled?: boolean;
  /** Whether the reset button should be disabled (e.g. while a save is still running). */
  resetDisabled?: boolean;
  /** Inline error to show above the actions when the last save attempt failed. */
  error?: string;
  /** Called when the reset button is clicked. */
  onReset: () => void;
  /** Called when the save button is clicked. */
  onSave: () => void;
}

/**
 * A fixed bottom action bar shown when a form has unsaved changes.
 * Provides reset and save actions.
 */
export default function UnsavedChangesBar({
  message,
  resetLabel,
  saveLabel,
  savingLabel,
  isSaving,
  saveDisabled = false,
  resetDisabled = false,
  error = undefined,
  onReset,
  onSave,
}: UnsavedChangesBarProps) {
  return (
    <Paper
      sx={{
        position: 'fixed',
        bottom: 0,
        left: 0,
        right: 0,
        p: 2,
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: 2,
        borderRadius: '12px 12px 0 0',
        boxShadow: '0 -4px 20px rgba(0, 0, 0, 0.1)',
        zIndex: 1000,
        bgcolor: 'background.paper',
      }}
    >
      {error && (
        <Alert severity="error" sx={{width: '100%', maxWidth: 720}}>
          {error}
        </Alert>
      )}
      <Stack direction="row" spacing={2} alignItems="center">
        <Typography variant="body2" sx={{display: 'flex', alignItems: 'center', gap: 1}}>
          <Box
            component="span"
            sx={{
              width: 20,
              height: 20,
              borderRadius: '50%',
              border: '2px solid',
              borderColor: 'text.secondary',
              color: 'text.secondary',
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontSize: '12px',
              fontWeight: 'bold',
            }}
          >
            !
          </Box>
          {message}
        </Typography>
        <Button variant="outlined" color="inherit" onClick={onReset} disabled={resetDisabled}>
          {resetLabel}
        </Button>
        <Button variant="contained" onClick={onSave} disabled={isSaving || saveDisabled}>
          {isSaving ? savingLabel : saveLabel}
        </Button>
      </Stack>
    </Paper>
  );
}
