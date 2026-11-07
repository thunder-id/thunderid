// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {BaseFlowVariables, ThemeProvider as SdkThemeProvider, getFlowVariableOptions} from '@thunderid/react';
import type {FlowVariableDefinition, FlowVariableOption} from '@thunderid/react';
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogTitle,
  Divider,
  IconButton,
  Paper,
  Stack,
  Tooltip,
  Typography,
  type Theme,
} from '@wso2/oxygen-ui';
import {Plus, SlidersHorizontal, TrashIcon, X} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import useConsoleSurface from './useConsoleSurface';
import useFlowVariableConnections from './useFlowVariableConnections';
import {createVariable, findProblems} from './variableDefinitions';
import VariableEditor from './VariableEditor';
import type {VariablesGroup} from '../../context/FlowVariablesContext';

/**
 * Props interface of {@link VariablesDialog}
 */
export interface VariablesDialogProps {
  /**
   * The variables each variables node of the flow declares now.
   */
  groups: VariablesGroup[];
  /**
   * The variable, counted across all the nodes, that is selected for editing when the dialog opens.
   * @default 0
   */
  initialIndex?: number;
  /**
   * Called with the variables of each node once they are applied.
   */
  onApply: (groups: VariablesGroup[]) => void;
  onClose: () => void;
  open: boolean;
}

/**
 * A full-screen editor for the variables a flow declares. The right side sets each variable up; the left side shows
 * how a delegated admin will see them, drawn by the same component the delegated admin portal uses, so what is shown
 * is what is built.
 *
 * @param props - Props injected to the component.
 * @returns The VariablesDialog component.
 */
function VariablesDialog({groups, initialIndex = 0, onApply, onClose, open}: VariablesDialogProps): ReactElement {
  const {t} = useTranslation();
  const {pageSx, scheme, sdkTheme} = useConsoleSurface();
  const [draft, setDraft] = useState<VariablesGroup[]>(groups);
  const [selected, setSelected] = useState<number>(0);

  useEffect(() => {
    if (open) {
      setDraft(groups);
      setSelected(initialIndex);
    }
    // The draft starts from what the flow declares each time the dialog opens, and not again while it is open.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const connections = useFlowVariableConnections();

  // The variables of every node in one list, which is what the selection and the checks count across.
  const all: FlowVariableDefinition[] = draft.flatMap((group: VariablesGroup) => group.variables);
  const current: FlowVariableDefinition | undefined = all[selected];
  const connectionOptions: FlowVariableOption[] = current ? getFlowVariableOptions(current, connections) : [];
  const hasProblems: boolean = all.some(
    (_: FlowVariableDefinition, index: number) => findProblems(all, index).length > 0,
  );

  // Where a variable of the combined list sits: in which node, and where in it.
  const locate = (position: number): {group: number; index: number} | undefined => {
    let offset = 0;

    for (let group = 0; group < draft.length; group += 1) {
      const count: number = draft[group].variables.length;

      if (position < offset + count) {
        return {group, index: position - offset};
      }
      offset += count;
    }

    return undefined;
  };

  const add = (group: number): void => {
    const position: number = draft
      .slice(0, group + 1)
      .reduce((total: number, {variables}) => total + variables.length, 0);

    setDraft((previous: VariablesGroup[]) =>
      previous.map((existing: VariablesGroup, index: number) =>
        index === group
          ? {
              ...existing,
              variables: [
                ...existing.variables,
                createVariable(previous.flatMap((other: VariablesGroup) => other.variables)),
              ],
            }
          : existing,
      ),
    );
    setSelected(position);
  };

  const remove = (position: number): void => {
    const where = locate(position);

    if (!where) {
      return;
    }

    setDraft((previous: VariablesGroup[]) =>
      previous.map((existing: VariablesGroup, index: number) =>
        index === where.group
          ? {
              ...existing,
              variables: existing.variables.filter((_: FlowVariableDefinition, i: number) => i !== where.index),
            }
          : existing,
      ),
    );
    setSelected((previous: number) =>
      Math.max(0, previous > position || previous === all.length - 1 ? previous - 1 : previous),
    );
  };

  const update = (definition: FlowVariableDefinition): void => {
    const where = locate(selected);

    if (!where) {
      return;
    }

    setDraft((previous: VariablesGroup[]) =>
      previous.map((existing: VariablesGroup, index: number) =>
        index === where.group
          ? {
              ...existing,
              variables: existing.variables.map((variable: FlowVariableDefinition, i: number) =>
                i === where.index ? definition : variable,
              ),
            }
          : existing,
      ),
    );
  };

  const apply = (): void => {
    onApply(draft);
    onClose();
  };

  return (
    <Dialog fullScreen open={open} onClose={onClose} slotProps={{paper: {sx: pageSx}}} data-testid="variables-dialog">
      <DialogTitle sx={{alignItems: 'center', display: 'flex', justifyContent: 'space-between'}}>
        {t('flows:core.variables.dialog.title', 'Flow variables')}
        <IconButton onClick={onClose} aria-label={t('flows:core.variables.dialog.close', 'Close')}>
          <X size={20} />
        </IconButton>
      </DialogTitle>
      <Divider />
      <Box sx={{display: 'flex', flex: 1, minHeight: 0}}>
        <Box
          data-testid="variables-preview-pane"
          sx={[
            {flex: 1, overflowY: 'auto', p: 5},
            // The divider alone is faint on a light page, so the preview sits on a slightly darker canvas.
            (theme: Theme) => theme.applyStyles('light', {backgroundColor: 'rgba(15, 23, 42, 0.04)'}),
          ]}
        >
          <Box sx={{maxWidth: 760, mx: 'auto'}}>
            <Typography variant="subtitle2" color="text.secondary">
              {t('flows:core.variables.dialog.preview', 'How a delegated admin will see it')}
            </Typography>
            {/* The SDK keeps a color scheme of its own, so it is given the console's, or its cards stay white in a dark console. */}
            <SdkThemeProvider mode={scheme} theme={sdkTheme}>
              <Box sx={{mt: 1.5}}>
                {draft.length === 0 && (
                  <Typography color="text.secondary">
                    {t('flows:core.variables.dialog.previewEmpty', 'Add a variable to see it here.')}
                  </Typography>
                )}
                {draft.map((group: VariablesGroup, groupIndex: number) => {
                  // Where this node's variables start in the combined list.
                  const offset: number = draft
                    .slice(0, groupIndex)
                    .reduce((total: number, {variables}: VariablesGroup) => total + variables.length, 0);

                  return (
                    <Box key={group.nodeId} data-testid={`variables-group-${group.nodeId}`}>
                      {groupIndex > 0 && <Divider sx={{my: 3}} />}
                      <Typography
                        variant="subtitle2"
                        data-testid={`variables-group-title-${group.nodeId}`}
                        sx={{mb: 1.5}}
                      >
                        {group.nodeId}
                      </Typography>
                      {group.variables.length === 0 ? (
                        <Typography color="text.secondary" variant="body2">
                          {t('flows:core.variables.dialog.groupEmpty', 'This node declares no variables yet.')}
                        </Typography>
                      ) : (
                        <BaseFlowVariables
                          variables={group.variables}
                          connections={connections}
                          renderField={(
                            field: ReactElement,
                            _definition: FlowVariableDefinition,
                            index: number,
                          ): ReactNode => (
                            <Box
                              data-testid={`variables-item-${offset + index}`}
                              onClick={() => setSelected(offset + index)}
                              sx={{
                                borderRadius: 1,
                                boxShadow: (theme: Theme) =>
                                  offset + index === selected ? `-4px 0 0 0 ${theme.palette.primary.main}` : 'none',
                                cursor: 'pointer',
                              }}
                            >
                              {field}
                            </Box>
                          )}
                          renderFieldAction={(_definition: FlowVariableDefinition, index: number): ReactNode => (
                            <Tooltip title={t('flows:core.variables.dialog.remove', 'Remove')}>
                              <IconButton
                                size="small"
                                color="error"
                                aria-label={t('flows:core.variables.dialog.remove', 'Remove')}
                                onClick={(event) => {
                                  event.stopPropagation();
                                  remove(offset + index);
                                }}
                              >
                                <TrashIcon size={16} />
                              </IconButton>
                            </Tooltip>
                          )}
                        />
                      )}
                      <Button
                        variant="outlined"
                        startIcon={<Plus size={16} />}
                        onClick={() => add(groupIndex)}
                        data-testid={`variables-add-${group.nodeId}`}
                        sx={{mt: 2, width: '100%'}}
                      >
                        {t('flows:core.variables.dialog.add', 'Add')}
                      </Button>
                    </Box>
                  );
                })}
              </Box>
            </SdkThemeProvider>
            <Typography
              variant="caption"
              color="text.secondary"
              data-testid="variables-author-note"
              sx={{display: 'block', mt: 1}}
            >
              {t(
                'flows:core.variables.dialog.authorNote',
                'The add and remove controls, and the marker on the selected variable, are only for you while you design. A delegated admin does not see them.',
              )}
            </Typography>
          </Box>
        </Box>
        <Paper
          square
          // Framed like the flow editor's configuration panel: the theme's panel elevation and no hard edge.
          elevation={5}
          data-testid="variables-inspector"
          sx={{border: 'none', display: 'flex', flexDirection: 'column', width: 400}}
        >
          <Stack
            direction="row"
            alignItems="center"
            gap={1}
            sx={{borderBottom: '1px solid', borderColor: 'divider', flexShrink: 0, minHeight: 52, px: 2}}
          >
            <SlidersHorizontal size={16} />
            <Typography variant="subtitle2" noWrap>
              {current?.label ?? t('flows:core.variables.dialog.settings', 'Variable settings')}
            </Typography>
            {current && (
              <Typography variant="body2" color="text.secondary" noWrap>
                {current.identifier}
              </Typography>
            )}
          </Stack>
          <Box sx={{flex: 1, overflowY: 'auto', p: 3}}>
            {current ? (
              <VariableEditor
                // Remount per variable so the uncontrolled options field starts from that variable's options.
                key={selected}
                connectionOptions={connectionOptions}
                definitions={all}
                index={selected}
                onChange={update}
              />
            ) : (
              <Typography color="text.secondary">
                {t('flows:core.variables.dialog.noneYet', 'This flow declares no variables yet. Add one to start.')}
              </Typography>
            )}
          </Box>
        </Paper>
      </Box>
      <Divider />
      <DialogActions sx={{px: 3, py: 2}}>
        <Button variant="text" onClick={onClose}>
          {t('flows:core.variables.dialog.cancel', 'Cancel')}
        </Button>
        <Button variant="contained" disabled={hasProblems} onClick={apply} data-testid="variables-apply">
          {t('flows:core.variables.dialog.apply', 'Apply')}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

export default VariablesDialog;
