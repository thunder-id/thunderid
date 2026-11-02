// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Checkbox, Collapse, Divider, IconButton, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';

export interface AuthMethodGroupProps {
  /**
   * Optional test id for the group's root element.
   */
  'data-testid'?: string;

  /**
   * Group title, e.g. "Passwordless Login".
   */
  title: string;

  /**
   * Short description of what this group of sign-in methods does.
   */
  subtitle?: string;

  /**
   * Whether the master checkbox is checked. `true` when every method in the group is enabled,
   * `false` when none are — pair with `indeterminate` when some but not all are.
   */
  checked: boolean;

  /**
   * Whether some (but not all) methods in the group are enabled.
   */
  indeterminate?: boolean;

  /**
   * Called with the new checkbox state when the master checkbox is toggled. Bulk-enables or
   * bulk-disables every method in the group.
   */
  onCheckedChange: (checked: boolean) => void;

  /**
   * Disables the whole group — checkbox, chevron, and collapses the list — because a
   * pre-configured flow is selected instead of individual toggles.
   */
  disabled?: boolean;

  /**
   * Disables just the master checkbox, e.g. when nothing in the group is currently available to
   * enable. Unlike `disabled`, this leaves the chevron enabled so the (empty/unavailable) list can
   * still be inspected. Defaults to `disabled`.
   */
  checkboxDisabled?: boolean;

  /**
   * Whether the method list starts expanded. Defaults to collapsed.
   */
  defaultExpanded?: boolean;

  /**
   * Optional content rendered between the header and the (collapsible) list box, e.g. an info
   * alert explaining why the checkbox is disabled. Always rendered regardless of expand state.
   */
  banner?: ReactNode;

  /**
   * `AuthenticationMethodItem` rows (with `Divider`s between them) for this group.
   */
  children: ReactNode;
}

/**
 * Presentational wrapper for one category of sign-in methods on the Sign-in Experience step: a
 * master checkbox + title + subtitle header, with its own chevron to expand/collapse the method
 * list below — same master-checkbox-plus-expandable-list pattern as
 * `OrganizationUnitDefaultsSection`/`UserAccessSection`. The chevron can still collapse/expand
 * independently of the checkbox (e.g. to inspect an already-enabled group), but checking the box
 * always expands, so the admin immediately sees the default it just enabled. Used for each of the
 * Prompt for Credentials, Passwordless Login, Social Login, and Multi-Factor Login groups so they
 * read as peer categories rather than one flat list. Borderless — like those two components, the
 * shared card border and dividers between groups are owned by the parent (`ConfigureSignInOptions`).
 */
export default function AuthMethodGroup({
  'data-testid': dataTestId = undefined,
  title,
  subtitle = undefined,
  checked,
  indeterminate = false,
  onCheckedChange,
  disabled = false,
  checkboxDisabled = disabled,
  defaultExpanded = false,
  banner = undefined,
  children,
}: AuthMethodGroupProps): JSX.Element {
  const {t} = useTranslation();
  const [expanded, setExpanded] = useState(defaultExpanded);

  // Collapses the method list the moment this group becomes disabled (e.g. a pre-configured flow
  // is selected), rather than leaving an already-expanded group's now-inert rows on display, and
  // restores it to the caller's intended expanded state once re-enabled (e.g. clearing that flow
  // and going back to individual toggles) rather than leaving it stuck collapsed with no way for
  // the caller to reach the very switch that re-enables it. Adjusted during render (React's
  // recommended alternative to an effect for this) rather than committing a stale state first.
  const [prevDisabled, setPrevDisabled] = useState(disabled);
  if (disabled !== prevDisabled) {
    setPrevDisabled(disabled);
    setExpanded(disabled ? false : defaultExpanded);
  }

  return (
    <Box data-testid={dataTestId}>
      <Box sx={{display: 'flex', alignItems: 'center', gap: 1, p: 2}}>
        <Checkbox
          checked={checked}
          indeterminate={indeterminate}
          disabled={checkboxDisabled}
          onChange={(_event, next) => {
            onCheckedChange(next);
            // Checking the box reveals the list it just populated a default into, rather than
            // leaving the admin to separately hunt for the chevron.
            if (next) setExpanded(true);
          }}
          inputProps={{'aria-label': title}}
          sx={{mt: -0.5}}
        />
        <Box sx={{flex: 1, minWidth: 0}}>
          <Typography variant="body1" fontWeight={600}>
            {title}
          </Typography>
          {subtitle && (
            <Typography variant="body2" color="text.secondary">
              {subtitle}
            </Typography>
          )}
        </Box>
        <IconButton
          size="small"
          onClick={() => setExpanded((prev) => !prev)}
          disabled={disabled}
          aria-label={expanded ? t('common:actions.collapse', 'Collapse') : t('common:actions.expand', 'Expand')}
          aria-expanded={expanded}
        >
          {expanded ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
        </IconButton>
      </Box>

      {banner}

      <Collapse in={expanded} unmountOnExit>
        <Divider />
        <Box role="list" sx={{pl: 6}}>
          {children}
        </Box>
      </Collapse>
    </Box>
  );
}
