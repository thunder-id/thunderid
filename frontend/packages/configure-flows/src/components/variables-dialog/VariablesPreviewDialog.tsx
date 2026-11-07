// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {BaseFlowVariables, ThemeProvider as SdkThemeProvider} from '@thunderid/react';
import type {FlowVariableDefinition} from '@thunderid/react';
import {Alert, Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Typography} from '@wso2/oxygen-ui';
import type {ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import useConsoleSurface from './useConsoleSurface';
import useFlowVariableConnections from './useFlowVariableConnections';

/**
 * Props interface of {@link VariablesPreviewDialog}
 */
export interface VariablesPreviewDialogProps {
  onClose: () => void;
  open: boolean;
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
function VariablesPreviewDialog({onClose, open, variables}: VariablesPreviewDialogProps): ReactElement {
  const {t} = useTranslation();
  const {scheme, sdkTheme} = useConsoleSurface();
  const connections = useFlowVariableConnections();

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm" data-testid="variables-preview-dialog">
      <DialogTitle>{t('flows:core.variables.preview.title', 'How a delegated admin will see it')}</DialogTitle>
      <DialogContent>
        {/* The SDK keeps a color scheme of its own, so it is given the console's, or its cards stay white in a dark console. */}
        <SdkThemeProvider mode={scheme} theme={sdkTheme}>
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
        </SdkThemeProvider>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('flows:core.variables.preview.close', 'Close')}</Button>
      </DialogActions>
    </Dialog>
  );
}

export default VariablesPreviewDialog;
