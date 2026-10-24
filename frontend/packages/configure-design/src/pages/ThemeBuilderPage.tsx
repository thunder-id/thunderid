// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {BuilderLayout, BuilderStaticPanel, DocumentTitle} from '@thunderid/components';
import {Box, Button, useColorScheme} from '@wso2/oxygen-ui';
import {ArrowLeft, Save, Trash, Undo2} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useRef, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ThemeConfigPanel from '../components/ThemeConfigPanel';
import ThemePreviewPanel from '../components/ThemePreviewPanel';
import ThemeBuilderLeftPanel from '../components/themes/ThemeBuilderLeftPanel';
import ThemeDeleteDialog from '../components/themes/ThemeDeleteDialog';
import DesignUIConstants from '../constants/design-ui-constants';
import useThemeBuilder from '../contexts/ThemeBuilder/useThemeBuilder';
import useDesignRoutes from '../hooks/useDesignRoutes';

export default function ThemeBuilderPage(): JSX.Element {
  const {t} = useTranslation('design');
  const {mode, systemMode} = useColorScheme();
  const navigate = useNavigate();
  const routes = useDesignRoutes();

  const {
    themeId,
    displayName,
    isReadOnly,
    activeSection,
    setActiveSection,
    isDirty,
    draftTheme,
    setDraftTheme,
    setIsDirty,
    resetDraft,
  } = useThemeBuilder();

  const saveHandlerRef = useRef<() => void>(() => null);
  const [isSaving, setIsSaving] = useState(false);
  const [isPanelOpen, setIsPanelOpen] = useState(true);
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);

  const handleTogglePanel = useCallback(() => {
    setIsPanelOpen((prev) => !prev);
  }, []);

  const handleBack = useCallback(() => {
    // eslint-disable-next-line @typescript-eslint/no-floating-promises
    navigate(routes.design.list());
  }, [navigate, routes.design]);

  const handleDeleteSuccess = useCallback(() => {
    // eslint-disable-next-line @typescript-eslint/no-floating-promises
    navigate(routes.design.list());
  }, [navigate, routes.design]);

  const bgColor = (systemMode ?? mode) === 'dark' ? '#141414' : '#f6f7f9';

  const [toolbarPortal, setToolbarPortal] = useState<HTMLDivElement | null>(null);

  const leftPanelContent = (
    <ThemeBuilderLeftPanel
      onPanelToggle={handleTogglePanel}
      draftTheme={draftTheme}
      setDraftTheme={setDraftTheme}
      setIsDirty={setIsDirty}
      activeSection={activeSection}
      setActiveSection={setActiveSection}
    />
  );

  return (
    <>
      <DocumentTitle title={displayName ?? t('themes.section.title', 'Themes')} />
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
        {/* ── Top bar: back button | toolbar (portal target) | action buttons ── */}
        <Box sx={{display: 'flex', alignItems: 'center', px: 2, py: 1, flexShrink: 0}}>
          <Button
            variant="text"
            size="small"
            startIcon={<ArrowLeft size={14} />}
            onClick={handleBack}
            sx={{textTransform: 'none', fontSize: '0.8rem', color: 'text.secondary', whiteSpace: 'nowrap'}}
          >
            {t('themes.builder.actions.back_to_design.label', 'Back to design')}
          </Button>
          {/* Portal target — the PreviewToolbar from GatePreview renders here */}
          <Box ref={setToolbarPortal} sx={{flex: 1, display: 'flex', justifyContent: 'center'}} />
          <Box sx={{display: 'flex', gap: 2}}>
            {!isReadOnly && (
              <Button
                variant="text"
                color="error"
                startIcon={<Trash size={18} />}
                onClick={() => setDeleteDialogOpen(true)}
              >
                {t('themes.builder.actions.delete.label', 'Delete')}
              </Button>
            )}
            <Button
              variant="outlined"
              disabled={!isDirty || isSaving}
              startIcon={<Undo2 size={18} />}
              onClick={resetDraft}
            >
              {t('themes.builder.actions.revert.label', 'Revert')}
            </Button>
            <Button
              variant="contained"
              disabled={!isDirty}
              startIcon={<Save size={18} />}
              onClick={() => saveHandlerRef.current()}
            >
              {t('themes.builder.actions.save.label', 'Save')}
            </Button>
          </Box>
        </Box>

        {/* ── Three-column builder area ──────────────────────────────────────── */}
        <Box sx={{flex: 1, overflow: 'hidden', p: 1, pt: 0}}>
          <BuilderLayout
            open={isPanelOpen}
            onPanelToggle={handleTogglePanel}
            panelWidth={DesignUIConstants.LEFT_PANEL_WIDTH}
            panelContent={leftPanelContent}
            expandTooltip={t('themes.builder.tooltips.show_sections', 'Show sections')}
            panelPaperSx={{
              overflow: 'hidden',
              display: 'flex',
              flexDirection: 'column',
              borderRight: '1px solid',
              borderColor: 'divider',
            }}
            rightPanel={
              <BuilderStaticPanel
                width={DesignUIConstants.RIGHT_PANEL_WIDTH}
                header={
                  activeSection
                    ? t(`themes.builder.sections.${activeSection}.label`, activeSection)
                    : t('themes.builder.config.label', 'Config')
                }
              >
                <ThemeConfigPanel
                  themeId={themeId ?? null}
                  activeSection={activeSection}
                  saveHandlerRef={saveHandlerRef}
                  onSavingChange={setIsSaving}
                />
              </BuilderStaticPanel>
            }
          >
            {/* ── Center: canvas preview ─────────────────────────────────── */}
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
              <ThemePreviewPanel themeId={themeId ?? null} toolbarPortal={toolbarPortal} />
            </Box>
          </BuilderLayout>
        </Box>
      </Box>

      <ThemeDeleteDialog
        open={deleteDialogOpen}
        themeId={themeId ?? null}
        themeName={displayName ?? null}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={handleDeleteSuccess}
      />
    </>
  );
}
