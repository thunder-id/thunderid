// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {type BasicFlowDefinition} from '@thunderid/configure-flows';
import {alpha, Autocomplete, Box, ButtonBase, Collapse, Divider, TextField, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp, Workflow} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the FlowsListView component
 */
export interface FlowsListViewProps {
  /**
   * Available authentication flows
   */
  availableFlows: BasicFlowDefinition[];

  /**
   * Currently selected authentication flow
   */
  selectedAuthFlow: BasicFlowDefinition | null;

  /**
   * Callback when a flow is selected
   */
  onFlowSelect: (flowId: string) => void;

  /**
   * Callback when clearing flow selection
   */
  onClearSelection: () => void;

  /**
   * Whether the flows list should be disabled
   */
  disabled?: boolean;
}

/**
 * Renders the "use a pre-configured flow instead" picker below the Prompt for Credentials /
 * Passwordless / Social / MFA groups. Picking a flow isn't a set of independent per-method
 * toggles like those groups, so it deliberately looks different rather than joining their shared
 * card: its own dashed-bordered card, behind a plain click-to-expand row (icon + title + chevron)
 * instead of a master checkbox, preceded by an "or" divider to read as an alternative rather than
 * another peer category.
 */
export default function FlowsListView({
  availableFlows,
  selectedAuthFlow,
  onFlowSelect,
  onClearSelection,
  disabled = false,
}: FlowsListViewProps): JSX.Element | null {
  const {t} = useTranslation();
  const [expanded, setExpanded] = useState(selectedAuthFlow !== null);

  // Exclude the console-app flows which is reserved for the system management console itself & default flows.
  const selectableFlows: BasicFlowDefinition[] = useMemo(
    () => availableFlows.filter((flow) => !flow.handle.includes('console-app-') && !flow.handle.startsWith('default-')),
    [availableFlows],
  );

  if (selectableFlows.length === 0) {
    return null;
  }

  return (
    <>
      <Divider>
        <Typography variant="body2" color="text.secondary" sx={{px: 2}}>
          {t('common:or')}
        </Typography>
      </Divider>

      <Box sx={{border: '1px dashed', borderColor: 'divider', borderRadius: '10px', overflow: 'hidden'}}>
        <ButtonBase
          onClick={() => setExpanded((prev) => !prev)}
          aria-expanded={expanded}
          sx={{display: 'flex', alignItems: 'center', gap: 1.5, p: 2, width: '100%', textAlign: 'left'}}
        >
          <Box
            sx={{
              width: 34,
              height: 34,
              borderRadius: '10px',
              bgcolor: (theme) => alpha(theme.palette.primary.main, 0.12),
              color: 'primary.main',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
            }}
          >
            <Workflow size={18} />
          </Box>
          <Typography variant="body1" fontWeight={600} sx={{flex: 1, minWidth: 0}}>
            {t('applications:onboarding.configure.SignInOptions.preConfiguredFlows.toggleLabel')}
          </Typography>
          {expanded ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
        </ButtonBase>

        <Collapse in={expanded} unmountOnExit>
          <Divider />
          <Box sx={{p: 2}}>
            <Autocomplete
              disabled={disabled}
              options={selectableFlows}
              getOptionLabel={(option) => option.name}
              value={selectableFlows.find((flow) => flow.id === selectedAuthFlow?.id) ?? null}
              onChange={(_, newValue) => {
                if (newValue?.id) {
                  onFlowSelect(newValue.id);
                } else {
                  onClearSelection();
                }
              }}
              renderInput={(params) => (
                <TextField
                  {...params}
                  placeholder={t('applications:onboarding.configure.SignInOptions.preConfiguredFlows.searchFlows')}
                />
              )}
              renderOption={(props, option) => (
                <Box component="li" {...props}>
                  <Box sx={{display: 'flex', alignItems: 'center', gap: 1, width: '100%'}}>
                    <Workflow size={20} />
                    <Box>
                      <Typography variant="body2" fontWeight="medium">
                        {option.name}
                      </Typography>
                    </Box>
                  </Box>
                </Box>
              )}
            />
          </Box>
        </Collapse>
      </Box>
    </>
  );
}
