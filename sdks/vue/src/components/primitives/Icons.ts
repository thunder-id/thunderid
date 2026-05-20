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

import {h, type VNode} from 'vue';

const defaultProps: Record<string, string> = {
  fill: 'none',
  height: '16',
  stroke: 'currentColor',
  'stroke-linecap': 'round',
  'stroke-linejoin': 'round',
  'stroke-width': '2',
  viewBox: '0 0 24 24',
  width: '16',
  xmlns: 'http://www.w3.org/2000/svg',
};

const icon = (paths: VNode[]): VNode => h('svg', {...defaultProps}, paths);

export const CheckIcon = (): VNode => icon([h('polyline', {points: '20 6 9 17 4 12'})]);

export const XIcon = (): VNode =>
  icon([h('line', {x1: '18', x2: '6', y1: '6', y2: '18'}), h('line', {x1: '6', x2: '18', y1: '6', y2: '18'})]);

export const EyeIcon = (): VNode =>
  icon([h('path', {d: 'M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z'}), h('circle', {cx: '12', cy: '12', r: '3'})]);

export const EyeOffIcon = (): VNode =>
  icon([
    h('path', {d: 'M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94'}),
    h('path', {d: 'M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19'}),
    h('line', {x1: '1', x2: '23', y1: '1', y2: '23'}),
  ]);

export const CircleAlertIcon = (): VNode =>
  icon([
    h('circle', {cx: '12', cy: '12', r: '10'}),
    h('line', {x1: '12', x2: '12', y1: '8', y2: '12'}),
    h('line', {x1: '12', x2: '12.01', y1: '16', y2: '16'}),
  ]);

export const CircleCheckIcon = (): VNode =>
  icon([h('path', {d: 'M22 11.08V12a10 10 0 1 1-5.93-9.14'}), h('polyline', {points: '22 4 12 14.01 9 11.01'})]);

export const InfoIcon = (): VNode =>
  icon([
    h('circle', {cx: '12', cy: '12', r: '10'}),
    h('line', {x1: '12', x2: '12', y1: '16', y2: '12'}),
    h('line', {x1: '12', x2: '12.01', y1: '8', y2: '8'}),
  ]);

export const TriangleAlertIcon = (): VNode =>
  icon([
    h('path', {d: 'M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z'}),
    h('line', {x1: '12', x2: '12', y1: '9', y2: '13'}),
    h('line', {x1: '12', x2: '12.01', y1: '17', y2: '17'}),
  ]);

export const PlusIcon = (): VNode =>
  icon([h('line', {x1: '12', x2: '12', y1: '5', y2: '19'}), h('line', {x1: '5', x2: '19', y1: '12', y2: '12'})]);

export const LogOutIcon = (): VNode =>
  icon([
    h('path', {d: 'M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4'}),
    h('polyline', {points: '16 17 21 12 16 7'}),
    h('line', {x1: '21', x2: '9', y1: '12', y2: '12'}),
  ]);

export const UserIcon = (): VNode =>
  icon([h('path', {d: 'M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2'}), h('circle', {cx: '12', cy: '7', r: '4'})]);

export const ArrowLeftRightIcon = (): VNode =>
  icon([
    h('polyline', {points: '7 16 3 12 7 8'}),
    h('line', {x1: '21', x2: '3', y1: '12', y2: '12'}),
    h('polyline', {points: '17 8 21 12 17 16'}),
  ]);

export const BuildingIcon = (): VNode =>
  icon([
    h('rect', {height: '20', rx: '2', ry: '2', width: '16', x: '4', y: '2'}),
    h('line', {x1: '9', x2: '9', y1: '6', y2: '6.01'}),
    h('line', {x1: '15', x2: '15', y1: '6', y2: '6.01'}),
    h('line', {x1: '9', x2: '9', y1: '10', y2: '10.01'}),
    h('line', {x1: '15', x2: '15', y1: '10', y2: '10.01'}),
    h('line', {x1: '9', x2: '9', y1: '14', y2: '14.01'}),
    h('line', {x1: '15', x2: '15', y1: '14', y2: '14.01'}),
    h('line', {x1: '9', x2: '15', y1: '18', y2: '18'}),
  ]);

export const ChevronDownIcon = (): VNode => icon([h('polyline', {points: '6 9 12 15 18 9'})]);

export const GlobeIcon = (): VNode =>
  icon([
    h('circle', {cx: '12', cy: '12', r: '10'}),
    h('line', {x1: '2', x2: '22', y1: '12', y2: '12'}),
    h('path', {d: 'M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z'}),
  ]);

export const PencilIcon = (): VNode =>
  icon([h('path', {d: 'M17 3a2.828 2.828 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5L17 3z'})]);
