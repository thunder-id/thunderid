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

import {useStore, useStoreApi} from '@xyflow/react';
import {useMemo, useRef, useState, type PropsWithChildren, type ReactElement} from 'react';
import EdgeGeometryContext, {type EdgeGeometryRegistry} from '../../context/EdgeGeometryContext';
import EdgePathsContext from '../../context/EdgePathsContext';
import useFlowConfig from '../../hooks/useFlowConfig';
import {
  calculateAllEdgePaths,
  type EdgeInput,
  type EdgePathResult,
  type EdgeStyle,
} from '../../utils/calculateEdgePath';
import {DRAGGING_OBSTACLES_KEY, SMOOTH_STEP_BORDER_RADIUS, selectObstaclesKey} from '../../utils/edgeRoutingKeys';

function sameGeometry(a: EdgeInput, b: EdgeInput): boolean {
  return (
    a.sourceX === b.sourceX &&
    a.sourceY === b.sourceY &&
    a.targetX === b.targetX &&
    a.targetY === b.targetY &&
    a.sourcePosition === b.sourcePosition &&
    a.targetPosition === b.targetPosition
  );
}

/**
 * Routes all edges together so overlapping segments separate into parallel
 * lanes (calculateAllEdgePaths), instead of each edge routing independently
 * and stacking on the same corridor.
 *
 * Each edge registers the endpoint geometry React Flow handed it, so the
 * combined routing works from exactly the coordinates the edges would use on
 * their own. Until an edge's geometry is registered (first paint), it renders
 * its individual path and picks up the separated one on the next pass.
 */
function EdgePathsProvider({children}: PropsWithChildren): ReactElement {
  const {edgeStyle} = useFlowConfig();
  const obstaclesKey = useStore(selectObstaclesKey);
  const store = useStoreApi();

  const geometryRef = useRef<Map<string, EdgeInput>>(new Map());
  const [version, setVersion] = useState<number>(0);

  const registry = useMemo(
    (): EdgeGeometryRegistry => ({
      register: (input: EdgeInput): void => {
        const current = geometryRef.current.get(input.id);
        if (current && sameGeometry(current, input)) {
          return;
        }
        geometryRef.current.set(input.id, input);
        setVersion((previous) => previous + 1);
      },
      unregister: (id: string): void => {
        if (geometryRef.current.delete(id)) {
          setVersion((previous) => previous + 1);
        }
      },
    }),
    [],
  );

  const paths = useMemo((): Map<string, EdgePathResult> | null => {
    // While dragging, edges render their cheap built-in paths; recompute on drop.
    if (obstaclesKey === DRAGGING_OBSTACLES_KEY || geometryRef.current.size < 2) {
      return null;
    }
    const {nodes} = store.getState();
    return calculateAllEdgePaths(
      [...geometryRef.current.values()],
      nodes,
      edgeStyle as EdgeStyle,
      SMOOTH_STEP_BORDER_RADIUS,
    );
    // `version` tracks the registry content, which lives in a ref.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [version, obstaclesKey, edgeStyle, store]);

  return (
    <EdgeGeometryContext.Provider value={registry}>
      <EdgePathsContext.Provider value={paths}>{children}</EdgePathsContext.Provider>
    </EdgeGeometryContext.Provider>
  );
}

export default EdgePathsProvider;
