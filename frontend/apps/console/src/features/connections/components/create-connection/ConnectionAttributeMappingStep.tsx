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

import {Box, Button, CircularProgress, Paper, Stack, Typography} from '@wso2/oxygen-ui';
import {ChevronLeft, Share2} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useState} from 'react';
import {useTranslation} from 'react-i18next';
import type {AttributeConfiguration} from '../../models/connection';
import AttributeMappingSection from '../AttributeMappingSection';

interface ConnectionAttributeMappingStepProps {
  vendorDisplayName: string;
  initialConfig?: AttributeConfiguration;
  onChange: (config: AttributeConfiguration | undefined, valid: boolean) => void;
  onBack: () => void;
  onCreate: () => void;
  isPending: boolean;
  createDisabled: boolean;
}

/**
 * Final wizard step shared by the branded configure wizard and the custom-connection wizard:
 * the "Map provider attributes" heading, the attribute-mapping card, and the Back / Skip / Create
 * button row. The create-button label switches to "Skip and Create" while no mappings exist.
 */
export default function ConnectionAttributeMappingStep({
  vendorDisplayName,
  initialConfig = undefined,
  onChange,
  onBack,
  onCreate,
  isPending,
  createDisabled,
}: ConnectionAttributeMappingStepProps): JSX.Element {
  const {t} = useTranslation('connections');
  const [hasMappings, setHasMappings] = useState<boolean>(Boolean(initialConfig?.userTypeAttributeMappings?.length));

  const handleChange = (config: AttributeConfiguration | undefined, valid: boolean): void => {
    setHasMappings(Boolean(config?.userTypeAttributeMappings?.length));
    onChange(config, valid);
  };

  return (
    <Stack direction="column" spacing={3}>
      <Stack direction="column" spacing={1}>
        <Typography variant="h4" fontWeight={700}>
          {t('attributeMapping.stepTitle')}
        </Typography>
        <Typography variant="body1" color="text.secondary">
          {t('attributeMapping.stepSubtitle', {vendor: vendorDisplayName})}
        </Typography>
      </Stack>

      <Paper variant="outlined" sx={{p: 3}}>
        <Stack direction="row" spacing={1.5} alignItems="flex-start" sx={{mb: 3}}>
          <Box
            sx={{
              width: 40,
              height: 40,
              borderRadius: 2,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              bgcolor: 'action.hover',
              flexShrink: 0,
            }}
          >
            <Share2 size={18} />
          </Box>
          <Box>
            <Typography variant="subtitle1" fontWeight={600}>
              {t('attributeMapping.title')}
            </Typography>
            <Typography variant="body2" color="text.secondary">
              {t('attributeMapping.cardDescription')}{' '}
              <Typography component="span" variant="body2" color="text.disabled">
                {t('attributeMapping.cardOptionalNote')}
              </Typography>
            </Typography>
          </Box>
        </Stack>

        <AttributeMappingSection initialConfig={initialConfig} onChange={handleChange} />
      </Paper>

      <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
        <Button variant="outlined" startIcon={<ChevronLeft size={16} />} onClick={onBack} disabled={isPending}>
          {t('common:actions.back')}
        </Button>
        <Stack direction="row" spacing={2} alignItems="center">
          {isPending && <CircularProgress size={20} />}
          <Button
            variant="contained"
            disabled={createDisabled || isPending}
            onClick={onCreate}
            data-testid="wizard-create"
          >
            {hasMappings ? t('form.actions.create') : t('attributeMapping.skipAndCreate')}
          </Button>
        </Stack>
      </Box>
    </Stack>
  );
}
