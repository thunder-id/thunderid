// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Grid, Paper, Tab, Tabs, Typography} from '@wso2/oxygen-ui';
import {Lock} from '@wso2/oxygen-ui-icons-react';
import type {ReactNode, SyntheticEvent} from 'react';

const TAB_ID_PREFIX = 'token-audience-tab-';
const PANEL_ID = 'token-audience-panel';
const DESCRIPTION_ID_PREFIX = 'token-audience-description-';

/** One selectable token audience, i.e. one subject a token can be issued to. */
export interface TokenAudienceOption {
  /** Stable identifier passed back to onChange. */
  value: string;
  /** Short name shown in the selector, e.g. "Application". */
  label: string;
  /** One line describing which token this audience configures. */
  description: string;
  /** Whether this audience's settings do not apply to the current entity. */
  isLocked?: boolean;
}

interface TokenAudienceSelectorProps {
  /** Heading above the audience list, and the accessible name of the tab list. */
  title: string;
  options: TokenAudienceOption[];
  /** The selected option's `value`. */
  value: string;
  onChange: (value: string) => void;
  /** Optional note under the list, e.g. that each audience is configured independently. */
  footnote?: string;
  /** Disables the whole selector (e.g. for a read-only resource). Options stay visible but inert. */
  disabled?: boolean;
  /** Settings for the selected audience. */
  children: ReactNode;
}

/**
 * Two-column token settings layout: a vertical tab list of token audiences on the left, and the
 * selected audience's settings on the right.
 *
 * Built on the design system's Tabs so the full tab interaction model comes for free: roving focus,
 * arrow key navigation, and Home/End. The tabs are styled as cards to match the audience design.
 * Each tab's accessible name stays the audience label alone, with the description referenced rather
 * than folded into the name.
 *
 * Locked audiences stay selectable so their explanation is reachable without being shown to someone
 * working on a different audience. Callers decide what a locked audience renders.
 *
 * Shared by the application and agent token tabs, which differ only in their audience labels and in
 * how they present a locked audience.
 */
export default function TokenAudienceSelector({
  title,
  options,
  value,
  onChange,
  footnote = undefined,
  disabled = false,
  children,
}: TokenAudienceSelectorProps): ReactNode {
  const handleChange = (_event: SyntheticEvent, newValue: string): void => {
    onChange(newValue);
  };

  // A picker with a single choice is noise, so with one selectable audience the settings are shown
  // on their own. Locked audiences are then unreachable, along with whatever the caller renders for
  // them, which is the point: there is nothing to choose between.
  if (options.filter((option) => option.isLocked !== true).length <= 1) {
    return children;
  }

  return (
    <Grid container spacing={3}>
      <Grid size={{xs: 12, md: 4, lg: 3}}>
        <Paper variant="outlined" sx={{p: 2}}>
          <Typography variant="overline" color="text.secondary" sx={{display: 'block', mb: 1, letterSpacing: '0.08em'}}>
            {title}
          </Typography>
          <Tabs
            orientation="vertical"
            value={value}
            onChange={handleChange}
            aria-label={title}
            sx={{
              '& .MuiTabs-indicator': {display: 'none'},
              '& .MuiTabs-list, & .MuiTabs-flexContainer': {gap: 1},
            }}
          >
            {options.map((option) => (
              <Tab
                key={option.value}
                value={option.value}
                disabled={disabled}
                id={`${TAB_ID_PREFIX}${option.value}`}
                aria-controls={PANEL_ID}
                aria-label={option.label}
                aria-describedby={`${DESCRIPTION_ID_PREFIX}${option.value}`}
                label={
                  // Spans throughout: a tab renders a button, whose content model is phrasing only.
                  <Box component="span" sx={{display: 'block', width: '100%'}}>
                    <Box component="span" sx={{display: 'flex', alignItems: 'center', gap: 0.75}}>
                      <Typography component="span" variant="body2" sx={{fontWeight: 500}}>
                        {option.label}
                      </Typography>
                      {option.isLocked === true && <Lock size={12} />}
                    </Box>
                    <Typography
                      id={`${DESCRIPTION_ID_PREFIX}${option.value}`}
                      component="span"
                      variant="caption"
                      color="text.secondary"
                      sx={{display: 'block', mt: 0.25}}
                    >
                      {option.description}
                    </Typography>
                  </Box>
                }
                sx={{
                  alignItems: 'flex-start',
                  textAlign: 'left',
                  textTransform: 'none',
                  width: '100%',
                  maxWidth: 'none',
                  minHeight: 'auto',
                  px: 1.5,
                  py: 1.25,
                  borderRadius: 1,
                  border: '1px solid',
                  borderColor: 'divider',
                  '&:hover': {borderColor: 'text.disabled'},
                  '&.Mui-selected': {borderColor: 'primary.main', bgcolor: 'action.selected'},
                }}
              />
            ))}
          </Tabs>
        </Paper>
        {footnote !== undefined && (
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', mt: 1.5, px: 0.5}}>
            {footnote}
          </Typography>
        )}
      </Grid>
      <Grid size={{xs: 12, md: 8, lg: 9}} id={PANEL_ID} role="tabpanel" aria-labelledby={`${TAB_ID_PREFIX}${value}`}>
        {children}
      </Grid>
    </Grid>
  );
}
