// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {NameSuggestion, ResourceAvatar} from '@thunderid/components';
import {buildAvatarSpec, pickAnonymousEntityName} from '@thunderid/react';
import {Typography, Stack, TextField, FormControl, FormLabel} from '@wso2/oxygen-ui';
import type {ChangeEvent, JSX} from 'react';
import {useEffect} from 'react';
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
   * URL or emoji of the currently selected application logo.
   */
  appLogo: string | null;

  /**
   * Callback function when a logo is selected
   */
  onLogoSelect: (logoUrl: string) => void;

  /**
   * Callback function to broadcast whether this step is ready to proceed
   */
  onReadyChange?: (isReady: boolean) => void;

  /**
   * Whether to render this step's own heading. Set to false when composed inside a larger step
   * (e.g. the Details step) that already renders its own title.
   * @defaultValue true
   */
  showTitle?: boolean;

  /**
   * Application names already in use, so a duplicate can be flagged before submission
   */
  existingAppNames?: string[];
}

/**
 * React component that renders the application name input step in the
 * application creation onboarding flow.
 *
 * This component pairs a compact logo picker with the application name field, since naming
 * an application and giving it an icon are the same branding decision. Instead of a whole
 * row of name suggestions competing for attention, a single random suggestion is offered
 * inline (reshuffled on demand) below the field. The step is marked as ready when a
 * non-empty name is provided.
 *
 * @param props - The component props
 * @param props.appName - The current application name value
 * @param props.onAppNameChange - Callback invoked when the name is changed
 * @param props.appLogo - The current application logo value
 * @param props.onLogoSelect - Callback invoked when the logo is changed
 * @param props.onReadyChange - Optional callback to notify parent of step readiness
 *
 * @returns JSX element displaying the application name configuration interface
 *
 * @example
 * ```tsx
 * import ConfigureName from './ConfigureName';
 *
 * function OnboardingFlow() {
 *   const [name, setName] = useState('');
 *   const [logo, setLogo] = useState<string | null>(null);
 *   const [isReady, setIsReady] = useState(false);
 *
 *   return (
 *     <ConfigureName
 *       appName={name}
 *       onAppNameChange={setName}
 *       appLogo={logo}
 *       onLogoSelect={setLogo}
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
  appLogo,
  onLogoSelect,
  onReadyChange = undefined,
  showTitle = true,
  existingAppNames = [],
}: ConfigureNameProps): JSX.Element {
  const {t} = useTranslation();

  // Exact match, mirroring the server's uniqueness check.
  const isDuplicateName: boolean = appName !== '' && existingAppNames.includes(appName);

  /**
   * Default to a generated entity avatar the first time this step is reached, so the logo
   * picker never opens on a blank tile.
   */
  useEffect((): void => {
    if (!appLogo) {
      onLogoSelect(
        buildAvatarSpec({
          colors: 0,
          content: pickAnonymousEntityName(appName),
          shape: 'rounded',
          variant: 'anonymous_entity',
        }),
      );
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /**
   * Broadcast readiness whenever appName changes.
   */
  useEffect((): void => {
    const isReady: boolean = appName.trim().length > 0 && !isDuplicateName;
    if (onReadyChange) {
      onReadyChange(isReady);
    }
  }, [appName, isDuplicateName, onReadyChange]);

  return (
    <Stack direction="column" spacing={4} data-testid="application-configure-name">
      {showTitle && (
        <Typography variant="h1" gutterBottom>
          {t('applications:onboarding.configure.name.title')}
        </Typography>
      )}

      <FormControl fullWidth required>
        <FormLabel htmlFor="app-name-input">{t('applications:onboarding.configure.name.fieldLabel')}</FormLabel>
        <Stack direction="row" spacing={1.5} alignItems="flex-start">
          <ResourceAvatar
            editable
            variant="rounded"
            size={64}
            value={appLogo ?? ''}
            seedText={appName}
            supportedShapes={['rounded']}
            onSelect={onLogoSelect}
            editAriaLabel={t('applications:onboarding.configure.name.logoAriaLabel')}
          />
          <Stack direction="column" spacing={0.75} sx={{flex: 1, minWidth: 0}}>
            <TextField
              fullWidth
              id="app-name-input"
              value={appName}
              onChange={(e: ChangeEvent<HTMLInputElement>): void => onAppNameChange(e.target.value)}
              placeholder={t('applications:onboarding.configure.name.placeholder')}
              error={isDuplicateName}
              helperText={isDuplicateName ? t('applications:errors.APP-1020') : undefined}
              inputProps={{
                'data-testid': 'app-name-input',
              }}
            />

            <NameSuggestion onSelect={onAppNameChange} />
          </Stack>
        </Stack>
      </FormControl>
    </Stack>
  );
}
