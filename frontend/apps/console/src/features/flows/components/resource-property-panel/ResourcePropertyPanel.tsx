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

import {BuilderStaticPanel} from '@thunderid/components';
import {Box, IconButton} from '@wso2/oxygen-ui';
import {X, TrashIcon} from '@wso2/oxygen-ui-icons-react';
import {useReactFlow} from '@xyflow/react';
import {memo, useCallback, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import PanelActionButton from './PanelActionButton';
import ResourceProperties from './ResourceProperties';
import useInteractionState from '../../hooks/useInteractionState';
import useUIPanelState from '../../hooks/useUIPanelState';
import {type Element} from '../../models/elements';
import {ResourceTypes} from '../../models/resources';

/**
 * Props interface of {@link ResourcePropertyPanel}
 */
export interface ResourcePropertyPanelPropsInterface {
  open?: boolean;
  onComponentDelete: (stepId: string, component: Element) => void;
}

/**
 * Component to render the resource property panel as a static side panel.
 *
 * @param props - Props injected to the component.
 * @returns The ResourcePropertyPanel component.
 */
function ResourcePropertyPanel({open = false, onComponentDelete}: ResourcePropertyPanelPropsInterface): ReactElement {
  const {deleteElements} = useReactFlow();
  const {t} = useTranslation();

  const {resourcePropertiesPanelHeading, setIsOpenResourcePropertiesPanel} = useUIPanelState();
  const {lastInteractedStepId, lastInteractedResource} = useInteractionState();

  const handleClose = useCallback(() => {
    setIsOpenResourcePropertiesPanel(false);
  }, [setIsOpenResourcePropertiesPanel]);

  const handleDelete = useCallback(() => {
    if (!lastInteractedResource) return;

    if (lastInteractedResource.resourceType === ResourceTypes.Step) {
      deleteElements({nodes: [{id: lastInteractedResource.id}]}).catch(() => {
        // Deletion may fail silently if the node doesn't exist or is protected
      });
    } else {
      onComponentDelete(lastInteractedStepId, lastInteractedResource as Element);
    }
    setIsOpenResourcePropertiesPanel(false);
  }, [
    deleteElements,
    lastInteractedResource,
    lastInteractedStepId,
    onComponentDelete,
    setIsOpenResourcePropertiesPanel,
  ]);

  return (
    <BuilderStaticPanel
      open={open}
      width={350}
      anchor="right"
      header={
        <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%'}}>
          {resourcePropertiesPanelHeading}
          <IconButton onClick={handleClose} size="small" aria-label="Close properties panel">
            <X height={16} width={16} />
          </IconButton>
        </Box>
      }
    >
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          overflowY: 'auto',
          overflowX: 'hidden',
          '&::-webkit-scrollbar': {
            width: '6px',
          },
          '&::-webkit-scrollbar-track': {
            background: 'transparent',
          },
          '&::-webkit-scrollbar-thumb': {
            background: 'rgba(0, 0, 0, 0.2)',
            borderRadius: '3px',
            '&:hover': {
              background: 'rgba(0, 0, 0, 0.3)',
            },
          },
        }}
      >
        <ResourceProperties />
      </Box>
      {lastInteractedResource && lastInteractedResource.deletable !== false && (
        <Box flexShrink={0}>
          <PanelActionButton accent="error" onClick={handleDelete} startIcon={<TrashIcon size={16} />}>
            {t('flows:core.propertiesPanel.delete', 'Delete')}
          </PanelActionButton>
        </Box>
      )}
    </BuilderStaticPanel>
  );
}

export default memo(ResourcePropertyPanel);
