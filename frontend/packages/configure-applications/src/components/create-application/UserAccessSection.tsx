// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ToggleCard} from '@thunderid/components';
import type {UserTypeListItem} from '@thunderid/configure-user-types';
import {Autocomplete, Box, Checkbox, Collapse, Divider, IconButton, TextField, Typography} from '@wso2/oxygen-ui';
import {ChevronDown, ChevronUp} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';

export interface UserAccessSectionProps {
  /**
   * Available user types. The section renders nothing when fewer than two exist.
   */
  userTypes: UserTypeListItem[];

  /**
   * Currently selected user type names.
   */
  selectedUserTypes: string[];

  /**
   * Callback invoked when the selection changes.
   */
  onUserTypesChange: (userTypes: string[]) => void;
}

/** Above this many user types, the expanded list switches to a searchable Autocomplete. */
const AUTOCOMPLETE_THRESHOLD = 5;

/**
 * Lets an admin restrict which user types can sign up through the application being
 * created, using the same master-checkbox-plus-expandable-list pattern as
 * OrganizationUnitDefaultsSection. Renders nothing when the deployment has fewer than two user
 * types, since a single type is used implicitly.
 */
export default function UserAccessSection({
  userTypes,
  selectedUserTypes,
  onUserTypesChange,
}: UserAccessSectionProps): JSX.Element | null {
  const {t} = useTranslation();
  const [expanded, setExpanded] = useState(false);

  if (userTypes.length < 2) {
    return null;
  }

  const allNames = userTypes.map((userType) => userType.name);
  const selectedCount = allNames.filter((name) => selectedUserTypes.includes(name)).length;
  const allSelected = selectedCount === allNames.length;
  const noneSelected = selectedCount === 0;
  const indeterminate = !allSelected && !noneSelected;
  const useAutocomplete = userTypes.length > AUTOCOMPLETE_THRESHOLD;

  const handleMasterChange = (checked: boolean): void => {
    onUserTypesChange(checked ? allNames : []);
    // Unchecking "allow all" leaves no valid resting selection, so open the list immediately
    // rather than leaving the admin on an empty, still-collapsed state.
    if (!checked) {
      setExpanded(true);
    }
  };

  const handleToggleUserType = (name: string, checked: boolean): void => {
    onUserTypesChange(checked ? [...selectedUserTypes, name] : selectedUserTypes.filter((n) => n !== name));
  };

  const title = t(
    'applications:onboarding.configure.applicationDetails.userAccess.title',
    'Allow all user types to sign up for this application',
  );

  return (
    <Box data-testid="application-configure-user-access">
      <ToggleCard
        bordered={false}
        checked={allSelected}
        indeterminate={indeterminate}
        onChange={handleMasterChange}
        title={title}
        subtitle={t(
          'applications:onboarding.configure.applicationDetails.userAccess.subtitle',
          'Users can sign up as any user type',
        )}
        error={
          noneSelected
            ? t('applications:onboarding.configure.details.userTypes.error', 'Please select at least one user type')
            : undefined
        }
        action={
          <IconButton
            size="small"
            onClick={() => setExpanded((prev) => !prev)}
            aria-label={expanded ? t('common:actions.collapse', 'Collapse') : t('common:actions.expand', 'Expand')}
          >
            {expanded ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
          </IconButton>
        }
      />

      <Collapse in={expanded}>
        <Divider />
        <Box sx={{pt: 2, pr: 2, pb: 2, pl: 6}}>
          {useAutocomplete ? (
            <Autocomplete
              multiple
              size="small"
              options={userTypes}
              getOptionLabel={(option) => option.name}
              value={userTypes.filter((userType) => selectedUserTypes.includes(userType.name))}
              onChange={(_event, newValue: UserTypeListItem[]): void => {
                onUserTypesChange(newValue.map((userType) => userType.name));
              }}
              isOptionEqualToValue={(option, value) => option.name === value.name}
              renderInput={(params) => (
                <TextField
                  {...params}
                  placeholder={t(
                    'applications:onboarding.configure.applicationDetails.userAccess.placeholder',
                    'Select user types',
                  )}
                />
              )}
            />
          ) : (
            <Box sx={{display: 'flex', flexDirection: 'column', gap: 1.5}}>
              {userTypes.map((userType) => (
                <Box key={userType.id} sx={{display: 'flex', alignItems: 'center', gap: 1}}>
                  <Checkbox
                    checked={selectedUserTypes.includes(userType.name)}
                    onChange={(_event, checked) => handleToggleUserType(userType.name, checked)}
                    inputProps={{'aria-label': userType.name}}
                    sx={{p: 0.5}}
                  />
                  <Typography variant="body2">{userType.name}</Typography>
                </Box>
              ))}
            </Box>
          )}
        </Box>
      </Collapse>
    </Box>
  );
}
