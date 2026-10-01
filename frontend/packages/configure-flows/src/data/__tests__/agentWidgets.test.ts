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

    it('should collect both delegation values on a single prompt', () => {
      expect(steps).toHaveLength(1);
      expect(steps[0].type).toBe('VIEW');

      const refs = componentsOf(steps[0])
        .filter((component) => component.ref)
        .map((component) => component.ref);

      expect(refs).toEqual(['delegated', 'redirectUris']);
    });

    // The executor compares the submitted string against a parsed boolean, and only BOOLEAN_INPUT
    // renders as a checkbox at runtime, so the type is what makes the value usable.
    it('should collect delegation as a boolean input', () => {
      const delegated = componentsOf(steps[0]).find((component) => component.ref === 'delegated');

      expect(delegated?.type).toBe('BOOLEAN_INPUT');
    });

    // Neither value is required: an agent acting on its own behalf needs neither, and the provider
    // supplies a default redirect URI when delegation is on without one.
    it('should leave both inputs optional', () => {
      const inputs = componentsOf(steps[0]).filter((component) => component.ref);

      inputs.forEach((input) => expect(input.required).toBe(false));
    });
  });

  // Every id a widget generates must be declared, or the drop leaves an unresolved placeholder in
  // the canvas.
  it('should declare a replacer for every generated step id', () => {
    ['AGENT_OWNER_RESOLUTION', 'AGENT_DELEGATION'].forEach((type) => {
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
