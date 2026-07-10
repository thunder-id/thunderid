/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {useLogger} from '@thunderid/logger/react';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  Card,
  CardActionArea,
  CardContent,
  Chip,
  Stack,
  Typography,
} from '@wso2/oxygen-ui';
import {
  ChevronDownIcon,
  Fingerprint,
  Globe,
  KeyRound,
  RotateCcw,
  ShieldCheck,
  Sparkles,
} from '@wso2/oxygen-ui-icons-react';
import {useMemo, type JSX, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useGetFlowsMeta from '../api/useGetFlowsMeta';
import {FlowType} from '../models/flows';
import type {FlowTemplate} from '../models/templates';

/**
 * Props interface of {@link CapabilityCatalog}
 */
export interface CapabilityCatalogProps {
  /**
   * Presentation variant. `full` renders a prominent gallery (e.g. as the flows
   * list empty state); `compact` renders a collapsed, expandable section.
   * @defaultValue 'full'
   */
  variant?: 'full' | 'compact';
}

interface CatalogCard {
  id: string;
  icon: ReactElement;
  titleKey: string;
  descriptionKey: string;
  /**
   * Template category to pre-filter in the create wizard.
   */
  category?: string;
  /**
   * Flow type to preselect in the create wizard.
   */
  flowType?: FlowType;
}

const CATALOG_CARDS: CatalogCard[] = [
  {
    id: 'passwords',
    icon: <KeyRound size={22} />,
    titleKey: 'flows:catalog.cards.passwords.title',
    descriptionKey: 'flows:catalog.cards.passwords.description',
    category: 'PASSWORD',
  },
  {
    id: 'socialLogin',
    icon: <Globe size={22} />,
    titleKey: 'flows:catalog.cards.socialLogin.title',
    descriptionKey: 'flows:catalog.cards.socialLogin.description',
    category: 'SOCIAL_LOGIN',
  },
  {
    id: 'mfa',
    icon: <ShieldCheck size={22} />,
    titleKey: 'flows:catalog.cards.mfa.title',
    descriptionKey: 'flows:catalog.cards.mfa.description',
    category: 'MFA',
  },
  {
    id: 'passwordless',
    icon: <Fingerprint size={22} />,
    titleKey: 'flows:catalog.cards.passwordless.title',
    descriptionKey: 'flows:catalog.cards.passwordless.description',
    category: 'PASSWORDLESS',
  },
  {
    id: 'recovery',
    icon: <RotateCcw size={22} />,
    titleKey: 'flows:catalog.cards.recovery.title',
    descriptionKey: 'flows:catalog.cards.recovery.description',
    flowType: FlowType.RECOVERY,
  },
];

/**
 * Showcases the capabilities available in the flow builder as a card gallery.
 *
 * Cards are derived from the flow template catalog and deep-link into the flow
 * create wizard with the matching template category or flow type preselected.
 *
 * @param props - Props injected to the component.
 * @returns The CapabilityCatalog component.
 */
export default function CapabilityCatalog({variant = 'full'}: CapabilityCatalogProps): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const logger = useLogger('CapabilityCatalog');
  const {data} = useGetFlowsMeta();

  const templateCounts: Record<string, number> = useMemo(() => {
    const templates = data.templates.filter((template: FlowTemplate) => template.type !== 'BLANK');

    return Object.fromEntries(
      CATALOG_CARDS.map((card: CatalogCard) => [
        card.id,
        templates.filter((template: FlowTemplate) =>
          card.category ? template.category === card.category : template.flowType === card.flowType,
        ).length,
      ]),
    );
  }, [data.templates]);

  const handleCardClick = (card: CatalogCard): void => {
    const params = card.category ? `?category=${card.category}` : `?flowType=${card.flowType}`;

    (async () => {
      await navigate(`/flows/create${params}`);
    })().catch((error: unknown) => {
      logger.error('Failed to navigate to flow create page', {card: card.id, error});
    });
  };

  const cardGrid = (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))',
        gap: 2,
      }}
    >
      {CATALOG_CARDS.map((card: CatalogCard) => (
        <Card key={card.id} variant="outlined" sx={{borderRadius: 2}} data-testid={`capability-card-${card.id}`}>
          <CardActionArea onClick={() => handleCardClick(card)} sx={{height: '100%'}}>
            <CardContent>
              <Stack direction="column" spacing={1.5} alignItems="flex-start">
                <Box
                  sx={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    width: 40,
                    height: 40,
                    borderRadius: 1.5,
                    bgcolor: 'action.selected',
                    color: 'primary.main',
                  }}
                >
                  {card.icon}
                </Box>
                <Typography variant="subtitle1" fontWeight={600}>
                  {t(card.titleKey)}
                </Typography>
                <Typography variant="body2" color="text.secondary">
                  {t(card.descriptionKey)}
                </Typography>
                <Chip
                  label={t('flows:catalog.templatesCount', {count: templateCounts[card.id]})}
                  size="small"
                  variant="outlined"
                  sx={{color: 'text.secondary'}}
                />
              </Stack>
            </CardContent>
          </CardActionArea>
        </Card>
      ))}
    </Box>
  );

  if (variant === 'compact') {
    return (
      <Accordion
        disableGutters
        variant="outlined"
        sx={{borderRadius: 2, '&:before': {display: 'none'}, mb: 2}}
        data-testid="capability-catalog-compact"
      >
        <AccordionSummary expandIcon={<ChevronDownIcon size={16} />}>
          <Stack direction="row" spacing={1} alignItems="center">
            <Sparkles size={16} />
            <Typography variant="subtitle2" fontWeight={600}>
              {t('flows:catalog.explore')}
            </Typography>
          </Stack>
        </AccordionSummary>
        <AccordionDetails>{cardGrid}</AccordionDetails>
      </Accordion>
    );
  }

  return (
    <Stack direction="column" spacing={1} sx={{py: 4}} data-testid="capability-catalog-full">
      <Typography variant="h5" fontWeight={600}>
        {t('flows:catalog.title')}
      </Typography>
      <Typography variant="body2" color="text.secondary" sx={{mb: 2}}>
        {t('flows:catalog.subtitle')}
      </Typography>
      {cardGrid}
    </Stack>
  );
}
