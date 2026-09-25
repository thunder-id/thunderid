// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Card, CardContent, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {CircleCheck, KeyRound, Mail, MessagesSquare, ShieldCheck} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {type ConnectionType, ConnectionTypes} from '../../models/connection';

/**
 * Selectable option in the "Add custom connection" wizard's type step. `'trusted-idp'` is a
 * UI-only pseudo-type (not a backend /connections vendor route) for configuring a trust-only
 * OIDC connection through the dedicated trusted-issuer form rather than the generic
 * `ConnectionForm`.
 */
export type SelectableConnectionType = ConnectionType | 'trusted-idp';

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
    {
      type: ConnectionTypes.SMTP,
      labelKey: 'wizard.type.smtp.label',
      labelDefault: 'Email Provider (SMTP)',
      descriptionKey: 'wizard.type.smtp.description',
      descriptionDefault: 'Deliver email through your own SMTP server.',
      tagKey: 'wizard.type.smtp.tag',
      tagDefault: 'Message sender · Email',
      icon: <Mail size={28} />,
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

      <Box sx={{display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 2}}>
        {options.map((option) => {
          const isSelected: boolean = selectedType === option.type;
          return (
            <Card
              key={option.type}
              variant="outlined"
              role="button"
              tabIndex={option.comingSoon ? -1 : 0}
              aria-pressed={isSelected}
              aria-disabled={option.comingSoon}
              data-testid={`connection-type-option-${option.type}`}
              onClick={option.comingSoon ? undefined : () => onSelect(option.type)}
              onKeyDown={(e) => {
                if (!option.comingSoon && (e.key === 'Enter' || e.key === ' ')) {
                  e.preventDefault();
                  onSelect(option.type);
                }
              }}
              sx={{
                cursor: option.comingSoon ? 'not-allowed' : 'pointer',
                opacity: option.comingSoon ? 0.6 : 1,
                borderColor: isSelected ? 'primary.main' : 'divider',
                transition: 'border-color 0.15s',
                '&:hover': option.comingSoon ? {} : {borderColor: 'primary.main'},
                '&:focus-visible': option.comingSoon ? {} : {outline: 'none', borderColor: 'primary.main'},
              }}
            >
              <CardContent sx={{p: 2.5, '&:last-child': {pb: 2.5}}}>
                <Stack direction="column" spacing={2}>
                  <Stack direction="row" justifyContent="space-between" alignItems="flex-start">
                    <Box
                      sx={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        width: 48,
                        height: 48,
                        color: isSelected ? 'primary.main' : 'text.secondary',
                      }}
                    >
                      {option.icon}
                    </Box>
                    {option.comingSoon ? (
                      <Chip size="small" label={t('card.comingSoon', 'Coming soon')} />
                    ) : (
                      isSelected && <CircleCheck size={20} color="var(--mui-palette-primary-main)" />
                    )}
                  </Stack>
                  <Stack direction="column" spacing={0.75}>
                    <Typography variant="subtitle1" sx={{fontWeight: 600, lineHeight: 1.3}}>
                      {t(option.labelKey, option.labelDefault)}
                    </Typography>
                    <Typography variant="body2" color="text.secondary" sx={{lineHeight: 1.5}}>
                      {t(option.descriptionKey, option.descriptionDefault)}
                    </Typography>
                  </Stack>
                  <Stack direction="row" spacing={0.75} flexWrap="wrap">
                    {t(option.tagKey, option.tagDefault)
                      .split(' · ')
                      .map((tag) => (
                        <Typography
                          key={tag}
                          variant="caption"
                          color="text.disabled"
                          sx={{fontWeight: 500, letterSpacing: 0.2}}
                        >
                          #{tag.toLocaleLowerCase()}
                        </Typography>
                      ))}
                  </Stack>
                </Stack>
              </CardContent>
            </Card>
          );
        })}
      </Box>
    </Stack>
  );
}
