// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import templates from '../templates.json';
import widgets from '../widgets.json';

interface Component {
  id?: string;
  ref?: string;
  type?: string;
  required?: boolean;
  hint?: string;
  options?: unknown[];
  components?: Component[];
}

interface Prompt {
  action?: {nextNode?: string; ref?: string};
  inputs?: {identifier: string; ref: string; type: string; required: boolean}[];
}

interface Node {
  id: string;
  type: string;
  onSuccess?: string;
  onFailure?: string;
  onIncomplete?: string;
  properties?: Record<string, unknown>;
  meta?: {components?: Component[]};
  prompts?: Prompt[];
  layout?: {position?: {x: number; y: number}};
}

interface Template {
  type?: string;
  config?: {name?: string; handle?: string; nodes?: Node[]};
}

interface WidgetStep {
  data?: {components?: Component[]};
}

interface Widget {
  type?: string;
  config?: {data?: {steps?: WidgetStep[]}};
}

const agentTemplate = (type: string): Template => {
  const found = (templates as Template[]).find((template) => template.type === type);

  expect(found, `${type} template should exist`).toBeDefined();

  return found ?? {};
};

const nodesOf = (template: Template): Map<string, Node> =>
  new Map((template.config?.nodes ?? []).map((node) => [node.id, node]));

const flatten = (components: Component[] = []): Component[] =>
  components.flatMap((component) => [component, ...flatten(component.components)]);

const widgetFields = (type: string): Component[] => {
  const widget = (widgets as Widget[]).find((candidate) => candidate.type === type);

  return flatten(widget?.config?.data?.steps?.[0]?.data?.components).filter((component) => component.ref);
};

/** The picker-and-text fields of a prompt node, keyed by the identifier they submit. */
const promptFields = (node: Node): Component[] => flatten(node.meta?.components).filter((component) => component.ref);

describe('agent onboarding templates', () => {
  const ownIdentity = nodesOf(agentTemplate('AGENT_ONBOARDING_OWN_IDENTITY'));
  const obo = nodesOf(agentTemplate('AGENT_ONBOARDING_OBO'));
  const generic = nodesOf(agentTemplate('AGENT_ONBOARDING_GENERIC'));

  it('should replace the single agent template with the three scenarios', () => {
    const types = (templates as Template[]).map((template) => template.type);

    expect(types).not.toContain('AGENT_ONBOARDING');
    expect(types.filter((type) => type?.startsWith('AGENT_ONBOARDING'))).toEqual([
      'AGENT_ONBOARDING_OWN_IDENTITY',
      'AGENT_ONBOARDING_OBO',
      'AGENT_ONBOARDING_GENERIC',
    ]);
  });

  it('should give each template its own handle', () => {
    const handles = ['AGENT_ONBOARDING_OWN_IDENTITY', 'AGENT_ONBOARDING_OBO', 'AGENT_ONBOARDING_GENERIC'].map(
      (type) => agentTemplate(type).config?.handle,
    );

    expect(new Set(handles).size).toBe(3);
  });

  // The mode is stated on every provisioning node so it never depends on an omitted property.
  it('should set the delegation property each scenario calls for', () => {
    expect(ownIdentity.get('provisioning')?.properties?.delegated).toBe(false);
    expect(obo.get('provisioning')?.properties?.delegated).toBe(true);
    // The generic template only falls back to own identity; the collected choice wins.
    expect(generic.get('provisioning')?.properties?.delegated).toBe(false);
  });

  it('should keep own identity free of on-behalf-of prompts', () => {
    expect(ownIdentity.has('prompt_obo_details')).toBe(false);
    expect(ownIdentity.has('prompt_agent_delegation')).toBe(false);
    expect(ownIdentity.get('owner_resolution')?.onSuccess).toBe('prompt_agent_name');
  });

  // Every template asks for the agent details before provisioning, so the choices on that prompt are
  // fixed in the flow and the details are checked for uniqueness before anything is created.
  it('should collect and check the agent details before provisioning', () => {
    expect(ownIdentity.get('owner_resolution')?.onSuccess).toBe('prompt_agent_name');
    expect(ownIdentity.get('prompt_agent_name')?.prompts?.[0]?.action?.nextNode).toBe('prompt_agent_details');
    expect(ownIdentity.get('prompt_agent_details')?.prompts?.[0]?.action?.nextNode).toBe('agent_attribute_uniqueness');
    expect(ownIdentity.get('agent_attribute_uniqueness')?.onSuccess).toBe('provisioning');
  });

  // Owner, name, agent details, the uniqueness check, then the prompt that decides or details the
  // mode, then provisioning. Anything provisioning still needs is requested on a dynamic prompt that
  // renders whatever inputs the executor forwards and returns to provisioning.
  it('should ask for the on-behalf-of details after the agent details and before provisioning', () => {
    [
      [obo, 'prompt_obo_details'],
      [generic, 'prompt_agent_delegation'],
    ].forEach(([nodes, promptId]) => {
      const flow = nodes as Map<string, Node>;

      expect(flow.get('owner_resolution')?.onSuccess).toBe('prompt_agent_name');
      expect(flow.get('prompt_agent_name')?.prompts?.[0]?.action?.nextNode).toBe('prompt_agent_details');
      expect(flow.get('prompt_agent_details')?.prompts?.[0]?.action?.nextNode).toBe('agent_attribute_uniqueness');
      expect(flow.get('agent_attribute_uniqueness')?.onSuccess).toBe(promptId);
      expect(flow.get('agent_attribute_uniqueness')?.onIncomplete).toBe('prompt_agent_details');
      expect(flow.get(promptId as string)?.prompts?.[0]?.action?.nextNode).toBe('provisioning');
    });
  });

  it('should collect missing inputs on a dynamic prompt in the on-behalf-of templates', () => {
    [obo, generic].forEach((nodes) => {
      expect(nodes.get('provisioning')?.onIncomplete).toBe('prompt_missing_inputs');

      const prompt = nodes.get('prompt_missing_inputs');

      expect(prompt?.prompts?.[0]?.action?.nextNode).toBe('provisioning');
      expect(
        flatten(prompt?.meta?.components).some((component) => component.type === 'DYNAMIC_INPUT_PLACEHOLDER'),
      ).toBe(true);
    });
  });

  // The choices come from the default agent type's schema, whose values the agent service validates at
  // creation. They are written into the flow so the prompt shows them without the executor
  // supplying them, and an author edits them in the flow definition.
  it('should fix the model provider and function choices on the details prompt', () => {
    [ownIdentity, obo, generic].forEach((nodes) => {
      const selects = promptFields(nodes.get('prompt_agent_details')!).filter(
        (component) => component.type === 'SELECT',
      );
      const options = Object.fromEntries(selects.map((component) => [component.ref ?? '', component.options ?? []]));

      expect(options.modelProvider).toEqual(['openai', 'anthropic', 'gemini', 'mistral', 'custom']);
      expect(options.function).toEqual([
        'task-automation',
        'rag-retrieval',
        'code-gen',
        'data-analysis',
        'orchestrator',
        'sub-agent',
        'assistant',
        'custom',
      ]);
    });
  });

  // The name has its own prompt and is validated by the agent service at creation, so a provisioning
  // failure returns to it in every template.
  it('should route a provisioning failure to the name prompt in every template', () => {
    [ownIdentity, obo, generic].forEach((nodes) => {
      expect(nodes.get('provisioning')?.onFailure).toBe('prompt_agent_name');
    });
  });

  it('should keep the details prompt as the incomplete path in own identity', () => {
    expect(ownIdentity.get('provisioning')?.onIncomplete).toBe('prompt_agent_details');
  });

  it('should resolve the agent type in every template', () => {
    [ownIdentity, obo, generic].forEach((nodes) => expect(nodes.get('agent_type_resolution')).toBeDefined());
  });

  it('should point every connection at a node that exists', () => {
    [ownIdentity, obo, generic].forEach((nodes) => {
      nodes.forEach((node) => {
        [node.onSuccess, node.onFailure, node.onIncomplete, ...(node.prompts ?? []).map((p) => p.action?.nextNode)]
          .filter((target): target is string => Boolean(target))
          .forEach((target) => expect(nodes.has(target), `${node.id} -> ${target}`).toBe(true));
      });
    });
  });

  it('should not stack two nodes on the same spot after inserting the prompt', () => {
    [obo, generic].forEach((nodes) => {
      const positions = [...nodes.values()].map((node) => `${node.layout?.position?.x},${node.layout?.position?.y}`);

      expect(new Set(positions).size).toBe(positions.length);
    });
  });

  describe('on-behalf-of prompt', () => {
    const prompt = obo.get('prompt_obo_details')!;

    it('should require all three details and offer no delegation choice', () => {
      const inputs = prompt.prompts?.[0]?.inputs ?? [];

      expect(inputs.map((input) => [input.identifier, input.type, input.required])).toEqual([
        ['redirectUris', 'TEXT_INPUT', true],
        ['authFlowId', 'AUTH_FLOW_SELECT', true],
        ['allowedUserTypes', 'USER_TYPE_SELECT', true],
      ]);
      expect(promptFields(prompt).some((component) => component.ref === 'delegated')).toBe(false);
    });

    // The template carries its own copy of the prompt, so the widget it was built from must not
    // drift: same fields, same types, same requirement.
    it('should match the Delegated Agent Details widget', () => {
      const shape = (fields: Component[]) => fields.map(({ref, type, required}) => ({ref, type, required}));

      expect(shape(promptFields(prompt))).toEqual(shape(widgetFields('AGENT_OBO_DETAILS')));
    });

    it('should say why each detail is asked for', () => {
      promptFields(prompt).forEach((component) => expect(component.hint, component.ref).toBeTruthy());
    });
  });

  describe('delegation choice prompt', () => {
    const prompt = generic.get('prompt_agent_delegation')!;

    it('should offer the choice and leave every input optional', () => {
      const inputs = prompt.prompts?.[0]?.inputs ?? [];

      expect(inputs.map((input) => [input.identifier, input.type, input.required])).toEqual([
        ['delegated', 'BOOLEAN_INPUT', false],
        ['redirectUris', 'TEXT_INPUT', false],
        ['authFlowId', 'AUTH_FLOW_SELECT', false],
        ['allowedUserTypes', 'USER_TYPE_SELECT', false],
      ]);
    });

    it('should match the Choose Agent Mode widget', () => {
      const shape = (fields: Component[]) => fields.map(({ref, type, required}) => ({ref, type, required}));

      expect(shape(promptFields(prompt))).toEqual(shape(widgetFields('AGENT_DELEGATION')));
    });
  });
});
