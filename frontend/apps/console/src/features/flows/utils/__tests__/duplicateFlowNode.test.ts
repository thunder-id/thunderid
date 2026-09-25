// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Node} from '@xyflow/react';
import {describe, expect, it, vi} from 'vitest';
import {EXECUTION_STACK_NODE_TYPE} from '../compactGraphTransforms';
import {
  DUPLICATE_NODE_OFFSET,
  canDuplicateNode,
  collectCanvasIds,
  duplicateFlowNode,
  generateUniqueResourceId,
} from '../duplicateFlowNode';

const {generateResourceIdMock} = vi.hoisted(() => ({generateResourceIdMock: vi.fn<(prefix?: string) => string>()}));

vi.mock('../generateResourceId', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../generateResourceId')>();
  generateResourceIdMock.mockImplementation(actual.default);
  return {default: generateResourceIdMock};
});

const baseNode: Node = {
  id: 'view_abcd',
  type: 'VIEW',
  position: {x: 100, y: 200},
  data: {},
};

describe('canDuplicateNode', () => {
  it('should allow duplicating a regular node', () => {
    expect(canDuplicateNode(baseNode)).toBe(true);
    expect(canDuplicateNode({...baseNode, deletable: true})).toBe(true);
  });

  it('should not allow duplicating a non-deletable node', () => {
    expect(canDuplicateNode({...baseNode, deletable: false})).toBe(false);
  });

  it('should not allow duplicating a synthetic execution stack node', () => {
    expect(canDuplicateNode({...baseNode, type: EXECUTION_STACK_NODE_TYPE})).toBe(false);
  });
});

describe('collectCanvasIds', () => {
  it('should collect node ids and nested component ids', () => {
    const nodes: Node[] = [
      {
        id: 'view_1',
        type: 'VIEW',
        position: {x: 0, y: 0},
        data: {
          components: [
            {id: 'form_1', type: 'FORM', components: [{id: 'input_1', type: 'INPUT'}]},
            {id: 'button_1', type: 'BUTTON'},
          ],
        },
      },
      {id: 'end', type: 'END', position: {x: 0, y: 0}, data: {}},
    ];

    expect([...collectCanvasIds(nodes)].sort()).toEqual(['button_1', 'end', 'form_1', 'input_1', 'view_1']);
  });
});

describe('generateUniqueResourceId', () => {
  it('should retry until the generated id is not taken', () => {
    generateResourceIdMock.mockReturnValueOnce('view_taken').mockReturnValueOnce('view_fresh');

    expect(generateUniqueResourceId('view', new Set(['view_taken']))).toBe('view_fresh');
  });
});

describe('duplicateFlowNode', () => {
  it('should generate a fresh id derived from the node type', () => {
    const copy = duplicateFlowNode(baseNode, new Set([baseNode.id]));

    expect(copy.id).not.toBe(baseNode.id);
    expect(copy.id.startsWith('view_')).toBe(true);
  });

  it('should offset the copy from the source node', () => {
    const copy = duplicateFlowNode(baseNode, new Set([baseNode.id]));

    expect(copy.position).toEqual({
      x: baseNode.position.x + DUPLICATE_NODE_OFFSET,
      y: baseNode.position.y + DUPLICATE_NODE_OFFSET,
    });
  });

  it('should return the copy selected and not dragging', () => {
    const copy = duplicateFlowNode({...baseNode, selected: false, dragging: true}, new Set([baseNode.id]));

    expect(copy.selected).toBe(true);
    expect(copy.dragging).toBe(false);
  });

  it('should deep-clone the node data so the copy is detached from the source', () => {
    const node: Node = {
      ...baseNode,
      data: {properties: {displayName: 'My View'}},
    };

    const copy = duplicateFlowNode(node, new Set([node.id]));

    expect(copy.data).not.toBe(node.data);
    expect(copy.data.properties).not.toBe(node.data.properties);
    expect(copy.data.properties).toEqual({displayName: 'My View'});
  });

  it('should keep configuration but regenerate component ids recursively', () => {
    const node: Node = {
      ...baseNode,
      data: {
        components: [
          {
            id: 'form_1111',
            type: 'FORM',
            components: [
              {id: 'input_2222', type: 'INPUT', config: {identifier: 'username'}},
              {id: 'button_3333', type: 'BUTTON'},
            ],
          },
          {id: 'richtext_4444', type: 'RICH_TEXT', label: 'Hello'},
        ],
      },
    };

    const copy = duplicateFlowNode(node, collectCanvasIds([node]));
    const components = copy.data.components as {
      id: string;
      type: string;
      label?: string;
      config?: {identifier: string};
      components?: {id: string; type: string; config?: {identifier: string}}[];
    }[];

    expect(components[0].id).not.toBe('form_1111');
    expect(components[0].id.startsWith('form_')).toBe(true);
    expect(components[0].components?.[0].id).not.toBe('input_2222');
    expect(components[0].components?.[0].id.startsWith('input_')).toBe(true);
    expect(components[0].components?.[0].config).toEqual({identifier: 'username'});
    expect(components[0].components?.[1].id).not.toBe('button_3333');
    expect(components[1].id).not.toBe('richtext_4444');
    expect(components[1].label).toBe('Hello');
  });

  it('should not reuse an id that is already taken on the canvas', () => {
    const existingIds = ['view_abcd', 'view_efgh'];

    const copy = duplicateFlowNode(baseNode, new Set(existingIds));

    expect(existingIds).not.toContain(copy.id);
  });

  it('should register every generated id into the shared set so successive copies cannot collide', () => {
    const node: Node = {
      ...baseNode,
      data: {components: [{id: 'input_1', type: 'INPUT'}]},
    };
    const takenIds = collectCanvasIds([node]);

    const first = duplicateFlowNode(node, takenIds);
    const second = duplicateFlowNode(node, takenIds);

    const allGeneratedIds = [first, second].flatMap((copy) => [
      copy.id,
      ...(copy.data.components as {id: string}[]).map((component) => component.id),
    ]);

    expect(new Set(allGeneratedIds).size).toBe(allGeneratedIds.length);
    allGeneratedIds.forEach((id) => expect(takenIds.has(id)).toBe(true));
  });

  it('should keep the node type and other node fields', () => {
    const node: Node = {...baseNode, deletable: true, measured: {width: 200, height: 100}};

    const copy = duplicateFlowNode(node, new Set([node.id]));

    expect(copy.type).toBe('VIEW');
    expect(copy.deletable).toBe(true);
    expect(copy.measured).toEqual({width: 200, height: 100});
  });
});
