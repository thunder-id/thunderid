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

import {InputBase, Tooltip, Typography} from '@wso2/oxygen-ui';
import {useNodeId} from '@xyflow/react';
import {useState, type KeyboardEvent, type MouseEvent, type ReactElement} from 'react';
import {useTranslation} from 'react-i18next';
import useRenameStep from '@/features/flows/hooks/useRenameStep';

/**
 * Props interface of {@link StepTitle}
 */
export interface StepTitleProps {
  /**
   * Fallback label shown when the step id is unavailable (e.g. outside a
   * React Flow node context).
   */
  label: string;
}

/**
 * Step header title showing the step's id — the step's identity in the flow
 * definition; the type of the step is already conveyed by the node's body.
 * Double-click edits the id, and committing a change rewires every edge
 * (source, target, and id-prefixed source handles) that references it.
 *
 * @param props - Props injected to the component.
 * @returns The StepTitle component.
 */
function StepTitle({label}: StepTitleProps): ReactElement {
  const {t} = useTranslation();
  const stepId = useNodeId();
  const {isValidStepId, renameStep} = useRenameStep();
  const [draft, setDraft] = useState<string | null>(null);
  const [isInvalid, setIsInvalid] = useState<boolean>(false);

  const isValid = (candidate: string): boolean => stepId !== null && isValidStepId(candidate, stepId);

  const commit = (): void => {
    if (draft === null || !stepId) {
      return;
    }
    const next = draft.trim();
    if (next === stepId || next === '') {
      setDraft(null);
      setIsInvalid(false);
      return;
    }
    if (!renameStep(stepId, next)) {
      setIsInvalid(true);
      return;
    }
    setDraft(null);
    setIsInvalid(false);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>): void => {
    if (event.key === 'Enter') {
      commit();
    }
    if (event.key === 'Escape') {
      setDraft(null);
      setIsInvalid(false);
    }
  };

  if (draft !== null) {
    return (
      <InputBase
        // eslint-disable-next-line jsx-a11y/no-autofocus -- the input replaces the double-clicked title; focus must follow
        autoFocus
        className="nodrag"
        value={draft}
        error={isInvalid}
        onChange={(event) => {
          setDraft(event.target.value);
          setIsInvalid(false);
        }}
        onBlur={() => {
          // An invalid draft is discarded on blur rather than kept in error state.
          if (draft.trim() === stepId || isValid(draft.trim())) {
            commit();
          } else {
            setDraft(null);
            setIsInvalid(false);
          }
        }}
        onKeyDown={handleKeyDown}
        onFocus={(event) => event.target.select()}
        inputProps={{'aria-label': t('flows:core.steps.stepId', 'Step ID'), 'aria-invalid': isInvalid}}
        sx={{
          color: 'common.white',
          fontSize: 'body2.fontSize',
          fontWeight: 600,
          py: 0,
          '& input': {py: 0.25},
          ...(isInvalid && {borderBottom: '2px solid', borderColor: 'error.main'}),
        }}
      />
    );
  }

  // Without a node context (e.g. palette previews) there is no id to edit —
  // render a plain, non-editable title.
  if (!stepId) {
    return (
      <Typography variant="body2" sx={{color: 'common.white', fontWeight: 600}}>
        {label}
      </Typography>
    );
  }

  return (
    <Tooltip title={t('flows:core.steps.renameTooltip', 'Double-click to edit the step ID')} enterDelay={500}>
      <Typography
        variant="body2"
        onDoubleClick={(event: MouseEvent<HTMLElement>) => {
          event.stopPropagation();
          setDraft(stepId);
        }}
        sx={{color: 'common.white', fontWeight: 600, cursor: 'text'}}
      >
        {stepId}
      </Typography>
    </Tooltip>
  );
}

export default StepTitle;
