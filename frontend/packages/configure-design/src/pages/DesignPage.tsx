// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink, QueryErrorNotice} from '@thunderid/components';
import {useGetThemes, useGetLayouts} from '@thunderid/design';
import {Box, Button, Grid, PageContent, PageTitle, Skeleton, Typography} from '@wso2/oxygen-ui';
import {ArrowUpRight, LayoutTemplate, Palette, Plus} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ItemCard from '../components/common/ItemCard';
import SectionHeader from '../components/common/SectionHeader';
import LayoutPresetThumbnail, {type LayoutPresetVariant} from '../components/layouts/LayoutPresetThumbnail';
import LayoutThumbnail from '../components/layouts/LayoutThumbnail';
import ThemeDeleteDialog from '../components/themes/ThemeDeleteDialog';
import ThemeThumbnail from '../components/themes/ThemeThumbnail';
import DesignUIConstants from '../constants/design-ui-constants';
import useDesignRoutes from '../hooks/useDesignRoutes';

const LAYOUT_PRESET_VARIANTS: readonly LayoutPresetVariant[] = ['centered', 'split', 'fullscreen', 'popup'];

function isLayoutPresetVariant(handle: string): handle is LayoutPresetVariant {
  return (LAYOUT_PRESET_VARIANTS as readonly string[]).includes(handle);
}

export default function DesignPage(): JSX.Element {
  const {t} = useTranslation('design');
  const navigate = useNavigate();
  const routes = useDesignRoutes();
  const {data: themesData, isLoading: themesLoading, error: themesError, refetch: refetchThemes} = useGetThemes();
  const {data: layoutsData, error: layoutsError, refetch: refetchLayouts} = useGetLayouts();

  const [showAllThemes, setShowAllThemes] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<{id: string; name: string} | null>(null);

  const allLayouts = layoutsData?.layouts ?? [];

  const allThemes = themesData?.themes ?? [];
  const visibleThemes = showAllThemes ? allThemes : allThemes.slice(0, DesignUIConstants.INITIAL_LIMIT);

  const skeletonCount = 4;

  return (
    <PageContent>
      <DocumentTitle title={t('page.title', 'Design')} />
      <PageTitle>
        <PageTitle.Header>{t('page.title', 'Design')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('page.subtitle', 'Create, customize, and manage visual themes & layouts for your applications.')}{' '}
          <ExternalLink docKey="design" confirmBeforeNavigate={false} />
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
                  await navigate(routes.design.themesCreate());
                })().catch(() => {
                  // Ignore navigation errors
                });
              }}
            >
              {t('themes.actions.add.label', 'Add theme')}
            </Button>
          }
        />

        {themesError ? (
          <QueryErrorNotice error={themesError} t={t} variant="inline" onRetry={() => void refetchThemes()} />
        ) : (
          <>
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
                              await navigate(routes.design.themeDetail(theme.id));
                            })().catch(() => {
                              // Ignore navigation errors
                            });
                          }}
                          onDelete={
                            theme.isReadOnly
                              ? undefined
                              : () => setDeleteTarget({id: theme.id, name: theme.displayName})
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
          </>
        )}
      </Box>

      <Box>
        {/* ── Layouts section ────────────────────────────────────────────── */}
        <SectionHeader
          title={t('layouts.section.title', 'Layouts')}
          count={allLayouts.length}
          icon={<LayoutTemplate size={18} />}
        />

        {layoutsError ? (
          <QueryErrorNotice error={layoutsError} t={t} variant="inline" onRetry={() => void refetchLayouts()} />
        ) : (
          <>
            <Grid container spacing={2}>
              {allLayouts.map((layout) => (
                <Grid key={layout.id} size={{xs: 6, sm: 4, md: 3, lg: 2}}>
                  <ItemCard
                    thumbnail={
                      isLayoutPresetVariant(layout.handle) ? (
                        <LayoutPresetThumbnail variant={layout.handle} />
                      ) : (
                        <LayoutThumbnail layout={layout} />
                      )
                    }
                    name={layout.displayName}
                    onClick={() => {
                      (async () => {
                        await navigate(routes.design.layoutDetail(layout.id));
                      })().catch(() => {
                        // Ignore navigation errors
                      });
                    }}
                  />
                </Grid>
              ))}
            </Grid>

            {allLayouts.length === 0 && (
              <Box sx={{py: 6, textAlign: 'center', color: 'text.secondary'}}>
                <LayoutTemplate size={32} style={{opacity: 0.3, marginBottom: 8}} />
                <Typography variant="body2">{t('layouts.empty_state.message', 'No layouts yet')}</Typography>
              </Box>
            )}
          </>
        )}
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
