// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Box, IconButton, Tooltip} from '@wso2/oxygen-ui';
import {
  Expand,
  LayoutGrid,
  Magnet,
  Map,
  Maximize,
  Minus,
  Plus,
  Redo2,
  Shrink,
  Undo2,
} from '@wso2/oxygen-ui-icons-react';
import {useReactFlow} from '@xyflow/react';
import {type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import EdgeStyleMenu from './EdgeStyleSelector';
import useEdgeStyleSelector from '../../hooks/useEdgeStyleSelector';
import useFlowConfig from '../../hooks/useFlowConfig';
import getEdgeStyleIcon from '../../utils/getEdgeStyleIcon';

export interface CanvasToolbarProps {
  onAutoLayout: () => void;
  /** Undo the last canvas edit. Omit to hide the undo/redo controls. */
  onUndo?: () => void;
  /** Redo the last undone canvas edit. */
  onRedo?: () => void;
  /** Whether an undo step is available. */
  canUndo?: boolean;
  /** Whether a redo step is available. */
  canRedo?: boolean;
}

function ToolbarDivider(): ReactElement {
  return <Box sx={{width: '1px', height: 16, bgcolor: 'divider', mx: 0.5, flexShrink: 0}} />;
}

export default function CanvasToolbar({
  onAutoLayout,
  onUndo = undefined,
  onRedo = undefined,
  canUndo = false,
  canRedo = false,
}: CanvasToolbarProps): ReactElement {
  const {t} = useTranslation();
  const {fitView, zoomIn, zoomOut} = useReactFlow();
  const {
    edgeStyle,
    isMiniMapVisible,
    isSnapToGridEnabled,
    isVerboseMode,
    setIsMiniMapVisible,
    setIsSnapToGridEnabled,
    setIsVerboseMode,
  } = useFlowConfig();
  const {anchorEl, handleClick: handleEdgeStyleClick, handleClose: handleEdgeStyleClose} = useEdgeStyleSelector();

  const showHistoryControls = Boolean(onUndo ?? onRedo);

  return (
    <>
      <Box
        role="toolbar"
        aria-label={t('flows:core.headerPanel.canvasToolbar', 'Canvas toolbar')}
        sx={{
          display: 'flex',
          alignItems: 'center',
          gap: 0.5,
          px: 2,
          py: 0.5,
          bgcolor: 'background.paper',
          borderRadius: 1,
          boxShadow: '0 8px 32px rgba(0,0,0,0.18), 0 2px 8px rgba(0,0,0,0.08)',
          border: '1px solid',
          borderColor: 'divider',
        }}
      >
        {showHistoryControls && (
          <>
            <Tooltip title={t('flows:core.headerPanel.undoTooltip', 'Undo (Ctrl+Z / Cmd+Z)')}>
              <span>
                <IconButton
                  size="small"
                  onClick={onUndo}
                  disabled={!canUndo || !onUndo}
                  sx={{borderRadius: 1, color: 'text.secondary'}}
                  aria-label={t('flows:core.headerPanel.undo', 'Undo')}
                >
                  <Undo2 size={16} />
                </IconButton>
              </span>
            </Tooltip>

            <Tooltip title={t('flows:core.headerPanel.redoTooltip', 'Redo (Ctrl+Shift+Z / Cmd+Shift+Z)')}>
              <span>
                <IconButton
                  size="small"
                  onClick={onRedo}
                  disabled={!canRedo || !onRedo}
                  sx={{borderRadius: 1, color: 'text.secondary'}}
                  aria-label={t('flows:core.headerPanel.redo', 'Redo')}
                >
                  <Redo2 size={16} />
                </IconButton>
              </span>
            </Tooltip>

            <ToolbarDivider />
          </>
        )}

        <Tooltip title={t('flows:core.headerPanel.autoLayout')}>
          <IconButton
            size="small"
            onClick={onAutoLayout}
            sx={{borderRadius: 1, color: 'text.secondary'}}
            aria-label={t('flows:core.headerPanel.autoLayout')}
          >
            <LayoutGrid size={16} />
          </IconButton>
        </Tooltip>

        <ToolbarDivider />

        <Tooltip title={t('flows:core.headerPanel.edgeStyleTooltip')}>
          <IconButton
            size="small"
            onClick={handleEdgeStyleClick}
            sx={{borderRadius: 1, color: 'text.secondary'}}
            aria-label={t('flows:core.headerPanel.edgeStyleTooltip')}
            aria-haspopup="true"
            aria-expanded={Boolean(anchorEl)}
          >
            {getEdgeStyleIcon(edgeStyle)}
          </IconButton>
        </Tooltip>

        <ToolbarDivider />

        <Tooltip
          title={
            isVerboseMode
              ? t('flows:core.headerPanel.compactViewTooltip', 'Switch to compact view')
              : t('flows:core.headerPanel.detailedViewTooltip', 'Switch to detailed view')
          }
        >
          <IconButton
            size="small"
            onClick={() => setIsVerboseMode((prev) => !prev)}
            sx={{borderRadius: 1, color: 'text.secondary'}}
            // The name stays fixed because `aria-pressed` already carries the
            // state; an action name that flips with it would announce
            // "switch to detailed view, pressed" while compact view is on.
            aria-label={t('flows:core.headerPanel.compactView', 'Compact view')}
            aria-pressed={!isVerboseMode}
          >
            {isVerboseMode ? <Shrink size={16} /> : <Expand size={16} />}
          </IconButton>
        </Tooltip>

        <ToolbarDivider />

        <Tooltip
          title={
            isSnapToGridEnabled
              ? t('flows:core.headerPanel.snapToGridDisableTooltip', 'Disable snap to grid')
              : t('flows:core.headerPanel.snapToGridEnableTooltip', 'Enable snap to grid')
          }
        >
          <IconButton
            size="small"
            onClick={() => setIsSnapToGridEnabled((prev) => !prev)}
            sx={{borderRadius: 1, color: isSnapToGridEnabled ? 'primary.main' : 'text.secondary'}}
            aria-label={t('flows:core.headerPanel.snapToGrid', 'Snap to grid')}
            aria-pressed={isSnapToGridEnabled}
          >
            <Magnet size={16} />
          </IconButton>
        </Tooltip>

        <Tooltip
          title={
            isMiniMapVisible
              ? t('flows:core.headerPanel.miniMapHideTooltip', 'Hide minimap')
              : t('flows:core.headerPanel.miniMapShowTooltip', 'Show minimap')
          }
        >
          <IconButton
            size="small"
            onClick={() => setIsMiniMapVisible((prev) => !prev)}
            sx={{borderRadius: 1, color: isMiniMapVisible ? 'primary.main' : 'text.secondary'}}
            aria-label={t('flows:core.headerPanel.miniMap', 'Minimap')}
            aria-pressed={isMiniMapVisible}
          >
            <Map size={16} />
          </IconButton>
        </Tooltip>

        <ToolbarDivider />

        <Tooltip title={t('flows:core.headerPanel.zoomOut', 'Zoom out')}>
          <IconButton
            size="small"
            onClick={() => {
              void zoomOut();
            }}
            sx={{borderRadius: 1, color: 'text.secondary'}}
            aria-label={t('flows:core.headerPanel.zoomOut', 'Zoom out')}
          >
            <Minus size={12} />
          </IconButton>
        </Tooltip>

        <Tooltip title={t('flows:core.headerPanel.zoomIn', 'Zoom in')}>
          <IconButton
            size="small"
            onClick={() => {
              void zoomIn();
            }}
            sx={{borderRadius: 1, color: 'text.secondary'}}
            aria-label={t('flows:core.headerPanel.zoomIn', 'Zoom in')}
          >
            <Plus size={12} />
          </IconButton>
        </Tooltip>

        <ToolbarDivider />

        <Tooltip title={t('flows:core.headerPanel.fitView', 'Fit view')}>
          <IconButton
            size="small"
            onClick={() => {
              void fitView({padding: 0.2, duration: 300});
            }}
            sx={{borderRadius: 1, color: 'text.secondary'}}
            aria-label={t('flows:core.headerPanel.fitView', 'Fit view')}
          >
            <Maximize size={14} />
          </IconButton>
        </Tooltip>
      </Box>

      <EdgeStyleMenu anchorEl={anchorEl} onClose={handleEdgeStyleClose} />
    </>
  );
}
