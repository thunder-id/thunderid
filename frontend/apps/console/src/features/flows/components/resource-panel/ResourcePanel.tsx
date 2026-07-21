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

import {BuilderLayout, BuilderPanelHeader} from '@thunderid/components';
import {
  Accordion,
  AccordionDetails,
  AccordionSummary,
  Box,
  IconButton,
  InputAdornment,
  Stack,
  TextField,
  Typography,
} from '@wso2/oxygen-ui';
import {
  BoxesIcon,
  BoxIcon,
  ChevronDownIcon,
  CogIcon,
  SearchIcon,
  SearchXIcon,
  XIcon,
  ZapIcon,
} from '@wso2/oxygen-ui-icons-react';
import {memo, useCallback, useMemo, useState, type HTMLAttributes, type ReactElement, type ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import ResourcePanelDraggable from './ResourcePanelDraggable';
import useUIPanelState from '../../hooks/useUIPanelState';
import type {Resource, Resources} from '../../models/resources';
import filterResourcePanelItems from '../../utils/filterResourcePanelItems';
import toResourcePanelItems, {type ResourcePanelListItem} from '../../utils/toResourcePanelItems';

/**
 * Props interface of {@link ResourcePanel}
 */
export interface ResourcePanelPropsInterface extends HTMLAttributes<HTMLDivElement> {
  /**
   * Flow resources.
   */
  resources: Resources;
  /**
   * Whether the panel is open.
   * @defaultValue undefined
   */
  open?: boolean;
  /**
   * Callback to be triggered when a resource add button is clicked.
   * @param resource - Added resource.
   */
  onAdd: (resource: Resource) => void;
  /**
   * Flag to disable the panel.
   * @defaultValue false
   */
  disabled?: boolean;
  /**
   * Flow title to display.
   */
  flowTitle?: string;
  /**
   * Flow handle (URL-friendly identifier).
   */
  flowHandle?: string;
  /**
   * Callback to be triggered when flow title changes.
   */
  onFlowTitleChange?: (newTitle: string) => void;
  /**
   * Optional right-hand side panel content rendered via BuilderLayout.
   */
  rightPanel?: ReactNode;
}

interface ResourcePanelSection {
  id: string;
  icon: ReactElement;
  titleKey: string;
  titleFallback: string;
  descriptionKey: string;
  descriptionFallback: string;
  items: ResourcePanelListItem[];
}

/**
 * Flow builder resource panel that contains draggable components.
 *
 * @param props - Props injected to the component.
 * @returns The ResourcePanel component.
 */
function ResourcePanel({
  children,
  open = undefined,
  resources,
  onAdd,
  disabled = false,
  flowTitle = '',
  flowHandle = '',
  onFlowTitleChange = undefined,
  rightPanel = undefined,
  ...rest
}: ResourcePanelPropsInterface): ReactElement {
  const {t} = useTranslation();
  const {setIsResourcePanelOpen} = useUIPanelState();
  const [searchQuery, setSearchQuery] = useState<string>('');

  const handleTogglePanel = useCallback((): void => {
    setIsResourcePanelOpen((prev: boolean) => !prev);
  }, [setIsResourcePanelOpen]);

  const sections: ResourcePanelSection[] = useMemo(
    () => [
      {
        id: 'widgets',
        icon: <CogIcon size={16} />,
        titleKey: 'flows:core.resourcePanel.widgets.title',
        titleFallback: 'Widgets',
        descriptionKey: 'flows:core.resourcePanel.widgets.description',
        descriptionFallback: 'Ready-made blocks like social login, OTP, and passkey',
        items: toResourcePanelItems(resources.widgets, 'widgets'),
      },
      {
        id: 'steps',
        icon: <BoxIcon size={16} />,
        titleKey: 'flows:core.resourcePanel.steps.title',
        titleFallback: 'Steps',
        descriptionKey: 'flows:core.resourcePanel.steps.description',
        descriptionFallback: 'Screens and logic that shape your flow',
        items: toResourcePanelItems(resources.steps, 'steps'),
      },
      {
        id: 'components',
        icon: <BoxesIcon size={16} />,
        titleKey: 'flows:core.resourcePanel.components.title',
        titleFallback: 'Components',
        descriptionKey: 'flows:core.resourcePanel.components.description',
        descriptionFallback: 'Form fields, buttons, and display elements',
        items: toResourcePanelItems(resources.elements, 'components'),
      },
      {
        id: 'executors',
        icon: <ZapIcon size={16} />,
        titleKey: 'flows:core.resourcePanel.executors.title',
        titleFallback: 'Executors',
        descriptionKey: 'flows:core.resourcePanel.executors.description',
        descriptionFallback: 'Backend actions like verifying credentials or sending OTPs',
        items: toResourcePanelItems(resources.executors, 'executors'),
      },
    ],
    [resources],
  );

  const isSearching: boolean = searchQuery.trim().length > 0;

  const visibleSections: ResourcePanelSection[] = useMemo(() => {
    if (!isSearching) {
      return sections;
    }

    return sections
      .map((section: ResourcePanelSection) => ({
        ...section,
        items: filterResourcePanelItems(section.items, searchQuery),
      }))
      .filter((section: ResourcePanelSection) => section.items.length > 0);
  }, [sections, searchQuery, isSearching]);

  // Memoized so the element reference stays stable across unrelated parent re-renders
  // (e.g. node drag ticks) and React can skip reconciling the whole palette subtree.
  const panelContent = useMemo(
    () => (
      <>
        <BuilderPanelHeader
          title={flowTitle}
          handle={flowHandle}
          onPanelToggle={handleTogglePanel}
          onTitleChange={onFlowTitleChange}
          hidePanelTooltip={t('flows:core.resourcePanel.hideResources')}
          editTitleTooltip={t('flows:core.headerPanel.editTitle')}
          saveTitleTooltip={t('flows:core.headerPanel.saveTitle')}
          cancelEditTooltip={t('flows:core.headerPanel.cancelEdit')}
        />

        <Box sx={{pb: 1.5, flexShrink: 0}}>
          <TextField
            fullWidth
            size="small"
            value={searchQuery}
            onChange={(event) => setSearchQuery(event.target.value)}
            placeholder={t('flows:core.resourcePanel.search.placeholder', 'Search (e.g. MFA, social, consent)')}
            slotProps={{
              htmlInput: {
                'aria-label': t('flows:core.resourcePanel.search.placeholder', 'Search (e.g. MFA, social, consent)'),
                type: 'search',
              },
              input: {
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon size={16} />
                  </InputAdornment>
                ),
                endAdornment: isSearching ? (
                  <InputAdornment position="end">
                    <IconButton
                      size="small"
                      onClick={() => setSearchQuery('')}
                      aria-label={t('flows:core.resourcePanel.search.clear', 'Clear search')}
                    >
                      <XIcon size={14} />
                    </IconButton>
                  </InputAdornment>
                ) : undefined,
              },
            }}
          />
        </Box>

        {visibleSections.map((section: ResourcePanelSection) => (
          <Accordion
            key={`${section.id}-${isSearching}`}
            defaultExpanded={isSearching}
            square
            disableGutters
            sx={{
              backgroundColor: 'transparent',
              '&:before': {
                display: 'none',
              },
              overflow: 'hidden',
              flexShrink: 0,
            }}
          >
            <AccordionSummary
              expandIcon={<ChevronDownIcon size={14} />}
              aria-controls={`panel-${section.id}-content`}
              id={`panel-${section.id}-header`}
              sx={{
                minHeight: 48,
                '&.Mui-expanded': {
                  minHeight: 48,
                },
                '& .MuiAccordionSummary-content': {
                  margin: '12px 0',
                  gap: 1,
                },
              }}
              slotProps={{
                content: {
                  sx: {alignItems: 'center'},
                },
              }}
            >
              <Box component="span" display="inline-flex" alignItems="center" sx={{flexShrink: 0}}>
                {section.icon}
              </Box>
              <Stack direction="column">
                <Typography variant="subtitle2" fontWeight={600}>
                  {t(section.titleKey, section.titleFallback)}
                </Typography>
                <Typography variant="caption" color="text.secondary" sx={{lineHeight: 1.3}}>
                  {t(section.descriptionKey, section.descriptionFallback)}
                </Typography>
              </Stack>
            </AccordionSummary>
            <AccordionDetails sx={{pt: 0, pb: 2, px: 2}}>
              <Stack direction="column" spacing={1}>
                {section.items.map(({id, resource}: ResourcePanelListItem) => (
                  <ResourcePanelDraggable id={id} key={id} resource={resource} onAdd={onAdd} disabled={disabled} />
                ))}
              </Stack>
            </AccordionDetails>
          </Accordion>
        ))}

        {isSearching && visibleSections.length === 0 && (
          <Stack alignItems="center" spacing={1} sx={{px: 2, py: 4, color: 'text.secondary'}}>
            <SearchXIcon size={24} />
            <Typography variant="body2">
              {t('flows:core.resourcePanel.search.noResults', 'No matching resources')}
            </Typography>
            <Typography variant="caption" color="text.secondary" textAlign="center">
              {t(
                'flows:core.resourcePanel.search.noResultsHint',
                'Try a different keyword, such as "OTP", "Google", or "passkey"',
              )}
            </Typography>
          </Stack>
        )}
      </>
    ),
    [
      visibleSections,
      isSearching,
      searchQuery,
      flowTitle,
      flowHandle,
      onFlowTitleChange,
      handleTogglePanel,
      onAdd,
      disabled,
      t,
    ],
  );

  return (
    <BuilderLayout
      open={open}
      onPanelToggle={handleTogglePanel}
      expandTooltip={t('flows:core.resourcePanel.showResources')}
      panelContent={panelContent}
      panelPaperSx={{
        overflow: 'hidden auto',
        display: 'flex',
        flexDirection: 'column',
        borderRight: '1px solid',
        borderColor: 'divider',
      }}
      rightPanel={rightPanel}
      {...rest}
    >
      {children}
    </BuilderLayout>
  );
}

export default memo(ResourcePanel);
