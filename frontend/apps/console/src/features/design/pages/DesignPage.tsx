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

import {useGetThemes, useGetLayouts, useCreateLayout} from '@thunderid/design';
import {Box, Button, Card, Grid, PageContent, PageTitle, Skeleton, Typography} from '@wso2/oxygen-ui';
import {ArrowUpRight, LayoutTemplate, Palette, Plus} from '@wso2/oxygen-ui-icons-react';
import {useState, useCallback, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ItemCard from '../components/common/ItemCard';
import SectionHeader from '../components/common/SectionHeader';
import LayoutPresetThumbnail, {type LayoutPresetVariant} from '../components/layouts/LayoutPresetThumbnail';
import ThemeDeleteDialog from '../components/themes/ThemeDeleteDialog';
import ThemeThumbnail from '../components/themes/ThemeThumbnail';
import DesignUIConstants from '../constants/design-ui-constants';

const LAYOUT_PRESET_IDS: LayoutPresetVariant[] = ['centered', 'split', 'fullscreen', 'popup'];

const LAYOUT_PRESET_KEY: Record<LayoutPresetVariant, string> = {
  centered: 'layouts.presets.centered.label',
  split: 'layouts.presets.split_screen.label',
  fullscreen: 'layouts.presets.full_screen.label',
  popup: 'layouts.presets.popup.label',
};

const LAYOUT_PRESET_DEFAULT: Record<LayoutPresetVariant, string> = {
  centered: 'Centered',
  split: 'Split Screen',
  fullscreen: 'Full Screen',
  popup: 'Popup',
};

export default function DesignPage(): JSX.Element {
  const {t} = useTranslation('design');
  const navigate = useNavigate();
  const {data: themesData, isLoading: themesLoading} = useGetThemes();
  const {data: layoutsData} = useGetLayouts();
  const {mutateAsync: createLayout} = useCreateLayout();

  const [showAllThemes, setShowAllThemes] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{id: string; name: string} | null>(null);

  // Build a map of handle → layoutId for API layouts
  const layoutIdByHandle = new Map((layoutsData?.layouts ?? []).map((l) => [l.handle, l.id]));

  const handleLayoutClick = useCallback(
    async (presetId: LayoutPresetVariant, existingLayoutId?: string) => {
      if (existingLayoutId) {
        await navigate(`/design/layouts/${existingLayoutId}`);
        return;
      }

      // Create a new layout with default config when none exists yet
      const created = await createLayout({
        handle: presetId,
        displayName: LAYOUT_PRESET_DEFAULT[presetId],
        layout: {},
      });
      await navigate(`/design/layouts/${created.id}`);
    },
    [navigate, createLayout],
  );

  const allThemes = themesData?.themes ?? [];
  const visibleThemes = showAllThemes ? allThemes : allThemes.slice(0, DesignUIConstants.INITIAL_LIMIT);

  const skeletonCount = 4;

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('page.title', 'Design')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('page.subtitle', 'Create, customize, and manage visual themes & layouts for your applications.')}
        </PageTitle.SubHeader>
      </PageTitle>

      <Box>
        {/* ── Themes section ─────────────────────────────────────────────── */}
        <SectionHeader
          title={t('themes.section.title', 'Themes')}
          count={allThemes.length}
          icon={<Palette size={18} />}
          action={
            <Button
              variant="contained"
              size="small"
              startIcon={<Plus size={16} />}
              onClick={() => {
                (async () => {
                  await navigate('/design/themes/create');
                })().catch(() => {
                  // Ignore navigation errors
                });
              }}
            >
              {t('themes.actions.add.label', 'Add Theme')}
            </Button>
          }
        />

        <Grid container spacing={2} sx={{mb: 5}}>
          {themesLoading
            ? Array.from({length: skeletonCount}, (_, i) => `theme-skeleton-${i}`).map((key) => (
                <Grid key={key} size={{xs: 6, sm: 4, md: 3, lg: 2}}>
                  <Skeleton variant="rounded" sx={{aspectRatio: '4/3', height: 'auto', borderRadius: 2}} />
                </Grid>
              ))
            : [
                ...visibleThemes.map((theme) => (
                  <Grid key={theme.id} size={{xs: 6, sm: 4, md: 3, lg: 2}}>
                    <ItemCard
                      thumbnail={<ThemeThumbnail theme={theme} />}
                      name={theme.displayName}
                      isReadOnly={theme.isReadOnly}
                      onClick={() => {
                        (async () => {
                          await navigate(`/design/themes/${theme.id}`);
                        })().catch(() => {
                          // Ignore navigation errors
                        });
                      }}
                      onDelete={
                        theme.isReadOnly ? undefined : () => setDeleteTarget({id: theme.id, name: theme.displayName})
                      }
                    />
                  </Grid>
                )),
                ...(!showAllThemes && allThemes.length > DesignUIConstants.INITIAL_LIMIT
                  ? [
                      <Grid key="show-more" size={{xs: 6, sm: 4, md: 3, lg: 2}}>
                        <Box
                          onClick={() => setShowAllThemes(true)}
                          sx={{
                            cursor: 'pointer',
                            borderRadius: 1,
                            border: '1.5px dashed',
                            borderColor: 'divider',
                            aspectRatio: '4/3',
                            display: 'flex',
                            flexDirection: 'column',
                            alignItems: 'center',
                            justifyContent: 'center',
                            gap: 0.75,
                            color: 'text.secondary',
                            transition: 'all 0.18s ease',
                            width: '100%',
                            height: '100%',
                            '&:hover': {borderColor: 'primary.main', color: 'primary.main', bgcolor: 'primary.50'},
                          }}
                        >
                          <ArrowUpRight size={20} />
                          <Typography variant="caption" sx={{fontSize: '0.75rem', fontWeight: 500}}>
                            {t('themes.show_more.label', 'Show {{count}} more', {
                              count: allThemes.length - DesignUIConstants.INITIAL_LIMIT,
                            })}
                          </Typography>
                        </Box>
                      </Grid>,
                    ]
                  : []),
              ]}
        </Grid>

        {!themesLoading && allThemes.length === 0 && (
          <Box sx={{mb: 5, py: 6, textAlign: 'center', color: 'text.secondary'}}>
            <Palette size={32} style={{opacity: 0.3, marginBottom: 8}} />
            <Typography variant="body2">{t('themes.empty_state.message', 'No themes yet')}</Typography>
          </Box>
        )}
      </Box>

      <Box>
        {/* ── Layouts section ────────────────────────────────────────────── */}
        <SectionHeader
          title={t('layouts.section.title', 'Layouts')}
          count={LAYOUT_PRESET_IDS.length}
          icon={<LayoutTemplate size={18} />}
        />

        <Grid container spacing={2}>
          {LAYOUT_PRESET_IDS.map((id) => {
            const apiLayoutId = layoutIdByHandle.get(id);
            const isEnabled = id === 'centered';

            return (
              <Grid key={id} size={{xs: 6, sm: 4, md: 3, lg: 2}}>
                <Card
                  sx={{
                    cursor: isEnabled ? 'pointer' : 'default',
                    opacity: isEnabled ? 1 : 0.72,
                    pointerEvents: isEnabled ? 'auto' : 'none',
                    transition: 'box-shadow 0.15s ease',
                    '&:hover': isEnabled ? {boxShadow: 4} : undefined,
                  }}
                  {...(isEnabled
                    ? {
                        role: 'button',
                        tabIndex: 0,
                        onClick: () => {
                          handleLayoutClick(id, apiLayoutId).catch(() => {
                            /* no-op */
                          });
                        },
                        onKeyDown: (e: React.KeyboardEvent) => {
                          if (e.key === 'Enter' || e.key === ' ') {
                            e.preventDefault();
                            handleLayoutClick(id, apiLayoutId).catch(() => {
                              /* no-op */
                            });
                          }
                        },
                      }
                    : {})}
                >
                  <Box sx={{aspectRatio: '4/3', overflow: 'hidden', position: 'relative'}}>
                    <Box sx={{width: '100%', height: '100%', filter: isEnabled ? 'none' : 'grayscale(1)'}}>
                      <LayoutPresetThumbnail variant={id} />
                    </Box>
                    {!isEnabled && (
                      <Box
                        sx={{
                          position: 'absolute',
                          top: 8,
                          right: 8,
                          bgcolor: 'warning.main',
                          color: 'warning.contrastText',
                          px: 1,
                          py: 0.4,
                          borderRadius: 1,
                          fontSize: '0.68rem',
                          fontWeight: 700,
                          letterSpacing: '0.03em',
                        }}
                      >
                        {t('layouts.badges.coming_soon.label', 'Coming Soon')}
                      </Box>
                    )}
                  </Box>
                  <Box sx={{px: 1.5, py: 1, borderTop: '1px solid', borderColor: 'divider'}}>
                    <Typography
                      variant="body2"
                      sx={{
                        fontWeight: 500,
                        fontSize: '0.8125rem',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {t(LAYOUT_PRESET_KEY[id], LAYOUT_PRESET_DEFAULT[id])}
                    </Typography>
                  </Box>
                </Card>
              </Grid>
            );
          })}
        </Grid>
      </Box>

      <ThemeDeleteDialog
        open={deleteTarget !== null}
        themeId={deleteTarget?.id ?? null}
        themeName={deleteTarget?.name ?? null}
        onClose={() => setDeleteTarget(null)}
      />
    </PageContent>
  );
}
