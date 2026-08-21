// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Application, OAuth2Config} from '@thunderid/configure-applications';
import {Alert} from '@wso2/oxygen-ui';
import {Lock} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ClientAccessTokenSection from './ClientAccessTokenSection';
import EditTokenSettings from './EditTokenSettings';
import TokenConstants from '../../../constants/token-constants';
import {hasClientAccess, hasUserAccess, isOAuthTokenMode} from '../../../utils/oauth2Rules';
import TokenAudienceSelector, {type TokenAudienceOption} from '../../common/TokenAudienceSelector';

const CLIENT_AUDIENCE = 'application';
const USER_AUDIENCE = 'user';

interface EditTokenSettingsTabsProps {
  application: Application;
  editedApp: Partial<Application>;
  oauth2Config?: OAuth2Config;
  onFieldChange: (field: keyof Application, value: unknown) => void;
  onValidationChange?: (hasErrors: boolean) => void;
  sectionResetKey?: number;
}

/**
 * Token settings for an application, split by the audience a token is issued to: the application
 * itself (client_credentials) or a signed-in user. Both audiences stay selectable; one whose
 * settings do not apply shows a notice in place of them, so the notice is only ever visible to
 * someone looking at that audience.
 *
 * The User audience covers both token shapes: for an OAuth application it edits the access token's
 * user config, and for an application with no OAuth configuration (app-native sign-in) it edits the
 * assertion config, which is the only token config such an application has. EditTokenSettings picks
 * the right one from the same `isOAuthTokenMode` check used here.
 */
export default function EditTokenSettingsTabs({
  application,
  editedApp,
  oauth2Config = undefined,
  onFieldChange,
  onValidationChange = undefined,
  sectionResetKey = 0,
}: EditTokenSettingsTabsProps): JSX.Element {
  const {t} = useTranslation();

  const grantTypes = oauth2Config?.grantTypes;
  const isOAuthMode = isOAuthTokenMode(oauth2Config);
  const clientUnlocked = hasClientAccess(grantTypes);
  const userUnlocked = !isOAuthMode || hasUserAccess(grantTypes);

  // Opens on the first audience that has settings to show, so an application whose Application
  // audience does not apply is not greeted by a notice. Both remain selectable afterwards.
  const [audience, setAudience] = useState(clientUnlocked ? CLIENT_AUDIENCE : USER_AUDIENCE);
  const [clientTabHasError, setClientTabHasError] = useState(false);
  const [userTabHasError, setUserTabHasError] = useState(false);

  // A locked audience is never mounted, so its last reported error state must not keep blocking the
  // save bar.
  useEffect(() => {
    onValidationChange?.((clientUnlocked && clientTabHasError) || (userUnlocked && userTabHasError));
  }, [clientUnlocked, userUnlocked, clientTabHasError, userTabHasError, onValidationChange]);

  const audienceOptions: TokenAudienceOption[] = [
    {
      value: CLIENT_AUDIENCE,
      label: t('applications:edit.token.tabs.application', 'Application'),
      description: t('applications:edit.token.audience.application.description', 'M2M access token'),
      isLocked: !clientUnlocked,
    },
    {
      value: USER_AUDIENCE,
      label: t('applications:edit.token.tabs.user', 'User'),
      description: t('applications:edit.token.audience.user.description', 'Tokens for a signed-in user'),
      isLocked: !userUnlocked,
    },
  ];

  return (
    <TokenAudienceSelector
      title={t('applications:edit.token.audience.title', 'Issued to')}
      options={audienceOptions}
      value={audience}
      onChange={setAudience}
      footnote={t(
        'applications:edit.token.audience.footnote',
        'Attribute sets are configured independently for each audience.',
      )}
    >
      {audience === CLIENT_AUDIENCE && !clientUnlocked && (
        <Alert severity="info" icon={<Lock size={20} />}>
          {t(
            'applications:edit.token.clientLock.message',
            'This application does not receive tokens for itself, so there is nothing to configure here.',
          )}
        </Alert>
      )}
      {audience === USER_AUDIENCE && !userUnlocked && (
        <Alert severity="info" icon={<Lock size={20} />}>
          {t(
            'applications:edit.token.userLock.message',
            'This application does not receive tokens for signed-in users, so there is nothing to configure here.',
          )}
        </Alert>
      )}
      {audience === CLIENT_AUDIENCE && clientUnlocked && (
        <ClientAccessTokenSection
          key={sectionResetKey}
          oauth2Config={oauth2Config}
          inboundAuthConfig={application.inboundAuthConfig}
          onFieldChange={onFieldChange}
          availableAttributes={[...TokenConstants.CLIENT_TOKEN_OPTIONAL_CLAIMS]}
          disabled={application.isReadOnly ?? false}
          onValidationChange={setClientTabHasError}
          subjectValue={application.id}
          subjectType="application"
          copy={{
            attributesTitle: t('applications:edit.token.client.attributes.title', 'Access Token Attributes'),
            attributesDescription: t(
              'applications:edit.token.client.attributes.description',
              'Extra attributes to add to the access token this application receives for itself.',
            ),
            attributesLabel: t('applications:edit.token.client.attributes.label', 'Add or Remove Attributes'),
            attributesHint: t(
              'applications:edit.token.client.attributes.hint',
              "Click on attributes to add them to this application's access token.",
            ),
            attributesEmpty: t('applications:edit.token.client.attributes.empty', 'No attributes available.'),
            validityTitle: t('applications:edit.token.client.validity.title', 'Token Validity'),
            validityDescription: t(
              'applications:edit.token.client.validity.description',
              'How long this access token remains valid before expiration.',
            ),
            validityLabel: t('applications:edit.token.client.validity.label', 'Token Validity'),
            validityHint: t(
              'applications:edit.token.client.validity.hint',
              'Token validity period in seconds (e.g., 3600 for 1 hour).',
            ),
            validityError: t(
              'applications:edit.token.client.validity.error',
              'Enter a validity period of at least 1 second.',
            ),
          }}
        />
      )}
      {audience === USER_AUDIENCE && userUnlocked && (
        <EditTokenSettings
          application={application}
          editedApp={editedApp}
          oauth2Config={oauth2Config}
          sectionResetKey={sectionResetKey}
          onFieldChange={onFieldChange}
          onValidationChange={setUserTabHasError}
        />
      )}
    </TokenAudienceSelector>
  );
}
