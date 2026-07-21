/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {generateRandomHumanReadableIdentifiers} from '@thunderid/utils';
import {Box, Typography, Stack, TextField, Chip, FormControl, FormLabel, useTheme} from '@wso2/oxygen-ui';
import {Lightbulb} from '@wso2/oxygen-ui-icons-react';
import type {ChangeEvent, JSX} from 'react';
import {useMemo, useEffect} from 'react';
import {useTranslation} from 'react-i18next';

/**
 * Props for the {@link ConfigureName} component.
 *
 * @public
 */
export interface ConfigureNameProps {
  /**
   * The current application name
   */
  appName: string;

  /**
   * Callback function when the application name changes
   */
  onAppNameChange: (name: string) => void;

  /**
   * Callback function to broadcast whether this step is ready to proceed
   */
  onReadyChange?: (isReady: boolean) => void;

  /**
   * Application names already in use, so a duplicate can be flagged before submission
   */
  existingAppNames?: string[];
}

/**
 * React component that renders the application name input step in the
 * application creation onboarding flow.
 *
 * This component provides a text field for users to enter their application name,
 * along with AI-generated name suggestions displayed as clickable chips. Users can
 * either type a custom name or select from the suggestions. The step is marked as
 * ready when a non-empty name is provided.
 *
 * The component generates random application name suggestions on mount and displays
 * them with helpful context about the naming purpose.
 *
 * @param props - The component props
 * @param props.appName - The current application name value
 * @param props.onAppNameChange - Callback invoked when the name is changed
 * @param props.onReadyChange - Optional callback to notify parent of step readiness
 * @param props.existingAppNames - Optional list of application names already in use
 *
 * @returns JSX element displaying the application name configuration interface
 *
 * @example
 * ```tsx
 * import ConfigureName from './ConfigureName';
 *
 * function OnboardingFlow() {
 *   const [name, setName] = useState('');
 *   const [isReady, setIsReady] = useState(false);
 *
 *   return (
 *     <ConfigureName
 *       appName={name}
 *       onAppNameChange={setName}
 *       onReadyChange={setIsReady}
 *     />
 *   );
 * }
 * ```
 *
 * @public
 */
export default function ConfigureName({
  appName,
  onAppNameChange,
  onReadyChange = undefined,
  existingAppNames = [],
}: ConfigureNameProps): JSX.Element {
  const {t} = useTranslation();
  const theme = useTheme();

  const appNameSuggestions: string[] = useMemo((): string[] => generateRandomHumanReadableIdentifiers(), []);

  // Exact match, mirroring the server's uniqueness check.
  const isDuplicateName: boolean = appName !== '' && existingAppNames.includes(appName);

  /**
   * Broadcast readiness whenever appName changes.
   */
  useEffect((): void => {
    const isReady: boolean = appName.trim().length > 0 && !isDuplicateName;
    if (onReadyChange) {
      onReadyChange(isReady);
    }
  }, [appName, isDuplicateName, onReadyChange]);

  const handleNameSuggestionClick = (suggestion: string): void => {
    onAppNameChange(suggestion);
  };

  return (
    <Stack direction="column" spacing={4} data-testid="application-configure-name">
      <Typography variant="h1" gutterBottom>
        {t('applications:onboarding.configure.name.title')}
      </Typography>

      <FormControl fullWidth required>
        <FormLabel htmlFor="app-name-input">{t('applications:onboarding.configure.name.fieldLabel')}</FormLabel>
        <TextField
          fullWidth
          id="app-name-input"
          value={appName}
          onChange={(e: ChangeEvent<HTMLInputElement>): void => onAppNameChange(e.target.value)}
          placeholder={t('applications:onboarding.configure.name.placeholder')}
          error={isDuplicateName}
          helperText={isDuplicateName ? t('applications:onboarding.configure.name.duplicate') : undefined}
          inputProps={{
            'data-testid': 'app-name-input',
          }}
        />
      </FormControl>

      {/* Name suggestions */}
      <Stack direction="column" spacing={2}>
        <Stack direction="row" alignItems="center" spacing={1}>
          <Lightbulb size={20} color={theme.vars?.palette.warning.main} />
          <Typography variant="body2" color="text.secondary">
            {t('applications:onboarding.configure.name.suggestions.label')}
          </Typography>
        </Stack>
        <Box sx={{display: 'flex', flexWrap: 'wrap', gap: 1}}>
          {appNameSuggestions.map(
            (suggestion: string): JSX.Element => (
              <Chip
                key={suggestion}
                label={suggestion}
                onClick={(): void => handleNameSuggestionClick(suggestion)}
                variant="outlined"
                clickable
                sx={{
                  '&:hover': {
                    bgcolor: 'primary.main',
                    color: 'text.primary',
                    borderColor: 'primary.main',
                  },
                }}
              />
            ),
          )}
        </Box>
      </Stack>
    </Stack>
  );
}
