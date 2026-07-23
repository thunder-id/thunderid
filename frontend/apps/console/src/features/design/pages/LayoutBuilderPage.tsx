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

import {BuilderLayout, BuilderStaticPanel} from '@thunderid/components';
import {useGetThemes, useGetTheme, type Stylesheet} from '@thunderid/design';
import {Autocomplete, Box, Button, IconButton, TextField, Tooltip, Typography, useColorScheme} from '@wso2/oxygen-ui';
import {ArrowLeft, Crosshair, Layers, Save} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useMemo, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import GatePreview from '../../../components/GatePreview/GatePreview';
import RouteConfig from '../../../configs/RouteConfig';
import LayoutConfigPanel from '../components/LayoutConfigPanel';
import LayoutPreviewPanel from '../components/LayoutPreviewPanel';
import AddScreenRow from '../components/layouts/AddScreenRow';
import type {CustomCSSEditorHandle} from '../components/layouts/CustomCSSEditor';
import ScreenListItem from '../components/layouts/ScreenListItem';
import DesignUIConstants from '../constants/design-ui-constants';
import useLayoutBuilder from '../contexts/LayoutBuilder/useLayoutBuilder';

export default function LayoutBuilderPage(): JSX.Element {
  const {t} = useTranslation('design');
  const {mode, systemMode} = useColorScheme();
  const navigate = useNavigate();

  const {
    layoutId,
    handle,
    displayName,
    draftLayout,
    updateDraftLayout,
    selectedScreen,
    setSelectedScreen,
    screenDraft,
    isDirty,
    addScreen,
    getAllScreens,
    getBaseScreenNames,
    setScreenDraft,
    setIsDirty,
  } = useLayoutBuilder();

  const saveHandlerRef = useRef<() => void>(() => {
    /* no-op */
  });
  const cssEditorRef = useRef<CustomCSSEditorHandle>(null);
  const [inspectorEnabled, setInspectorEnabled] = useState(true);
  const [isPanelOpen, setIsPanelOpen] = useState(true);
  const [toolbarPortal, setToolbarPortal] = useState<HTMLDivElement | null>(null);

  const handleTogglePanel = useCallback(() => {
    setIsPanelOpen((prev) => !prev);
  }, []);

  // Theme selector for preview
  const {data: themesData} = useGetThemes();
  const themeOptions = themesData?.themes ?? [];
  const [selectedThemeId, setSelectedThemeId] = useState<string | null>(null);
  const resolvedThemeId = selectedThemeId ?? themeOptions[0]?.id ?? null;
  const {data: themeData} = useGetTheme(resolvedThemeId ?? '');
  const previewTheme = themeData?.theme ?? undefined;

  // Extract page background from the selected screen draft
  const currentScreenDef = selectedScreen ? (screenDraft ?? draftLayout?.screens?.[selectedScreen]) : undefined;
  const pageBackground = (currentScreenDef?.background as Record<string, unknown> | undefined)?.value as
    | string
    | undefined;

  // Stylesheets from draft layout (layout-level, not per-screen)
  const draftHead = (draftLayout as Record<string, unknown> | null)?.head as Record<string, unknown> | undefined;
  const rawStylesheets = draftHead?.stylesheets as Stylesheet[] | undefined;
  const stylesheets = useMemo(() => rawStylesheets ?? [], [rawStylesheets]);

  const handleStylesheetsChange = useCallback(
    (updated: Stylesheet[]) => {
      updateDraftLayout(['head', 'stylesheets'], updated);
    },
    [updateDraftLayout],
  );

  /** When the inspector picks a selector, append a stub rule to the last inline stylesheet (or create one). */
  const handleSelectSelector = useCallback(
    (selector: string) => {
      // Flush any pending debounced edits so we read the latest content.
      cssEditorRef.current?.flush();
      const stub = `${selector} {\n  \n}\n`;
      const lastInlineIdx = stylesheets.map((s) => s.type).lastIndexOf('inline');

      if (lastInlineIdx >= 0) {
        const sheet = stylesheets[lastInlineIdx];
        if (sheet.type === 'inline') {
          const separator = sheet.content && !sheet.content.endsWith('\n') ? '\n\n' : '\n';
          const updated = stylesheets.map((s, i) =>
            i === lastInlineIdx && s.type === 'inline' ? {...s, content: `${s.content}${separator}${stub}`} : s,
          );
          handleStylesheetsChange(updated);
        }
      } else {
        const existing = new Set(stylesheets.map((s) => s.id));
        let n = stylesheets.length + 1;
        while (existing.has(`custom-${n}`)) n += 1;
        handleStylesheetsChange([...stylesheets, {id: `custom-${n}`, type: 'inline', content: stub}]);
      }
    },
    [stylesheets, handleStylesheetsChange],
  );

  const allScreens = getAllScreens();
  const screenNames = Object.keys(allScreens);
  const baseScreenNames = getBaseScreenNames();

  const handleNavigateBack = (): void => {
    (async () => {
      await navigate(RouteConfig.design.list());
    })().catch(() => {
      // Ignore navigation errors
    });
  };

  const handleAddScreen = (name: string, extendsBase: string): void => {
    addScreen(name, extendsBase);
  };

  const bgColor = (systemMode ?? mode) === 'dark' ? '#141414' : '#f6f7f9';

  const hasLeftPanel = handle !== 'centered';

  const leftPanelContent = hasLeftPanel ? (
    <>
      <Box
        sx={{
          px: 1.25,
          pt: 1.5,
          pb: 0.75,
          display: 'flex',
          alignItems: 'center',
          gap: 0.75,
        }}
      >
        <Layers size={14} style={{opacity: 0.5}} />
        <Typography
          variant="caption"
          sx={{
            fontWeight: 600,
            fontSize: '0.68rem',
            textTransform: 'uppercase',
            letterSpacing: '0.06em',
            color: 'text.secondary',
          }}
        >
          {t('layouts.builder.screens.label', 'Screens')}
        </Typography>
        <Box sx={{flex: 1}} />
        <Typography variant="caption" sx={{fontSize: '0.65rem', color: 'text.disabled'}}>
          {screenNames.length}
        </Typography>
      </Box>

      <Box sx={{flex: 1, overflowY: 'auto', px: 1.25, pb: 1, display: 'flex', flexDirection: 'column', gap: 0.5}}>
        {screenNames.map((name) => (
          <ScreenListItem
            key={name}
            name={name}
            extendsBase={allScreens[name]?.extends as string | undefined}
            isSelected={selectedScreen === name}
            onClick={() => setSelectedScreen(name)}
          />
        ))}
      </Box>

      {/* Add screen */}
      <Box sx={{px: 1.25, pb: 1.25, pt: 0.5, borderTop: '1px solid', borderColor: 'divider'}}>
        <AddScreenRow baseScreens={baseScreenNames} onAdd={handleAddScreen} />
      </Box>
    </>
  ) : undefined;

  const toolbarEnd =
    handle === 'centered' ? (
      <Box sx={{display: 'flex', alignItems: 'center', gap: 1}}>
        {themeOptions.length > 0 && (
          <Autocomplete
            size="small"
            options={themeOptions}
            getOptionLabel={(option) => option.displayName}
            value={themeOptions.find((opt) => opt.id === resolvedThemeId) ?? themeOptions[0] ?? null}
            onChange={(_e, newValue) => setSelectedThemeId(newValue?.id ?? null)}
            disableClearable
            sx={{
              width: 160,
              '& .MuiInputBase-root': {height: 28, fontSize: '0.75rem'},
              '& .MuiAutocomplete-endAdornment': {top: '50%', transform: 'translateY(-50%)'},
            }}
            renderInput={(params) => (
              <TextField
                {...params}
                placeholder={t('layouts.builder.toolbar.theme.placeholder', 'Theme')}
                variant="outlined"
              />
            )}
          />
        )}
        <Box sx={{width: '1px', height: 16, bgcolor: 'divider', mx: 0.5, flexShrink: 0}} />
        <Tooltip
          title={
            inspectorEnabled
              ? t('layouts.builder.toolbar.inspector.disable', 'Disable element inspector')
              : t('layouts.builder.toolbar.inspector.enable', 'Inspect elements')
          }
        >
          <IconButton
            size="small"
            aria-label={t('layouts.builder.toolbar.inspector.label', 'Element inspector')}
            aria-pressed={inspectorEnabled}
            onClick={() => setInspectorEnabled((prev) => !prev)}
            sx={{
              bgcolor: inspectorEnabled ? 'primary.main' : 'transparent',
              color: inspectorEnabled ? 'primary.contrastText' : 'text.secondary',
              '&:hover': {
                bgcolor: inspectorEnabled ? 'primary.dark' : 'action.hover',
              },
            }}
          >
            <Crosshair size={16} />
          </IconButton>
        </Tooltip>
      </Box>
    ) : undefined;

  return (
    <Box
      sx={{
        width: '100%',
        height: 'inherit',
        display: 'flex',
        flexDirection: 'column',
        bgcolor: 'var(--flow-builder-background-color)',
        '[data-color-scheme="dark"] &': {
          bgcolor: 'var(--flow-builder-background-color-dark)',
        },
      }}
    >
      {/* ── Top bar: back button | toolbar (portal target) | save button ──── */}
      <Box sx={{display: 'flex', alignItems: 'center', px: 2, py: 1, flexShrink: 0}}>
        <Button
          variant="text"
          size="small"
          startIcon={<ArrowLeft size={14} />}
          onClick={handleNavigateBack}
          sx={{textTransform: 'none', fontSize: '0.8rem', color: 'text.secondary', whiteSpace: 'nowrap'}}
        >
          {t('layouts.builder.actions.back_to_design.label', 'Back to Design')}
        </Button>
        {/* Portal target — the PreviewToolbar from GatePreview renders here */}
        <Box ref={setToolbarPortal} sx={{flex: 1, display: 'flex', justifyContent: 'center'}} />
        <Button
          variant="contained"
          // size="small"
          disabled={!isDirty}
          startIcon={<Save size={18} />}
          onClick={() => saveHandlerRef.current()}
        >
          {t('layouts.builder.actions.save.label', 'Save')}
        </Button>
      </Box>

      {/* ── Three-column builder area ──────────────────────────────────────── */}
      <Box sx={{flex: 1, overflow: 'hidden', p: 1, pt: 0}}>
        <BuilderLayout
          open={hasLeftPanel && isPanelOpen}
          onPanelToggle={handleTogglePanel}
          panelWidth={DesignUIConstants.LEFT_PANEL_WIDTH}
          panelContent={leftPanelContent}
          expandTooltip={t('layouts.builder.tooltips.show_screens', 'Show screens')}
          panelPaperSx={{
            overflow: 'hidden',
            display: 'flex',
            flexDirection: 'column',
            borderRight: '1px solid',
            borderColor: 'divider',
            p: 0,
          }}
          rightPanel={
            <BuilderStaticPanel
              width={DesignUIConstants.RIGHT_PANEL_WIDTH}
              header={(() => {
                if (handle === 'centered') return t('layouts.config.custom_css.title', 'Custom CSS');
                if (selectedScreen)
                  return t('layouts.builder.screen_header', 'Screen — {{name}}', {name: selectedScreen});
                return t('layouts.builder.constraints.label', 'Constraints');
              })()}
            >
              <LayoutConfigPanel
                layoutId={layoutId ?? null}
                selectedScreen={selectedScreen}
                onScreenChange={setSelectedScreen}
                screenDraft={screenDraft}
                onScreenDraftChange={setScreenDraft}
                onDirtyChange={setIsDirty}
                saveHandlerRef={saveHandlerRef}
                stylesheets={stylesheets}
                onStylesheetsChange={handleStylesheetsChange}
                cssEditorRef={cssEditorRef}
              />
            </BuilderStaticPanel>
          }
        >
          {/* ── Center: canvas ─────────────────────────────────────────── */}
          <Box
            sx={{
              height: '100%',
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              borderRadius: 1,
              bgcolor: bgColor,
            }}
          >
            {handle === 'centered' ? (
              <GatePreview
                theme={previewTheme ?? undefined}
                displayName={displayName ?? ''}
                pageBackground={pageBackground}
                stylesheets={stylesheets}
                inspectorEnabled={inspectorEnabled}
                onSelectSelector={handleSelectSelector}
                toolbarEnd={toolbarEnd}
                toolbarPortal={toolbarPortal}
              />
            ) : (
              <LayoutPreviewPanel
                layoutId={layoutId ?? null}
                selectedScreen={selectedScreen}
                screenDraft={screenDraft}
                showRulers
              />
            )}
          </Box>
        </BuilderLayout>
      </Box>
    </Box>
  );
}
