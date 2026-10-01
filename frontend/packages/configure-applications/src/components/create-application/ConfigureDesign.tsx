// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  LayoutPresetThumbnail,
  LayoutThumbnail,
  ThemeThumbnail,
  type LayoutPresetVariant,
} from '@thunderid/configure-design';
import {useConfig} from '@thunderid/contexts';
import {
  useGetThemes,
  useGetTheme,
  useGetLayouts,
  useGetLayout,
  type ThemeListItem,
  type LayoutListItem,
  type Theme,
  type LayoutConfig,
} from '@thunderid/design';
import {
  Typography,
  Stack,
  Card,
  CardContent,
  CardActionArea,
  Box,
  Grid,
  useTheme,
  Autocomplete,
  TextField,
  CircularProgress,
  Radio,
  RadioGroup,
  FormControlLabel,
  ColorSchemeImage,
} from '@wso2/oxygen-ui';
import {Palette, ExternalLink, Code, Lightbulb, LayoutTemplate} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ChangeEvent} from 'react';
import {useState, useEffect} from 'react';
import {useTranslation} from 'react-i18next';
import useApplicationCreateContext from '../../hooks/useApplicationCreateContext';
import {ApplicationCreateFlowSignInApproach, OrganizationUnitDefaultItem} from '../../models/application-create-flow';

const LAYOUT_PRESET_VARIANTS: readonly LayoutPresetVariant[] = ['centered', 'split', 'fullscreen', 'popup'];

function isLayoutPresetVariant(handle: string): handle is LayoutPresetVariant {
  return (LAYOUT_PRESET_VARIANTS as readonly string[]).includes(handle);
}

/**
 * Props for the {@link ConfigureDesign} component.
 *
 * @public
 */
export interface ConfigureDesignProps {
  /**
   * The ID of the currently selected theme (from API response)
   */
  themeId?: string | null;

  /**
   * The ID of the currently selected layout (from API response)
   */
  layoutId?: string | null;

  /**
   * Callback function when a theme is selected, receives theme ID and config separately
   */
  onThemeSelect: (themeId: string, themeConfig: Theme) => void;

  /**
   * Callback function when a layout is selected, receives layout ID and config separately
   */
  onLayoutSelect: (layoutId: string, layoutConfig: LayoutConfig) => void;

  /**
   * Callback function to broadcast whether this step is ready to proceed
   */
  onReadyChange?: (isReady: boolean) => void;

  /**
   * Currently selected sign-in approach.
   */
  selectedApproach: ApplicationCreateFlowSignInApproach;

  /**
   * Callback invoked when the sign-in approach changes.
   */
  onApproachChange: (approach: ApplicationCreateFlowSignInApproach) => void;

  /**
   * Whether the embedded (native) sign-in approach is allowed. Browser-based SPAs (public
   * clients) must use the redirect-based approach, so the embedded option is hidden for them.
   * Defaults to true.
   */
  allowEmbeddedApproach?: boolean;

  /**
   * Whether the sign-in approach picker (hosted pages vs. custom UI) is shown at all. Browser
   * SPAs have no choice to make here (redirect-only). Defaults to true.
   */
  showApproachSection?: boolean;
}

/**
 * React component that renders the experience step in the application creation onboarding
 * flow: choosing a sign-in approach and a theme for the application. The application's logo is
 * picked earlier, inline with its name on the Details step.
 *
 * @param props - The component props
 * @returns JSX element displaying the experience customization interface
 *
 * @public
 */
export default function ConfigureDesign({
  themeId: externallyProvidedThemeId = null,
  layoutId: externallyProvidedLayoutId = null,
  onThemeSelect,
  onLayoutSelect,
  onReadyChange = undefined,
  selectedApproach,
  onApproachChange,
  allowEmbeddedApproach = true,
  showApproachSection = true,
}: ConfigureDesignProps): JSX.Element {
  const {t} = useTranslation();
  const theme = useTheme();
  const {config} = useConfig();
  const {brand} = config;
  const {product_name: productName} = brand || {};
  const {ouDefaults, selectedTemplateConfig, appName} = useApplicationCreateContext();
  const {data: themesData, isLoading: loadingThemes} = useGetThemes({limit: 100});
  const {data: layoutsData} = useGetLayouts({limit: 100});

  const [selectedThemeId, setSelectedThemeId] = useState<string | null>(externallyProvidedThemeId ?? null);
  const [selectedLayoutId, setSelectedLayoutId] = useState<string | null>(externallyProvidedLayoutId ?? null);

  const showTheme = !ouDefaults[OrganizationUnitDefaultItem.THEME];
  const showLayout = !ouDefaults[OrganizationUnitDefaultItem.LAYOUT];

  const THEME_GRID_THRESHOLD = 8;
  const themeList = themesData?.themes ?? [];
  const hasThemes = Boolean(themeList.length);
  const useAutocomplete = themeList.length > THEME_GRID_THRESHOLD;

  const layoutList = layoutsData?.layouts ?? [];
  // Only worth surfacing a picker when there's an actual choice to make; with 0 or 1 layouts the
  // fallback below already selects the only option there is.
  const showLayoutPicker = showLayout && layoutList.length > 1;
  const useLayoutAutocomplete = layoutList.length > THEME_GRID_THRESHOLD;

  // Falls back to the first available theme/layout until the user (or an OU default) makes an
  // explicit choice, so a selection is always in effect once data loads.
  const effectiveThemeId = selectedThemeId ?? (showTheme ? (themeList[0]?.id ?? null) : null);
  const effectiveLayoutId = selectedLayoutId ?? (showLayout ? (layoutList[0]?.id ?? null) : null);

  const {data: selectedThemeDetails} = useGetTheme(effectiveThemeId ?? '');
  const {data: selectedLayoutDetails} = useGetLayout(effectiveLayoutId ?? '');

  // The template's preferred approach leads the picker, so e.g. a template defaulting to Embedded
  // shows that card first instead of always leading with Hosted Pages. This is fixed at the
  // template's default rather than the currently selected approach, so the cards don't reorder
  // themselves as the user clicks between them.
  const isEmbeddedApproachDefault =
    selectedTemplateConfig?.defaults?.signInApproach === ApplicationCreateFlowSignInApproach.EMBEDDED;

  /**
   * Notify parent when theme details load.
   */
  useEffect((): void => {
    if (selectedThemeDetails) {
      onThemeSelect(selectedThemeDetails.id, selectedThemeDetails.theme);
    }
  }, [selectedThemeDetails, onThemeSelect]);

  /**
   * Notify parent when layout details load.
   */
  useEffect((): void => {
    if (selectedLayoutDetails) {
      onLayoutSelect(selectedLayoutDetails.id, selectedLayoutDetails.layout);
    }
  }, [selectedLayoutDetails, onLayoutSelect]);

  /**
   * Broadcast readiness — Experience step is always ready since it has default values.
   */
  useEffect((): void => {
    if (onReadyChange) {
      onReadyChange(true);
    }
  }, [onReadyChange]);

  const handleThemeSelect = (themeItem: ThemeListItem): void => {
    setSelectedThemeId(themeItem.id);
  };

  const handleLayoutSelect = (layoutItem: LayoutListItem): void => {
    setSelectedLayoutId(layoutItem.id);
  };

  const handleApproachChange = (event: ChangeEvent<HTMLInputElement>): void => {
    onApproachChange(event.target.value as ApplicationCreateFlowSignInApproach);
  };

  let themeSelectionContent: JSX.Element;

  if (!hasThemes) {
    themeSelectionContent = (
      <Stack
        direction="column"
        spacing={2}
        alignItems="center"
        sx={{
          p: 4,
          borderRadius: '12px',
          border: `1px dashed ${theme.vars?.palette.divider}`,
        }}
      >
        <Palette size={32} color={theme.vars?.palette.text.secondary} />
        <Typography variant="body1" color="text.secondary" textAlign="center">
          {t('applications:onboarding.configure.design.theme.emptyState')}
        </Typography>
        <Typography variant="caption" color="text.secondary" textAlign="center">
          {t('applications:onboarding.configure.design.theme.emptyStateHint')}
        </Typography>
      </Stack>
    );
  } else if (useAutocomplete) {
    themeSelectionContent = (
      <Autocomplete
        fullWidth
        options={themeList}
        getOptionLabel={(option) => (typeof option === 'string' ? option : option.displayName)}
        value={themeList.find((themeListItem) => themeListItem.id === effectiveThemeId) ?? null}
        onChange={(_event, newValue): void => {
          if (newValue) handleThemeSelect(newValue);
        }}
        loading={loadingThemes}
        renderInput={(params) => (
          <TextField
            {...params}
            placeholder={t('applications:onboarding.configure.design.theme.title')}
            slotProps={{
              input: {
                ...params.InputProps,
                endAdornment: (
                  <>
                    {loadingThemes ? <CircularProgress color="inherit" size={20} /> : null}
                    {params.InputProps.endAdornment}
                  </>
                ),
              },
            }}
          />
        )}
      />
    );
  } else {
    themeSelectionContent = (
      <Grid container spacing={2}>
        {themeList.map((themeItem: ThemeListItem) => {
          const isSelected: boolean = effectiveThemeId === themeItem.id;
          return (
            <Grid key={themeItem.id} size={{xs: 3, sm: 4, md: 3, lg: 2}}>
              <Card
                data-testid={`theme-card-${themeItem.id}`}
                onClick={(): void => handleThemeSelect(themeItem)}
                sx={{
                  cursor: 'pointer',
                  border: isSelected ? `2px solid ${theme.vars?.palette.primary.main}` : undefined,
                  '&:hover': {
                    borderColor: 'primary.main',
                    boxShadow: '0 4px 20px rgba(0,0,0,0.1)',
                    transform: 'translateY(-2px)',
                  },
                }}
              >
                <Box sx={{aspectRatio: '4/3', overflow: 'hidden', position: 'relative'}}>
                  <ThemeThumbnail theme={themeItem} />
                </Box>
                <Box sx={{px: 1.25, py: 0.75, borderTop: '1px solid', borderColor: 'divider'}}>
                  <Typography
                    variant="body2"
                    sx={{
                      fontWeight: isSelected ? 600 : 500,
                      fontSize: '0.75rem',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {themeItem.displayName}
                  </Typography>
                </Box>
              </Card>
            </Grid>
          );
        })}
      </Grid>
    );
  }

  let layoutSelectionContent: JSX.Element | null = null;

  if (showLayout && useLayoutAutocomplete) {
    layoutSelectionContent = (
      <Autocomplete
        fullWidth
        options={layoutList}
        getOptionLabel={(option) => (typeof option === 'string' ? option : option.displayName)}
        value={layoutList.find((layoutItem) => layoutItem.id === effectiveLayoutId) ?? null}
        onChange={(_event, newValue): void => {
          if (newValue) handleLayoutSelect(newValue);
        }}
        renderInput={(params) => (
          <TextField {...params} placeholder={t('applications:onboarding.configure.design.layout.title')} />
        )}
      />
    );
  } else if (showLayoutPicker) {
    layoutSelectionContent = (
      <Grid container spacing={2}>
        {layoutList.map((layoutItem: LayoutListItem) => {
          const isSelected: boolean = effectiveLayoutId === layoutItem.id;
          return (
            <Grid key={layoutItem.id} size={{xs: 3, sm: 4, md: 3, lg: 2}}>
              <Card
                data-testid={`layout-card-${layoutItem.id}`}
                onClick={(): void => handleLayoutSelect(layoutItem)}
                sx={{
                  cursor: 'pointer',
                  border: isSelected ? `2px solid ${theme.vars?.palette.primary.main}` : undefined,
                  '&:hover': {
                    borderColor: 'primary.main',
                    boxShadow: '0 4px 20px rgba(0,0,0,0.1)',
                    transform: 'translateY(-2px)',
                  },
                }}
              >
                <Box sx={{aspectRatio: '4/3', overflow: 'hidden', position: 'relative'}}>
                  {isLayoutPresetVariant(layoutItem.handle) ? (
                    <LayoutPresetThumbnail variant={layoutItem.handle} />
                  ) : (
                    <LayoutThumbnail layout={layoutItem} />
                  )}
                </Box>
                <Box sx={{px: 1.25, py: 0.75, borderTop: '1px solid', borderColor: 'divider'}}>
                  <Typography
                    variant="body2"
                    sx={{
                      fontWeight: isSelected ? 600 : 500,
                      fontSize: '0.75rem',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {layoutItem.displayName}
                  </Typography>
                </Box>
              </Card>
            </Grid>
          );
        })}
      </Grid>
    );
  }

  return (
    <Stack direction="column" spacing={4} data-testid="application-configure-design">
      {/* Sign-In Approach is the leading heading for this step when shown; otherwise the page
          falls back to the generic experience title below. */}
      {showApproachSection ? (
        <Stack direction="column" spacing={1}>
          <Typography variant="h1" gutterBottom>
            {t('applications:onboarding.configure.approach.title')}
          </Typography>
          <Stack direction="row" alignItems="center" spacing={1}>
            <Lightbulb size={20} color={theme.vars?.palette.warning.main} />
            <Typography variant="body2" color="text.secondary">
              {t('applications:onboarding.configure.approach.subtitle')}
            </Typography>
          </Stack>
        </Stack>
      ) : (
        <Typography variant="h1" gutterBottom>
          {t('applications:onboarding.configure.design.title')}
        </Typography>
      )}

      {/* Sign-In Approach - hosted pages vs. custom UI. Hidden for browser SPAs, which are
          redirect-only and have no choice to make here. */}
      {showApproachSection &&
        (() => {
          const isInbuiltSelected = selectedApproach === ApplicationCreateFlowSignInApproach.REDIRECT_BASED;
          const isEmbeddedSelected = selectedApproach === ApplicationCreateFlowSignInApproach.EMBEDDED;

          const hostedPagesCard = (
            <Card
              key="hosted-pages"
              variant="outlined"
              onClick={() => onApproachChange(ApplicationCreateFlowSignInApproach.REDIRECT_BASED)}
              sx={{borderRadius: '14px'}}
            >
              <CardActionArea
                sx={{
                  height: '100%',
                  cursor: 'pointer',
                  border: 1,
                  borderColor: isInbuiltSelected ? 'primary.main' : 'divider',
                  bgcolor: isInbuiltSelected ? 'action.selected' : 'transparent',
                  transition: 'all 0.2s ease-in-out',
                  '&:hover': {
                    borderColor: 'primary.main',
                    bgcolor: isInbuiltSelected ? 'action.selected' : 'action.hover',
                  },
                }}
              >
                <CardContent sx={{p: 3}}>
                  <Stack direction="row" spacing={2} alignItems="flex-start">
                    <FormControlLabel
                      value={ApplicationCreateFlowSignInApproach.REDIRECT_BASED}
                      control={<Radio />}
                      label=""
                      sx={{m: 0, mt: 0.25}}
                      onClick={(e) => e.stopPropagation()}
                    />
                    <Box sx={{flex: 1}}>
                      <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 1}}>
                        <ExternalLink size={20} />
                        <Typography variant="h6">
                          {t('applications:onboarding.configure.approach.inbuilt.title', {
                            product: productName,
                          })}
                        </Typography>
                      </Stack>
                      <Typography variant="body2" color="text.secondary" sx={{mb: 2.5}}>
                        {t('applications:onboarding.configure.approach.inbuilt.description', {
                          product: productName,
                        })}
                      </Typography>

                      {/* Illustrative preview of the hosted sign-in page, not a live render. */}
                      <Box
                        sx={{
                          width: 260,
                          maxWidth: '100%',
                          borderRadius: '10px',
                          overflow: 'hidden',
                          border: 1,
                          borderColor: 'divider',
                          bgcolor: 'background.paper',
                          boxShadow: 2,
                        }}
                      >
                        <Box
                          sx={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: 0.75,
                            px: 1.25,
                            py: 1,
                            bgcolor: 'action.hover',
                            borderBottom: 1,
                            borderColor: 'divider',
                          }}
                        >
                          <Box sx={{width: 6, height: 6, borderRadius: '50%', bgcolor: 'text.disabled'}} />
                          <Box sx={{width: 6, height: 6, borderRadius: '50%', bgcolor: 'text.disabled'}} />
                          <Box sx={{width: 6, height: 6, borderRadius: '50%', bgcolor: 'text.disabled'}} />
                        </Box>
                        <Box sx={{p: 2.5, display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1.25}}>
                          <ColorSchemeImage
                            src={{
                              light: `${import.meta.env.BASE_URL}/assets/images/logo-mini.svg`,
                              dark: `${import.meta.env.BASE_URL}/assets/images/logo-mini-inverted.svg`,
                            }}
                            alt={{light: 'ThunderID', dark: 'ThunderID'}}
                            height={20}
                            width="auto"
                          />
                          <Typography variant="caption" sx={{fontWeight: 600, textAlign: 'center'}}>
                            {t('applications:onboarding.configure.approach.inbuilt.preview.title', {
                              appName: appName || productName,
                            })}
                          </Typography>
                          <Box
                            sx={{
                              width: '100%',
                              height: 20,
                              borderRadius: '6px',
                              bgcolor: 'action.hover',
                              border: 1,
                              borderColor: 'divider',
                            }}
                          />
                          <Box
                            sx={{
                              width: '100%',
                              height: 20,
                              borderRadius: '6px',
                              bgcolor: 'action.hover',
                              border: 1,
                              borderColor: 'divider',
                            }}
                          />
                          <Box sx={{width: '100%', height: 22, borderRadius: '6px', bgcolor: 'primary.main'}} />
                        </Box>
                      </Box>
                    </Box>
                  </Stack>
                </CardContent>
              </CardActionArea>
            </Card>
          );

          // Hidden for public-client SPAs which must use redirect-based flows.
          const embeddedCard = allowEmbeddedApproach && (
            <Card
              key="embedded"
              variant="outlined"
              onClick={() => onApproachChange(ApplicationCreateFlowSignInApproach.EMBEDDED)}
              sx={{borderRadius: '14px'}}
            >
              <CardActionArea
                sx={{
                  height: '100%',
                  cursor: 'pointer',
                  border: 1,
                  borderColor: isEmbeddedSelected ? 'primary.main' : 'divider',
                  bgcolor: isEmbeddedSelected ? 'action.selected' : 'transparent',
                  transition: 'all 0.2s ease-in-out',
                  '&:hover': {
                    borderColor: 'primary.main',
                    bgcolor: isEmbeddedSelected ? 'action.selected' : 'action.hover',
                  },
                }}
              >
                <CardContent sx={{p: 3}}>
                  <Stack direction="row" spacing={2} alignItems="flex-start">
                    <FormControlLabel
                      value={ApplicationCreateFlowSignInApproach.EMBEDDED}
                      control={<Radio />}
                      label=""
                      sx={{m: 0, mt: 0.25}}
                      onClick={(e) => e.stopPropagation()}
                    />
                    <Box sx={{flex: 1}}>
                      <Stack direction="row" spacing={1} alignItems="center" sx={{mb: 1}}>
                        <Code size={20} />
                        <Typography variant="h6">
                          {t('applications:onboarding.configure.approach.native.title')}
                        </Typography>
                      </Stack>
                      <Typography variant="body2" color="text.secondary">
                        {t('applications:onboarding.configure.approach.native.description', {
                          product: productName,
                        })}
                      </Typography>
                    </Box>
                  </Stack>
                </CardContent>
              </CardActionArea>
            </Card>
          );

          const orderedCards = isEmbeddedApproachDefault
            ? [embeddedCard, hostedPagesCard]
            : [hostedPagesCard, embeddedCard];

          return (
            <Stack direction="column" spacing={2}>
              <RadioGroup value={selectedApproach} onChange={handleApproachChange}>
                <Stack direction="column" spacing={2}>
                  {orderedCards}
                </Stack>
              </RadioGroup>
            </Stack>
          );
        })()}

      {/* Theme Selection */}
      {showTheme && (
        <Stack direction="column" spacing={3}>
          <Stack direction="row" alignItems="center" spacing={1}>
            <Palette size={14} />
            <Typography variant="h6">{t('applications:onboarding.configure.design.theme.title')}</Typography>
          </Stack>

          {themeSelectionContent}
        </Stack>
      )}

      {/* Layout Selection — only shown when there's an actual choice between multiple layouts */}
      {layoutSelectionContent && (
        <Stack direction="column" spacing={3}>
          <Stack direction="row" alignItems="center" spacing={1}>
            <LayoutTemplate size={14} />
            <Typography variant="h6">{t('applications:onboarding.configure.design.layout.title')}</Typography>
          </Stack>

          {layoutSelectionContent}
        </Stack>
      )}
    </Stack>
  );
}
