// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SdkTheme} from '@thunderid/components';
import {BaseFlowVariables} from '@thunderid/react';
import type {FlowVariableDefinition} from '@thunderid/react';
import {Alert, Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import ConnectionsLoadError from './ConnectionsLoadError';
import useFlowVariableConnections from './useFlowVariableConnections';

/**
 * Props interface of {@link VariablesPreviewDialog}
 */
export interface VariablesPreviewDialogProps {
  onClose: () => void;
  open: boolean;
  /**
   * The organization unit that owns the flow, which limits the connections shown to the ones it owns or was shared.
   */
  ownerOuId?: string;
  /**
   * The variables to show the way a delegated admin sees them.
   */
  variables: FlowVariableDefinition[];
}

/**
 * Shows how a delegated admin sees the variables, drawn by the same component the delegated admin portal uses, so
 * what is shown is what is built.
 *
 * @param props - Props injected to the component.
 * @returns The VariablesPreviewDialog component.
 */
function VariablesPreviewDialog({
  onClose,
  open,
  ownerOuId = undefined,
  variables,
}: VariablesPreviewDialogProps): ReactElement {
  const {t} = useTranslation();
  const {connections, error: connectionsError, refetch: refetchConnections} = useFlowVariableConnections(ownerOuId);

  return (
    <Dialog
      open={open}
      onClose={onClose}
      fullWidth
      maxWidth="sm"
      // In a dark console the dialog is nearly the shade of the canvas behind it, so an outline sets it apart.
      slotProps={{paper: {sx: {border: 1, borderColor: 'divider'}}}}
      data-testid="variables-preview-dialog"
    >
      <DialogTitle>{t('flows:core.variables.preview.title', 'How a delegated admin will see it')}</DialogTitle>
      <DialogContent>
        {connectionsError && <ConnectionsLoadError error={connectionsError} onRetry={refetchConnections} />}
        {/* The SDK keeps a color scheme of its own, so it is given the console's, or its cards stay white in a dark console. */}
        <SdkTheme>
          <Box sx={{pt: 1}}>
            {variables.length === 0 ? (
              <Typography color="text.secondary">
                {t('flows:core.variables.preview.empty', 'Add a variable to see it here.')}
              </Typography>
            ) : (
              <BaseFlowVariables variables={variables} connections={connections} />
            )}
            {variables.some((variable: FlowVariableDefinition) => variable.type === 'CONNECTION_SELECT') && (
              <Alert severity="info" sx={{mt: 2}} data-testid="variables-preview-connections-note">
                {t(
                  'flows:core.variables.preview.connectionsNote',
                  'A delegated admin sees only the connections their own organization unit owns or has been shared.',
                )}
              </Alert>
            )}
          </Box>
        </SdkTheme>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('common:actions.close', 'Close')}</Button>
      </DialogActions>
    </Dialog>
  );
}

export default VariablesPreviewDialog;
