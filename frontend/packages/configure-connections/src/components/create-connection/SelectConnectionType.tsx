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

import {Box, Card, CardActionArea, CardContent, Chip, Stack, Typography} from '@wso2/oxygen-ui';
import {ArrowLeftRight, CircleCheck, KeyRound, LogIn, Send, ShieldCheck, Webhook} from '@wso2/oxygen-ui-icons-react';
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
  /**
   * Keys with a wired `customConfigureSteps` slot on the parent wizard (see
   * `ConnectionCreateWizardPage`). Options that require a custom configure step (e.g.
   * `'trusted-idp'`) are only rendered when their key is present here — the package can't let a
   * consumer select a type it has no way to complete.
   */
  customTypes?: string[];
}

interface TypeOption {
  type: SelectableConnectionType;
  labelKey: string;
  descriptionKey: string;
  tagKey: string;
  icon: JSX.Element;
  tagIcon: JSX.Element;
  comingSoon: boolean;
  /** Whether this option only works when the consumer supplies a matching `customConfigureSteps` entry. */
  requiresCustomStep?: boolean;
}

export default function SelectConnectionType({
  selectedType,
  onSelect,
  customTypes = [],
}: SelectConnectionTypeProps): JSX.Element {
  const {t} = useTranslation('connections');

  const allOptions: TypeOption[] = [
    {
      type: ConnectionTypes.OIDC,
      labelKey: 'wizard.type.oidc.label',
      descriptionKey: 'wizard.type.oidc.description',
      tagKey: 'wizard.type.oidc.tag',
      icon: <ShieldCheck size={28} />,
      tagIcon: <LogIn size={14} />,
      comingSoon: false,
    },
    {
      type: ConnectionTypes.OAUTH,
      labelKey: 'wizard.type.oauth.label',
      descriptionKey: 'wizard.type.oauth.description',
      tagKey: 'wizard.type.oauth.tag',
      icon: <KeyRound size={28} />,
      tagIcon: <LogIn size={14} />,
      comingSoon: false,
    },
    {
      type: 'trusted-idp',
      labelKey: 'wizard.type.trustedIdp.label',
      descriptionKey: 'wizard.type.trustedIdp.description',
      tagKey: 'wizard.type.trustedIdp.tag',
      icon: <ShieldCheck size={28} />,
      tagIcon: <ArrowLeftRight size={14} />,
      comingSoon: false,
      requiresCustomStep: true,
    },
    {
      // Backend support (/connections/sms-gateway) is wired; the console wizard for this
      // card is a follow-up, so it stays comingSoon here.
      type: 'custom-sms' as ConnectionType,
      labelKey: 'wizard.type.sms.label',
      descriptionKey: 'wizard.type.sms.description',
      tagKey: 'wizard.type.sms.tag',
      icon: <Webhook size={28} />,
      tagIcon: <Send size={14} />,
      comingSoon: true,
    },
  ];
  const options: TypeOption[] = allOptions.filter(
    (option) => !option.requiresCustomStep || customTypes.includes(option.type),
  );

  return (
    <Stack direction="column" spacing={1} data-testid="select-connection-type">
      <Typography variant="h1" gutterBottom>
        {t('wizard.type.heading')}
      </Typography>
      <Typography variant="subtitle1" gutterBottom>
        {t('wizard.type.subheading')}
      </Typography>

      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)', md: 'repeat(3, 1fr)', xl: 'repeat(4, 1fr)'},
          gap: 2,
          mt: 3,
        }}
      >
        {options.map((option) => {
          const isSelected: boolean = selectedType === option.type;
          return (
            <Card key={option.type} variant="outlined" sx={{opacity: option.comingSoon ? 0.6 : 1}}>
              <CardActionArea
                disabled={option.comingSoon}
                onClick={() => onSelect(option.type)}
                data-testid={`connection-type-option-${option.type}`}
                sx={{
                  height: '100%',
                  border: 1,
                  borderColor: isSelected ? 'primary.main' : 'divider',
                  transition: 'all 0.2s ease-in-out',
                  '&:hover': {borderColor: option.comingSoon ? 'divider' : 'primary.main'},
                }}
              >
                <CardContent sx={{p: 2.5}}>
                  <Stack direction="row" justifyContent="space-between" alignItems="flex-start">
                    <Box sx={{color: isSelected ? 'primary.main' : 'text.secondary'}}>{option.icon}</Box>
                    {option.comingSoon ? (
                      <Chip size="small" label={t('card.comingSoon')} />
                    ) : (
                      isSelected && <CircleCheck size={20} color="var(--mui-palette-primary-main)" />
                    )}
                  </Stack>
                  <Typography variant="h6" sx={{mt: 2}}>
                    {t(option.labelKey)}
                  </Typography>
                  <Typography variant="body2" color="text.secondary" sx={{mt: 0.5}}>
                    {t(option.descriptionKey)}
                  </Typography>
                  <Stack direction="row" spacing={0.5} alignItems="center" sx={{mt: 1.5, color: 'text.secondary'}}>
                    {option.tagIcon}
                    <Typography variant="caption" color="text.secondary">
                      {t(option.tagKey)}
                    </Typography>
                  </Stack>
                </CardContent>
              </CardActionArea>
            </Card>
          );
        })}
      </Box>
    </Stack>
  );
}
