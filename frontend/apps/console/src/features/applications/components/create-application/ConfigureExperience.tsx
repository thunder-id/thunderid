/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import type {UserTypeListItem} from '@thunderid/configure-user-types';
import {useConfig} from '@thunderid/contexts';
import {
  Box,
  Typography,
  Stack,
  Card,
  CardContent,
  Radio,
  RadioGroup,
  FormControlLabel,
  CardActionArea,
  FormControl,
  Autocomplete,
  TextField,
  Grid,
} from '@wso2/oxygen-ui';
import {ExternalLink, Code, User} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ChangeEvent} from 'react';
import {useEffect} from 'react';
import {useTranslation} from 'react-i18next';
import {ApplicationCreateFlowSignInApproach} from '../../models/application-create-flow';

/**
 * Props for the {@link ConfigureExperience} component.
 *
 * @public
 */
export interface ConfigureExperienceProps {
  /**
   * Currently selected sign-in approach
   */
  selectedApproach: ApplicationCreateFlowSignInApproach;

  /**
   * Callback function when the sign-in approach changes
   */
  onApproachChange: (approach: ApplicationCreateFlowSignInApproach) => void;

  /**
   * Whether the embedded (native) sign-in approach is allowed. Browser-based SPAs
   * (public clients) must use the redirect-based approach, so the embedded option is
   * hidden for them. Defaults to true.
   */
  allowEmbeddedApproach?: boolean;

  /**
   * Callback function to broadcast whether this step is ready to proceed
   */
  onReadyChange?: (isReady: boolean) => void;

  /**
   * Optional array of available user types for selection
   */
  userTypes?: UserTypeListItem[];

  /**
   * Optional array of currently selected user type names
   */
  selectedUserTypes?: string[];

  /**
   * Optional callback function invoked when user type selection changes
   */
  onUserTypesChange?: (userTypes: string[]) => void;
}

/**
 * React component that renders the sign-in approach selection step in the
 * application creation onboarding flow.
 *
 * This component allows users to choose between two authentication approaches:
 * 1. Inbuilt - Uses product's hosted login pages for authentication
 * 2. Custom - Uses native/custom UI with product as the authentication API
 *
 * The component displays two selectable cards with radio buttons, providing
 * clear descriptions of each approach. It automatically marks the step as ready
 * since a default selection is always available.
 *
 * @param props - The component props
 * @param props.selectedApproach - The currently selected sign-in approach (inbuilt or custom)
 * @param props.onApproachChange - Callback invoked when user selects a different approach
 * @param props.onReadyChange - Optional callback to notify parent that step is ready to proceed
 *
 * @returns JSX element displaying the sign-in approach selection interface
 *
 * @example
 * ```tsx
 * import ConfigureExperience from './ConfigureExperience';
 *
 * function OnboardingFlow() {
 *   const [approach, setApproach] = useState(
 *     ApplicationCreateFlowSignInApproach.INBUILT
 *   );
 *
 *   return (
 *     <ConfigureExperience
 *       selectedApproach={approach}
 *       onApproachChange={setApproach}
 *       onReadyChange={(isReady) => console.log('Step ready:', isReady)}
 *     />
 *   );
 * }
 * ```
 *
 * @public
 */
export default function ConfigureExperience({
  selectedApproach,
  onApproachChange,
  allowEmbeddedApproach = true,
  onReadyChange = undefined,
  userTypes = [],
  selectedUserTypes = [],
  onUserTypesChange = undefined,
}: ConfigureExperienceProps): JSX.Element {
  const {t} = useTranslation();
  const {config} = useConfig();

  const {brand} = config;
  const {product_name: productName} = brand || {};

  // Determine if user types should be shown (2 or more available)
  const showUserTypes = userTypes.length >= 2;

  /**
   * Broadcast readiness based on user type selection if applicable.
   * - If no user types or only 1 user type: always ready
   * - If 2+ user types: ready only when at least one is selected
   */
  useEffect((): void => {
    const isReady = !showUserTypes || selectedUserTypes.length > 0;
    onReadyChange?.(isReady);
  }, [onReadyChange, showUserTypes, selectedUserTypes.length]);

  /**
   * Auto-select the first user type by default if none are selected
   */
  useEffect((): void => {
    if (showUserTypes && selectedUserTypes.length === 0 && userTypes.length > 0 && onUserTypesChange) {
      onUserTypesChange([userTypes[0].name]);
    }
  }, [showUserTypes, userTypes, selectedUserTypes.length, onUserTypesChange]);

  /**
   * Reset to the redirect-based approach if the embedded approach is not allowed
   * but is currently selected (e.g. after switching to a public-client SPA template).
   */
  useEffect((): void => {
    if (!allowEmbeddedApproach && selectedApproach === ApplicationCreateFlowSignInApproach.EMBEDDED) {
      onApproachChange(ApplicationCreateFlowSignInApproach.INBUILT);
    }
  }, [allowEmbeddedApproach, selectedApproach, onApproachChange]);

  const handleApproachChange = (event: ChangeEvent<HTMLInputElement>): void => {
    onApproachChange(event.target.value as ApplicationCreateFlowSignInApproach);
  };

  return (
    <Stack direction="column" spacing={3} data-testid="application-configure-experience">
      <Stack direction="column" spacing={1}>
        <Typography variant="h1" gutterBottom>
          {t('applications:onboarding.configure.experience.title')}
        </Typography>
        <Typography variant="subtitle1" gutterBottom>
          {t(
            showUserTypes
              ? 'applications:onboarding.configure.experience.subtitle'
              : 'applications:onboarding.configure.experience.subtitleWithoutUserTypes',
          )}
        </Typography>
      </Stack>

      <Stack direction="column" spacing={2} sx={{mt: 2}}>
        <Stack direction="column" spacing={1}>
          <Typography variant="h6">{t('applications:onboarding.configure.experience.approach.title')}</Typography>
          <Typography variant="body2" color="text.disabled" gutterBottom>
            {t('applications:onboarding.configure.experience.approach.subtitle')}
          </Typography>
        </Stack>
        <RadioGroup value={selectedApproach} onChange={handleApproachChange}>
          <Stack direction="column" spacing={2}>
            {/* Hosted Pages Option */}
            <Card variant="outlined" onClick={() => onApproachChange(ApplicationCreateFlowSignInApproach.INBUILT)}>
              <CardActionArea
                sx={{
                  height: '100%',
                  cursor: 'pointer',
                  border: 1,
                  borderColor:
                    selectedApproach === ApplicationCreateFlowSignInApproach.INBUILT ? 'primary.main' : 'divider',
                  transition: 'all 0.2s ease-in-out',
                  '&:hover': {
                    borderColor: 'primary.main',
                    bgcolor:
                      selectedApproach === ApplicationCreateFlowSignInApproach.INBUILT
                        ? 'action.selected'
                        : 'action.hover',
                  },
                }}
              >
                <CardContent>
                  <Stack direction="row" spacing={2} alignItems="flex-start">
                    <FormControlLabel
                      value={ApplicationCreateFlowSignInApproach.INBUILT}
                      control={<Radio />}
                      label=""
                      sx={{m: 0}}
                      onClick={(e) => e.stopPropagation()}
                    />
                    <Box sx={{flex: 1}}>
                      <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 1}}>
                        <ExternalLink size={20} />
                        <Typography variant="h6">
                          {t('applications:onboarding.configure.approach.inbuilt.title', {
                            product: productName,
                          })}
                        </Typography>
                      </Stack>
                      <Typography variant="body2" color="text.secondary">
                        {t('applications:onboarding.configure.approach.inbuilt.description', {
                          product: productName,
                        })}
                      </Typography>
                    </Box>
                  </Stack>
                </CardContent>
              </CardActionArea>
            </Card>

            {/* Native/Custom UI Option - hidden for public-client SPAs which must use redirect-based flows */}
            {allowEmbeddedApproach && (
              <Card variant="outlined" onClick={() => onApproachChange(ApplicationCreateFlowSignInApproach.EMBEDDED)}>
                <CardActionArea
                  sx={{
                    height: '100%',
                    cursor: 'pointer',
                    border: 1,
                    borderColor:
                      selectedApproach === ApplicationCreateFlowSignInApproach.EMBEDDED ? 'primary.main' : 'divider',
                    transition: 'all 0.2s ease-in-out',
                    '&:hover': {
                      borderColor: 'primary.main',
                      bgcolor:
                        selectedApproach === ApplicationCreateFlowSignInApproach.EMBEDDED
                          ? 'action.selected'
                          : 'action.hover',
                    },
                  }}
                >
                  <CardContent>
                    <Stack direction="row" spacing={2} alignItems="flex-start">
                      <FormControlLabel
                        value={ApplicationCreateFlowSignInApproach.EMBEDDED}
                        control={<Radio />}
                        label=""
                        sx={{m: 0}}
                        onClick={(e) => e.stopPropagation()}
                      />
                      <Box sx={{flex: 1}}>
                        <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 1}}>
                          <Code size={20} />
                          <Typography variant="h6">
                            {t('applications:onboarding.configure.approach.native.title', {
                              product: productName,
                            })}
                          </Typography>
                        </Stack>
                        <Typography variant="body2" color="text.secondary">
                          {t('applications:onboarding.configure.approach.native.description', {
                            product: productName,
                          })}
                        </Typography>
                      </Box>
                    </Stack>
                  </CardContent>
                </CardActionArea>
              </Card>
            )}
          </Stack>
        </RadioGroup>
      </Stack>

      {/* User Type Selection - Only show if there are 2 or more user types */}
      {showUserTypes && onUserTypesChange && (
        <Stack direction="column" spacing={2} sx={{mt: 2}}>
          <Stack direction="column" spacing={1}>
            <Typography variant="h6">
              {t('applications:onboarding.configure.experience.access.userTypes.title')}
            </Typography>
            <Typography variant="body2" color="text.disabled" gutterBottom>
              {t('applications:onboarding.configure.experience.access.userTypes.subtitle')}
            </Typography>
          </Stack>

          {/* Show card grid for less than 5 user types */}
          {userTypes.length < 5 ? (
            <Grid container spacing={2}>
              {userTypes.map((userType) => {
                const isSelected = selectedUserTypes.includes(userType.name);
                return (
                  <Grid size={{xs: 12, sm: 12, md: 12, lg: 6, xl: 3}} key={userType.id}>
                    <Card
                      variant="outlined"
                      onClick={() => {
                        const newSelection = isSelected
                          ? selectedUserTypes.filter((name) => name !== userType.name)
                          : [...selectedUserTypes, userType.name];
                        onUserTypesChange(newSelection);
                      }}
                      sx={{height: '100%'}}
                    >
                      <CardActionArea
                        sx={{
                          height: '100%',
                          cursor: 'pointer',
                          border: 1,
                          borderColor: isSelected ? 'primary.main' : 'divider',
                          transition: 'all 0.2s ease-in-out',
                          '&:hover': {
                            borderColor: 'primary.main',
                            bgcolor: isSelected ? 'action.hover' : 'action.hover',
                          },
                        }}
                      >
                        <CardContent>
                          <Stack direction="column" spacing={1} alignItems="center" sx={{py: 2}}>
                            <User size={40} />
                            <Typography variant="h6" textAlign="center">
                              {userType.name}
                            </Typography>
                          </Stack>
                        </CardContent>
                      </CardActionArea>
                    </Card>
                  </Grid>
                );
              })}
            </Grid>
          ) : (
            // Show autocomplete for 5 or more user types
            <FormControl fullWidth required>
              <Autocomplete
                multiple
                id="user-types-autocomplete"
                size="small"
                options={userTypes}
                getOptionLabel={(option) => option.name}
                value={userTypes.filter((ut: UserTypeListItem) => selectedUserTypes.includes(ut.name)) || []}
                onChange={(_event, newValue: UserTypeListItem[]): void => {
                  const userTypeNames: string[] = newValue.map((item: UserTypeListItem): string => item.name);
                  onUserTypesChange(userTypeNames);
                }}
                renderInput={(params) => (
                  <TextField
                    {...params}
                    placeholder={t('applications:onboarding.configure.details.userTypes.description')}
                    error={showUserTypes && selectedUserTypes.length === 0}
                    helperText={
                      showUserTypes && selectedUserTypes.length === 0
                        ? t('applications:onboarding.configure.details.userTypes.error')
                        : undefined
                    }
                  />
                )}
                isOptionEqualToValue={(option: UserTypeListItem, value: UserTypeListItem): boolean =>
                  option.name === value.name
                }
              />
            </FormControl>
          )}
        </Stack>
      )}
    </Stack>
  );
}
