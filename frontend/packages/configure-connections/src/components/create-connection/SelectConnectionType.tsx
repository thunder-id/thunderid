// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Stack, Typography} from '@wso2/oxygen-ui';
import {KeyRound, MessagesSquare, ServerCog, ShieldCheck} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import SelectableConnectionCard from './SelectableConnectionCard';
import {POLICY_DECISION_POINT_TYPE} from '../../constants/connection-wizard';
import {type ConnectionType, ConnectionTypes} from '../../models/connection';

/**
 * Selectable option in the "Add custom connection" wizard's type step. `'trusted-idp'` is a
 * UI-only pseudo-type (not a backend /connections vendor route) for configuring a trust-only
 * OIDC connection through the dedicated trusted-issuer form rather than the generic
 * `ConnectionForm`.
 */
export type SelectableConnectionType = ConnectionType | 'trusted-idp' | typeof POLICY_DECISION_POINT_TYPE;

interface SelectConnectionTypeProps {
  selectedType: SelectableConnectionType | null;
  onSelect: (type: SelectableConnectionType) => void;
}

interface TypeOption {
  type: SelectableConnectionType;
  labelKey: string;
  labelDefault: string;
  descriptionKey: string;
  descriptionDefault: string;
  tagKey: string;
  tagDefault: string;
  icon: JSX.Element;
  comingSoon: boolean;
}

export default function SelectConnectionType({selectedType, onSelect}: SelectConnectionTypeProps): JSX.Element {
  const {t} = useTranslation('connections');

  const options: TypeOption[] = [
    {
      type: ConnectionTypes.OIDC,
      labelKey: 'wizard.type.oidc.label',
      labelDefault: 'OpenID Connect Provider',
      descriptionKey: 'wizard.type.oidc.description',
      descriptionDefault: 'Connect any OpenID Connect identity provider.',
      tagKey: 'wizard.type.oidc.tag',
      tagDefault: 'Login provider · Enterprise',
      icon: <ShieldCheck size={28} />,
      comingSoon: false,
    },
    {
      type: ConnectionTypes.OAUTH,
      labelKey: 'wizard.type.oauth.label',
      labelDefault: 'OAuth 2 Provider',
      descriptionKey: 'wizard.type.oauth.description',
      descriptionDefault: 'Connect any OAuth 2 identity provider.',
      tagKey: 'wizard.type.oauth.tag',
      tagDefault: 'Login provider · Enterprise',
      icon: <KeyRound size={28} />,
      comingSoon: false,
    },
    {
      type: 'trusted-idp',
      labelKey: 'wizard.type.trustedIdp.label',
      labelDefault: 'Trusted Token Issuer',
      descriptionKey: 'wizard.type.trustedIdp.description',
      descriptionDefault: "Trust an external IdP's identity assertions and exchange them for access tokens.",
      tagKey: 'wizard.type.trustedIdp.tag',
      tagDefault: 'Token exchange · ID-JAG',
      icon: <ShieldCheck size={28} />,
      comingSoon: false,
    },
    {
      type: POLICY_DECISION_POINT_TYPE,
      labelKey: 'wizard.type.policyDecisionPoint.label',
      labelDefault: 'Policy Decision Point (PDP)',
      descriptionKey: 'wizard.type.policyDecisionPoint.description',
      descriptionDefault: 'Connect a policy decision point using a supported authorization protocol.',
      tagKey: 'wizard.type.policyDecisionPoint.tag',
      tagDefault: 'Authorization · PDP',
      icon: <ServerCog size={28} />,
      comingSoon: false,
    },
    {
      type: ConnectionTypes.SMS_GATEWAY,
      labelKey: 'wizard.type.sms.label',
      labelDefault: 'SMS gateway',
      descriptionKey: 'wizard.type.sms.description',
      descriptionDefault: 'Route SMS through your own HTTP gateway.',
      tagKey: 'wizard.type.sms.tag',
      tagDefault: 'Message sender · SMS',
      icon: <MessagesSquare size={28} />,
      comingSoon: false,
    },
  ];

  return (
    <Stack direction="column" spacing={3} data-testid="select-connection-type">
      <Stack direction="column" spacing={0.5}>
        <Typography variant="h1">{t('wizard.type.heading', 'What kind of connection do you want to add?')}</Typography>
        <Typography variant="body1" color="text.secondary">
          {t(
            'wizard.type.subheading',
            "Custom connections aren't in the vendor catalog. Pick the type of integration you want to wire up.",
          )}
        </Typography>
      </Stack>

      <Box sx={{display: 'grid', gridTemplateColumns: {xs: '1fr', md: 'repeat(2, 1fr)'}, gap: 2}}>
        {options.map((option) => {
          const isSelected: boolean = selectedType === option.type;
          return (
            <SelectableConnectionCard
              key={option.type}
              description={t(option.descriptionKey, option.descriptionDefault)}
              disabled={option.comingSoon}
              icon={option.icon}
              label={t(option.labelKey, option.labelDefault)}
              selected={isSelected}
              tags={t(option.tagKey, option.tagDefault).split(' · ')}
              testId={`connection-type-option-${option.type}`}
              onSelect={() => onSelect(option.type)}
            />
          );
        })}
      </Box>
    </Stack>
  );
}
