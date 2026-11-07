// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OverlayRules} from '@thunderid/configure-resource-sharing';
import type {FlowVariableDefinition} from '@thunderid/react';
import {Box, Button, Drawer, IconButton, Stack, Typography, type Theme} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import VariableSharingRules from './VariableSharingRules';

/**
 * Props interface of {@link UnitTermsDrawer}
 */
export interface UnitTermsDrawerProps {
  /**
   * What applying is called.
   */
  applyLabel: string;
  disabled?: boolean;
  onApply: (rules: OverlayRules) => void;
  onClose: () => void;
  open: boolean;
  /**
   * The terms the drawer opens with.
   */
  rules: OverlayRules | undefined;
  subtitle: string;
  title: string;
  /**
   * The variables the flow declares.
   */
  variables: FlowVariableDefinition[];
}

/**
 * A panel at the side of the Share screen for setting the terms of one granted unit, or the terms every unit starts
 * with. The terms are edited here and only written to the draft when they are applied.
 *
 * @param props - Props injected to the component.
 * @returns The UnitTermsDrawer component.
 */
function UnitTermsDrawer({
  applyLabel,
  disabled = false,
  onApply,
  onClose,
  open,
  rules,
  subtitle,
  title,
  variables,
}: UnitTermsDrawerProps): ReactElement {
  const {t} = useTranslation();
  const [draft, setDraft] = useState<OverlayRules>(rules ?? {});

  useEffect(() => {
    if (open) {
      setDraft(rules ?? {});
    }
    // The terms start from what the draft holds each time the drawer opens, and not again while it is open.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  return (
    <Drawer
      anchor="right"
      open={open}
      onClose={onClose}
      // The Share screen is a full-screen dialog, which sits above a drawer's default layer and would hide it. The drawer
      // takes the dialog's layer, not one above it, so the menus its fields open, which use that layer too, stay on top.
      sx={{zIndex: (theme: Theme) => theme.zIndex.modal}}
      slotProps={{paper: {sx: {display: 'flex', flexDirection: 'column', maxWidth: '100%', width: 660}}}}
      data-testid="unit-terms-drawer"
    >
      <Stack
        direction="row"
        justifyContent="space-between"
        alignItems="flex-start"
        gap={2}
        sx={{borderBottom: '1px solid', borderColor: 'divider', px: 3.5, py: 3}}
      >
        <Box>
          <Typography variant="h6">{title}</Typography>
          <Typography variant="body2" color="text.secondary">
            {subtitle}
          </Typography>
        </Box>
        <IconButton onClick={onClose} aria-label={t('flows:core.sharing.terms.close', 'Close')}>
          <X size={18} />
        </IconButton>
      </Stack>
      <Box sx={{flex: 1, overflowY: 'auto', px: 3.5, py: 2.5}}>
        <Typography variant="body2" color="text.secondary" sx={{mb: 2}}>
          {t(
            'flows:core.sharing.terms.intro',
            'Without a rule, a variable works as the flow sets it. Lock a variable, or let the unit choose within limits you set.',
          )}
        </Typography>
        <VariableSharingRules bare variables={variables} rules={draft} onChange={setDraft} disabled={disabled} />
      </Box>
      <Stack
        direction="row"
        justifyContent="flex-end"
        gap={1.5}
        sx={{borderTop: '1px solid', borderColor: 'divider', px: 3.5, py: 2}}
      >
        <Button variant="text" onClick={onClose}>
          {t('flows:core.sharing.terms.cancel', 'Cancel')}
        </Button>
        <Button variant="contained" disabled={disabled} onClick={() => onApply(draft)} data-testid="unit-terms-apply">
          {applyLabel}
        </Button>
      </Stack>
    </Drawer>
  );
}

export default UnitTermsDrawer;
