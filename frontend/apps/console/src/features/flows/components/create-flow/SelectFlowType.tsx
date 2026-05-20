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

import {Box, Card, CardActionArea, CardContent, Stack, Typography} from '@wso2/oxygen-ui';
import {KeyRound, Lock, UserPlus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {FlowType} from '../../models/flows';

interface SelectFlowTypeProps {
  selectedType: string | null;
  onTypeChange: (type: string) => void;
  onReadyChange: (ready: boolean) => void;
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
          gridTemplateColumns: 'repeat(3, 1fr)',
          maxWidth: 780,
          gap: 2,
          mt: 3,
        }}
      >
        {options.map((option) => {
          const isSelected = selectedType === option.type;
          return (
            <Card key={option.type} variant="outlined">
              <CardActionArea
                onClick={() => handleSelect(option.type)}
                sx={{
                  height: '100%',
                  border: 1,
                  borderColor: isSelected ? 'primary.main' : 'divider',
                  transition: 'all 0.2s ease-in-out',
                  '&:hover': {
                    borderColor: 'primary.main',
                    bgcolor: isSelected ? 'action.selected' : 'action.hover',
                  },
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
              </CardActionArea>
            </Card>
          );
        })}
      </Box>
    </Stack>
  );
}
