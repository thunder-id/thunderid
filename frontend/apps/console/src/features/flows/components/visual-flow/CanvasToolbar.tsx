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

import {Box, IconButton, Tooltip} from '@wso2/oxygen-ui';
import {LayoutGrid, Maximize, Minus, Plus, Redo2, Undo2} from '@wso2/oxygen-ui-icons-react';
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
  const {edgeStyle} = useFlowConfig();
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
            <Tooltip title={t('flows:core.headerPanel.undoTooltip', 'Undo (Ctrl+Z)')}>
              <span>
                <IconButton
                  size="small"
                  onClick={onUndo}
                  disabled={!canUndo}
                  sx={{borderRadius: 1, color: 'text.secondary'}}
                  aria-label={t('flows:core.headerPanel.undo', 'Undo')}
                >
                  <Undo2 size={16} />
                </IconButton>
              </span>
            </Tooltip>

            <Tooltip title={t('flows:core.headerPanel.redoTooltip', 'Redo (Ctrl+Shift+Z)')}>
              <span>
                <IconButton
                  size="small"
                  onClick={onRedo}
                  disabled={!canRedo}
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
