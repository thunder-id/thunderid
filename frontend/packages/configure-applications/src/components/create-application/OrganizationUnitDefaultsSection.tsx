// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnit} from '@thunderid/configure-organization-units';
import {Box, Checkbox, Collapse, Divider, IconButton, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {
  OrganizationUnitDefaultItem,
  type OrganizationUnitDefaultsSelection,
} from '../../models/application-create-flow';
import computeOrganizationUnitDefaultAvailability from '../../utils/computeOrganizationUnitDefaultAvailability';

export interface OrganizationUnitDefaultsSectionProps {
  /**
   * The resolved organization unit whose defaults are being offered. Undefined while loading.
   */
  organizationUnit: OrganizationUnit | undefined;

  /**
   * Whether the organization unit is still being fetched.
   */
  isLoading: boolean;

  /**
   * Current per-item "use organization unit default" selection.
   */
  ouDefaults: OrganizationUnitDefaultsSelection;

  /**
   * Callback invoked when the selection changes.
   */
  onChange: (selection: OrganizationUnitDefaultsSelection) => void;

  /**
   * Whether the selected template's flow includes a Security step. Templates without one (e.g.
   * machine-to-machine backends) never present a hosted sign-in screen, so the flows group is
   * hidden regardless of what the organization unit has configured.
   */
  hasSecurityStep: boolean;

  /**
   * Whether the selected template's flow includes a Design step. Templates without one have no
   * theme or layout of their own, so the Design group is hidden regardless of what the
   * organization unit has configured.
   */
  hasDesignStep: boolean;
}

interface DefaultGroup {
  items: OrganizationUnitDefaultItem[];
  titleKey: string;
  titleDefault: string;
  descriptionKey: string;
  descriptionDefault: string;
}

/** Sign in / sign up / recovery / sign out flows, presented as a single "flows" checkbox. */
const FLOWS_GROUP: DefaultGroup = {
  items: [
    OrganizationUnitDefaultItem.SIGN_IN,
    OrganizationUnitDefaultItem.SIGN_UP,
    OrganizationUnitDefaultItem.RECOVERY,
    OrganizationUnitDefaultItem.SIGN_OUT,
  ],
  titleKey: 'applications:onboarding.configure.applicationDetails.ouDefaults.flows.title',
  titleDefault: 'Sign-in, sign-up & recovery flows',
  descriptionKey: 'applications:onboarding.configure.applicationDetails.ouDefaults.flows.description',
  descriptionDefault: 'Follow the flows configured for {{ouName}}',
};

/** Theme and layout, presented as a single "Design" checkbox. */
const DESIGN_GROUP: DefaultGroup = {
  items: [OrganizationUnitDefaultItem.THEME, OrganizationUnitDefaultItem.LAYOUT],
  titleKey: 'applications:onboarding.configure.applicationDetails.ouDefaults.design.title',
  titleDefault: 'Design',
  descriptionKey: 'applications:onboarding.configure.applicationDetails.ouDefaults.design.description',
  descriptionDefault: 'Use the same theme & layout as {{ouName}}',
};

const GROUPS: DefaultGroup[] = [FLOWS_GROUP, DESIGN_GROUP];

/**
 * Collapsible accordion for choosing whether to snapshot the organization unit's configured
 * defaults (Sign In / Sign Up / Recovery / Sign Out flow, Theme, Layout) into the application
 * being created, instead of building each one from scratch. Theme and layout are grouped behind a
 * single "Design" checkbox. Items the organization unit has no value for are silently left off
 * when a checkbox is switched on.
 */
export default function OrganizationUnitDefaultsSection({
  organizationUnit,
  isLoading,
  ouDefaults,
  onChange,
  hasSecurityStep,
  hasDesignStep,
}: OrganizationUnitDefaultsSectionProps): JSX.Element | null {
  const {t} = useTranslation();
  const [expanded, setExpanded] = useState(false);

  const availability = useMemo(
    (): Record<OrganizationUnitDefaultItem, boolean> =>
      computeOrganizationUnitDefaultAvailability(organizationUnit, {hasSecurityStep, hasDesignStep}),
    [organizationUnit, hasSecurityStep, hasDesignStep],
  );

  const availableGroups = GROUPS.map((group) => ({
    ...group,
    availableItems: group.items.filter((item) => availability[item]),
  })).filter((group) => group.availableItems.length > 0);

  const allAvailableItems = availableGroups.flatMap((group) => group.availableItems);
  const selectedCount = allAvailableItems.filter((item) => ouDefaults[item]).length;
  const masterChecked = allAvailableItems.length > 0 && selectedCount === allAvailableItems.length;
  const masterIndeterminate = selectedCount > 0 && selectedCount < allAvailableItems.length;

  if (isLoading || allAvailableItems.length === 0) {
    return null;
  }

  const setItems = (items: OrganizationUnitDefaultItem[], checked: boolean): void => {
    const next = {...ouDefaults};
    items.forEach((item) => {
      next[item] = checked;
    });
    onChange(next);
  };

  const ouName = organizationUnit?.name;

  return (
    <Box data-testid="application-configure-ou-defaults">
      <Box sx={{display: 'flex', alignItems: 'flex-start', gap: 1, p: 2}}>
        <Checkbox
          checked={masterChecked}
          indeterminate={masterIndeterminate}
          onChange={(_event, checked) => setItems(allAvailableItems, checked)}
          sx={{mt: -0.5}}
        />
        <Box sx={{flex: 1, minWidth: 0}}>
          <Typography variant="body1" fontWeight={600}>
            {t('applications:onboarding.configure.applicationDetails.ouDefaults.title', 'Use organization defaults')}
          </Typography>
          <Typography variant="body2" color="text.secondary">
            {t(
              'applications:onboarding.configure.applicationDetails.ouDefaults.subtitle',
              "Inherit {{ouName}}'s flows, theme & layout instead of configuring from scratch.",
              {ouName},
            )}
          </Typography>
        </Box>
        <IconButton
          size="small"
          onClick={() => setExpanded((prev) => !prev)}
          aria-label={expanded ? t('common:actions.collapse', 'Collapse') : t('common:actions.expand', 'Expand')}
        >
          {expanded ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
        </IconButton>
      </Box>

      <Collapse in={expanded}>
        <Divider />
        <Box sx={{pt: 2, pr: 2, pb: 2, pl: 6, display: 'flex', flexDirection: 'column', gap: 2}}>
          {availableGroups.map((group) => {
            const groupSelectedCount = group.availableItems.filter((item) => ouDefaults[item]).length;
            const groupChecked = groupSelectedCount === group.availableItems.length;
            const groupIndeterminate = groupSelectedCount > 0 && groupSelectedCount < group.availableItems.length;

            return (
              <Box key={group.titleKey} sx={{display: 'flex', alignItems: 'flex-start', gap: 1}}>
                <Checkbox
                  checked={groupChecked}
                  indeterminate={groupIndeterminate}
                  onChange={(_event, checked) => setItems(group.availableItems, checked)}
                  sx={{mt: -0.5}}
                />
                <Box>
                  <Typography variant="body2" fontWeight={600}>
                    {t(group.titleKey, group.titleDefault)}
                  </Typography>
                  <Typography variant="body2" color="text.secondary">
                    {t(group.descriptionKey, group.descriptionDefault, {ouName})}
                  </Typography>
                </Box>
              </Box>
            );
          })}
        </Box>
      </Collapse>
    </Box>
  );
}
