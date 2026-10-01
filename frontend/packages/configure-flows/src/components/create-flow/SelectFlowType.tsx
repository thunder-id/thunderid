// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, Card, CardContent, Stack, Typography} from '@wso2/oxygen-ui';
import {KeyRound, Lock, LogOut, UserCog, UserPlus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {FlowType} from '../../models/flows';

interface SelectFlowTypeProps {
  selectedType: string | null;
  onTypeChange: (type: string) => void;
  onReadyChange: (isReady: boolean) => void;
}

interface FlowTypeOption {
  type: string;
  labelKey: string;
  labelDefault: string;
  descriptionKey: string;
  descriptionDefault: string;
  icon: JSX.Element;
}

export default function SelectFlowType({selectedType, onTypeChange, onReadyChange}: SelectFlowTypeProps): JSX.Element {
  const {t} = useTranslation();

  const options: FlowTypeOption[] = [
    {
      type: FlowType.AUTHENTICATION,
      labelKey: 'flows:create.type.signin.label',
      labelDefault: 'Sign-in',
      descriptionKey: 'flows:create.type.signin.description',
      descriptionDefault: 'Authenticate users with passwords, passkeys, or social providers',
      icon: <KeyRound size={28} />,
    },
    {
      type: FlowType.REGISTRATION,
      labelKey: 'flows:create.type.signup.label',
      labelDefault: 'Self Sign-up',
      descriptionKey: 'flows:create.type.signup.description',
      descriptionDefault: 'Let users register themselves with your application',
      icon: <UserPlus size={28} />,
    },
    {
      type: FlowType.RECOVERY,
      labelKey: 'flows:create.type.recovery.label',
      labelDefault: 'Password Recovery',
      descriptionKey: 'flows:create.type.recovery.description',
      descriptionDefault: 'Let users recover their password or account',
      icon: <Lock size={28} />,
    },
    {
      type: FlowType.SIGNOUT,
      labelKey: 'flows:create.type.signout.label',
      labelDefault: 'Sign Out',
      descriptionKey: 'flows:create.type.signout.description',
      descriptionDefault: 'Confirm and terminate an established SSO session',
      icon: <LogOut size={28} />,
    },
    {
      type: FlowType.ADMINISTRATION,
      labelKey: 'flows:create.type.administration.label',
      labelDefault: 'Administration',
      descriptionKey: 'flows:create.type.administration.description',
      descriptionDefault: 'Perform authenticated administrative and security operations',
      icon: <UserCog size={28} />,
    },
  ];

  const handleSelect = (type: string): void => {
    onTypeChange(type);
    onReadyChange(true);
  };

  return (
    <Stack direction="column" spacing={4} data-testid="select-flow-type">
      <Typography variant="h1" gutterBottom>
        {t('flows:create.type.title', 'What kind of flow do you want to create?')}
      </Typography>
      <Box
        sx={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
          maxWidth: 1040,
          gap: 2,
          mt: 3,
        }}
      >
        {options.map((option) => {
          const isSelected = selectedType === option.type;
          return (
            <Card
              key={option.type}
              variant="outlined"
              role="button"
              tabIndex={0}
              aria-pressed={isSelected}
              onClick={() => handleSelect(option.type)}
              onKeyDown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  handleSelect(option.type);
                }
              }}
              sx={{
                cursor: 'pointer',
                borderColor: isSelected ? 'primary.main' : 'divider',
                bgcolor: isSelected ? 'action.selected' : undefined,
                transition: 'border-color 0.15s',
                '&:hover': {borderColor: 'primary.main'},
                '&:focus-visible': {outline: 'none', borderColor: 'primary.main'},
              }}
            >
              <CardContent sx={{py: 2, px: 2}}>
                <Stack direction="column" spacing={1.5} alignItems="flex-start">
                  <Box sx={{color: isSelected ? 'primary.main' : 'text.secondary'}}>{option.icon}</Box>
                  <Stack direction="column" spacing={0.5}>
                    <Typography variant="subtitle1" sx={{fontWeight: 500}}>
                      {t(option.labelKey, option.labelDefault)}
                    </Typography>
                    <Typography variant="body2" color="text.secondary">
                      {t(option.descriptionKey, option.descriptionDefault)}
                    </Typography>
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
