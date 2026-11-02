// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import widgets from '../widgets.json';

interface WidgetComponent {
  id?: string;
  type?: string;
  ref?: string;
  required?: boolean;
  eventType?: string;
  components?: WidgetComponent[];
  action?: {onSuccess?: string};
  options?: unknown[];
  hint?: string;
}

interface WidgetStep {
  id?: string;
  type?: string;
  data?: {
    action?: {executor?: {name?: string}; onSuccess?: string; onIncomplete?: string};
    components?: WidgetComponent[];
  };
}

interface Widget {
  type?: string;
  config?: {data?: {steps?: WidgetStep[]; __generationMeta__?: {replacers?: {key: string}[]}}};
}

const widgetOfType = (type: string): Widget => {
  const found = (widgets as Widget[]).find((widget) => widget.type === type);

  expect(found, `${type} widget should exist`).toBeDefined();

  return found!;
};

const flatten = (components: WidgetComponent[] = []): WidgetComponent[] =>
  components.flatMap((component) => [component, ...flatten(component.components)]);

const componentsOf = (step: WidgetStep): WidgetComponent[] => flatten(step.data?.components);

/** One field of every component that carries a ref, keyed by that ref. */
const fieldByRef = (
  step: WidgetStep,
  field: 'type' | 'hint' | 'required',
): Record<string, string | boolean | undefined> => {
  const result: Record<string, string | boolean | undefined> = {};

  componentsOf(step).forEach((component) => {
    if (component.ref) {
      result[component.ref] = component[field];
    }
  });

  return result;
};

describe('agent onboarding widgets', () => {
  describe('AGENT_OWNER_RESOLUTION', () => {
    const widget = widgetOfType('AGENT_OWNER_RESOLUTION');
    const steps = widget.config?.data?.steps ?? [];
    const executorStep = steps.find((step) => step.type === 'TASK_EXECUTION');
    const promptStep = steps.find((step) => step.type === 'VIEW');

    it('should ship the resolver and its prompt as one unit', () => {
      expect(steps).toHaveLength(2);
      expect(executorStep?.data?.action?.executor?.name).toBe('OwnerResolver');
      expect(promptStep).toBeDefined();
    });

    // Dropping the widget should give a working loop, not two nodes the author must wire up.
    it('should wire the resolver to the prompt and back', () => {
      expect(executorStep?.data?.action?.onIncomplete).toBe(promptStep?.id);

      const submit = componentsOf(promptStep!).find((component) => component.eventType === 'SUBMIT');

      expect(submit?.action?.onSuccess).toBe(executorStep?.id);
    });

    // USER_SELECT, not SELECT: the client sources the candidates itself, the way OU_SELECT works,
    // so the picker shows display values while the flow stores the user id. A plain SELECT would
    // have to carry options, and one string per option means showing the identifier.
    it('should collect the owner through a user picker', () => {
      const select = componentsOf(promptStep!).find((component) => component.type === 'USER_SELECT');

      expect(select?.id).toBe('owner_input');
      expect(select?.ref).toBe('owner');
      expect(select?.options).toBeUndefined();
    });

    it('should leave the owner optional so the caller can own the entity', () => {
      const select = componentsOf(promptStep!).find((component) => component.type === 'USER_SELECT');

      expect(select?.required).toBe(false);
    });
  });

  describe('AGENT_DELEGATION', () => {
    const widget = widgetOfType('AGENT_DELEGATION');
    const steps = widget.config?.data?.steps ?? [];

    it('should collect the choice and the on-behalf-of details on a single prompt', () => {
      expect(steps).toHaveLength(1);
      expect(steps[0].type).toBe('VIEW');

      const refs = componentsOf(steps[0])
        .filter((component) => component.ref)
        .map((component) => component.ref);

      expect(refs).toEqual(['delegated', 'redirectUris', 'authFlowId', 'allowedUserTypes']);
    });

    // The executor compares the submitted string against a parsed boolean, and only BOOLEAN_INPUT
    // renders as a checkbox at runtime, so the type is what makes the value usable.
    it('should collect delegation as a boolean input', () => {
      const delegated = componentsOf(steps[0]).find((component) => component.ref === 'delegated');

      expect(delegated?.type).toBe('BOOLEAN_INPUT');
    });

    // An agent acting on its own behalf needs none of the details, so none of them may block a
    // step that chose that. The executor asks for whichever is missing once delegation is on.
    it('should leave every input optional', () => {
      const inputs = componentsOf(steps[0]).filter((component) => component.ref);

      inputs.forEach((input) => expect(input.required).toBe(false));
    });

    it('should pick the login flow and the user type through the typed pickers', () => {
      const types = fieldByRef(steps[0], 'type');

      expect(types.authFlowId).toBe('AUTH_FLOW_SELECT');
      expect(types.allowedUserTypes).toBe('USER_TYPE_SELECT');
    });
  });

  describe('AGENT_OBO_DETAILS', () => {
    const widget = widgetOfType('AGENT_OBO_DETAILS');
    const steps = widget.config?.data?.steps ?? [];

    it('should collect the three details on a single prompt with no delegation choice', () => {
      expect(steps).toHaveLength(1);
      expect(steps[0].type).toBe('VIEW');

      const refs = componentsOf(steps[0])
        .filter((component) => component.ref)
        .map((component) => component.ref);

      // The mode comes from the provisioning node, so the prompt must not offer or synthesize it.
      expect(refs).toEqual(['redirectUris', 'authFlowId', 'allowedUserTypes']);
      expect(componentsOf(steps[0]).some((component) => component.type === 'BOOLEAN_INPUT')).toBe(false);
    });

    it('should require every detail', () => {
      const inputs = componentsOf(steps[0]).filter((component) => component.ref);

      inputs.forEach((input) => expect(input.required).toBe(true));
    });

    it('should pick the login flow and the user type through the typed pickers', () => {
      const types = fieldByRef(steps[0], 'type');

      expect(types.redirectUris).toBe('TEXT_INPUT');
      expect(types.authFlowId).toBe('AUTH_FLOW_SELECT');
      expect(types.allowedUserTypes).toBe('USER_TYPE_SELECT');
    });

    it('should tell the person why each detail is asked for, and that URIs are separated by commas', () => {
      const hints = fieldByRef(steps[0], 'hint');

      Object.values(hints).forEach((hint) => expect(hint).toBeTruthy());
      expect(hints.redirectUris).toContain('commas');
    });

    it('should submit through a Continue action', () => {
      const submit = componentsOf(steps[0]).find((component) => component.eventType === 'SUBMIT');

      expect(submit).toBeDefined();
    });
  });

  // The two widgets describe the same three details, so only the requirement and the checkbox may
  // differ between them.
  it('should describe the on-behalf-of details the same way in both widgets', () => {
    const detailsOf = (type: string) =>
      componentsOf(widgetOfType(type).config?.data?.steps?.[0] ?? {})
        .filter((component) => ['redirectUris', 'authFlowId', 'allowedUserTypes'].includes(component.ref ?? ''))
        .map(({ref, type: inputType, hint}) => ({
          ref,
          inputType,
          hint: (hint ?? '').replace(/^Used when the agent acts on behalf of a signed-in user\. /, ''),
        }));

    expect(detailsOf('AGENT_DELEGATION')).toEqual(detailsOf('AGENT_OBO_DETAILS'));
  });

  // Every id a widget generates must be declared, or the drop leaves an unresolved placeholder in
  // the canvas.
  it('should declare a replacer for every generated step id', () => {
    ['AGENT_OWNER_RESOLUTION', 'AGENT_DELEGATION', 'AGENT_OBO_DETAILS'].forEach((type) => {
      const widget = widgetOfType(type);
      const declared = new Set((widget.config?.data?.__generationMeta__?.replacers ?? []).map((r) => r.key));
      const referenced = JSON.stringify(widget).match(/\{\{([A-Z0-9_]+)\}\}/g) ?? [];

      referenced
        .map((token) => token.slice(2, -2))
        .filter((key) => key !== 'ID')
        .forEach((key) => expect(declared, `${type} declares ${key}`).toContain(key));
    });
  });
});
