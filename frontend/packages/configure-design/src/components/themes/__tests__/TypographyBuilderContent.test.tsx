// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, cleanup} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {Theme} from '@thunderid/design';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import {useState} from 'react';
import {describe, it, expect, afterEach, vi} from 'vitest';
import TypographyBuilderContent from '../TypographyBuilderContent';

const VARIANT_KEYS = [
  'h1',
  'h2',
  'h3',
  'h4',
  'h5',
  'h6',
  'subtitle1',
  'subtitle2',
  'body1',
  'body2',
  'button',
  'caption',
  'overline',
];

/** A fully populated typography record so propagateWeight/clearVariantSizes have variant
 *  objects to actually mutate (not just skip via their `continue`/falsy guards). */
function makeFullTypography(): Record<string, unknown> {
  const typography: Record<string, unknown> = {
    fontFamily: 'Arial',
    fontWeightLight: 300,
    fontWeightRegular: 400,
    fontWeightMedium: 500,
    fontWeightBold: 700,
    fontSize: 14,
    htmlFontSize: 16,
  };
  for (const key of VARIANT_KEYS) {
    typography[key] = {fontWeight: 400, fontSize: '1rem', lineHeight: 1.2, letterSpacing: '0.02em'};
  }
  return typography;
}

// useFontStylesheetLink needs a ConfigProvider, which this test tree doesn't otherwise set up.
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({config: {brand: {product_name: 'ThunderID'}}}),
  };
});

afterEach(() => {
  cleanup();
});

interface MutableTypography {
  fontFamily?: string;
  font?: {importURL?: string};
}

function makeDraft(typography: MutableTypography = {fontFamily: ''}): Theme {
  return {typography} as unknown as Theme;
}

function typographyOf(draft: Theme): MutableTypography {
  return draft.typography as unknown as MutableTypography;
}

function renderEditor(draft: Theme) {
  const onUpdate = vi.fn((updater: (d: Theme) => void) => updater(draft));
  render(
    <OxygenUIThemeProvider>
      <TypographyBuilderContent draft={draft} onUpdate={onUpdate} />
    </OxygenUIThemeProvider>,
  );
  return onUpdate;
}

/** Mirrors the real ThemeConfigPanel.updateDraft: clones the draft and re-renders with the new
 *  object, needed for tests that exercise multiple sequential edits (renderEditor's in-place
 *  mutation doesn't give a fresh object reference each time). */
function StatefulEditor({initialDraft, onDraftChange}: {initialDraft: Theme; onDraftChange: (d: Theme) => void}) {
  const [draft, setDraft] = useState(initialDraft);
  const onUpdate = (updater: (d: Theme) => void): void => {
    const next = JSON.parse(JSON.stringify(draft)) as Theme;
    updater(next);
    setDraft(next);
    onDraftChange(next);
  };
  return <TypographyBuilderContent draft={draft} onUpdate={onUpdate} />;
}

function renderStatefulEditor(initialDraft: Theme) {
  let currentDraft = initialDraft;
  const utils = render(
    <OxygenUIThemeProvider>
      <StatefulEditor
        initialDraft={initialDraft}
        onDraftChange={(d) => {
          currentDraft = d;
        }}
      />
    </OxygenUIThemeProvider>,
  );
  return {...utils, getDraft: () => currentDraft};
}

describe('TypographyBuilderContent font modes', () => {
  it('defaults to the web-safe font picker when no import URL is set', () => {
    renderEditor(makeDraft());
    expect(screen.getByRole('combobox')).toBeTruthy();
    expect(screen.queryByLabelText('Font import URL')).toBeNull();
  });

  it('shows the bare name selected when the theme stores the default font stack', () => {
    renderEditor(makeDraft({fontFamily: "'Inter Variable', sans-serif"}));
    expect(screen.getByRole<HTMLInputElement>('combobox').value).toBe('Inter Variable');
  });

  it('starts in import mode showing the configured import URL', () => {
    const draft = makeDraft({
      fontFamily: 'Poppins',
      font: {importURL: 'https://fonts.googleapis.com/css2?family=Poppins'},
    });
    renderEditor(draft);
    const urlField = screen.getByLabelText<HTMLInputElement>('Font import URL');
    expect(urlField.value).toBe('https://fonts.googleapis.com/css2?family=Poppins');
  });

  it('reveals the import fields when switching to import mode', () => {
    renderEditor(makeDraft());
    fireEvent.click(screen.getByRole('button', {name: 'Use a custom font'}));
    expect(screen.getByLabelText('Font import URL')).toBeTruthy();
    expect(screen.getAllByLabelText('Font family').length).toBeGreaterThan(0);
  });

  it('clears the inherited family when switching to import so the field shows its placeholder', () => {
    const draft = makeDraft({fontFamily: "'Inter Variable', sans-serif"});
    renderEditor(draft);
    fireEvent.click(screen.getByRole('button', {name: 'Use a custom font'}));
    expect(typographyOf(draft).fontFamily).toBe('');
    expect(screen.getByLabelText<HTMLInputElement>('Font family').value).toBe('');
  });

  it('typing an import URL writes typography.font.importURL', () => {
    const draft = makeDraft();
    renderEditor(draft);
    fireEvent.click(screen.getByRole('button', {name: 'Use a custom font'}));
    fireEvent.change(screen.getByLabelText('Font import URL'), {
      target: {value: 'https://fonts.bunny.net/css?family=inter'},
    });
    expect(typographyOf(draft).font?.importURL).toBe('https://fonts.bunny.net/css?family=inter');
  });

  it('clears the applied import URL when switching to a web-safe font', () => {
    const draft = makeDraft({
      fontFamily: 'Poppins',
      font: {importURL: 'https://fonts.googleapis.com/css2?family=Poppins'},
    });
    renderEditor(draft);
    fireEvent.click(screen.getByRole('button', {name: 'Use a web-safe font'}));
    expect(typographyOf(draft).font).toBeUndefined();
  });

  it('remembers the import URL and family when toggling to web-safe and back', () => {
    const {getDraft} = renderStatefulEditor(
      makeDraft({fontFamily: 'Poppins', font: {importURL: 'https://fonts.googleapis.com/css2?family=Poppins'}}),
    );
    // Toggling to web-safe drops the import from the applied theme, but remembers it.
    fireEvent.click(screen.getByRole('button', {name: 'Use a web-safe font'}));
    expect(typographyOf(getDraft()).font).toBeUndefined();
    // Toggling back to import restores the remembered URL and family.
    fireEvent.click(screen.getByRole('button', {name: 'Use a custom font'}));
    expect(typographyOf(getDraft()).font?.importURL).toBe('https://fonts.googleapis.com/css2?family=Poppins');
    expect(typographyOf(getDraft()).fontFamily).toBe('Poppins');
  });

  it('promotes the toggle to import mode when a draft change restores an import URL (e.g. Revert)', () => {
    const webSafeDraft = makeDraft({fontFamily: 'Arial'});
    const {rerender} = render(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={webSafeDraft} onUpdate={vi.fn()} />
      </OxygenUIThemeProvider>,
    );
    expect(screen.queryByLabelText('Font import URL')).toBeNull();

    // A Revert replaces the draft in place with one that carries an import URL again.
    const revertedDraft = makeDraft({
      fontFamily: 'Poppins',
      font: {importURL: 'https://fonts.googleapis.com/css2?family=Poppins'},
    });
    rerender(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={revertedDraft} onUpdate={vi.fn()} />
      </OxygenUIThemeProvider>,
    );
    const urlField = screen.getByLabelText<HTMLInputElement>('Font import URL');
    expect(urlField.value).toBe('https://fonts.googleapis.com/css2?family=Poppins');
  });

  it('demotes the toggle back to web-safe when a draft change restores a plain family with no import URL (e.g. Revert)', () => {
    const importDraft = makeDraft({
      fontFamily: 'Poppins',
      font: {importURL: 'https://fonts.googleapis.com/css2?family=Poppins'},
    });
    const {rerender} = render(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={importDraft} onUpdate={vi.fn()} />
      </OxygenUIThemeProvider>,
    );
    expect(screen.getByLabelText('Font import URL')).toBeTruthy();

    // A Revert replaces the draft in place with the original web-safe theme (no import URL).
    const revertedDraft = makeDraft({fontFamily: 'Arial'});
    rerender(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={revertedDraft} onUpdate={vi.fn()} />
      </OxygenUIThemeProvider>,
    );
    expect(screen.queryByLabelText('Font import URL')).toBeNull();
    expect(screen.getByRole('combobox')).toBeTruthy();
  });

  it('demotes the toggle back to web-safe when a draft change restores an empty family with no import URL (e.g. Revert)', () => {
    const importDraft = makeDraft({
      fontFamily: 'Poppins',
      font: {importURL: 'https://fonts.googleapis.com/css2?family=Poppins'},
    });
    const {rerender} = render(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={importDraft} onUpdate={vi.fn()} />
      </OxygenUIThemeProvider>,
    );
    expect(screen.getByLabelText('Font import URL')).toBeTruthy();

    // A Revert can restore a draft with neither a family nor an import URL set.
    const revertedDraft = makeDraft();
    rerender(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={revertedDraft} onUpdate={vi.fn()} />
      </OxygenUIThemeProvider>,
    );
    expect(screen.queryByLabelText('Font import URL')).toBeNull();
    expect(screen.getByRole('combobox')).toBeTruthy();
  });

  it('falls the live preview back to the product default (not a generic serif) for a partial font name', () => {
    renderStatefulEditor(makeDraft());
    fireEvent.click(screen.getByRole('button', {name: 'Use a custom font'}));
    fireEvent.change(screen.getByLabelText('Font family'), {target: {value: 'P'}});

    // sx compiles to a generated CSS class via emotion, so assert against the injected stylesheet.
    screen.getByText('The quick brown fox jumps over the lazy dog.');
    const styles = Array.from(document.querySelectorAll('style'))
      .map((style) => style.textContent)
      .join('\n');
    expect(styles).toContain('font-family:P,');
    expect(styles).toMatch(/Inter Variable/);
  });

  it('loads the imported font stylesheet into the main document, not just the GatePreview iframe', () => {
    renderEditor(
      makeDraft({fontFamily: 'Poppins', font: {importURL: 'https://fonts.googleapis.com/css2?family=Poppins'}}),
    );
    const link = document.querySelector(
      'link[rel="stylesheet"][href="https://fonts.googleapis.com/css2?family=Poppins"]',
    );
    expect(link).not.toBeNull();
  });

  it('clearing the import URL also clears the family, reverting the whole selection to default', () => {
    const draft = makeDraft({
      fontFamily: 'Poppins',
      font: {importURL: 'https://fonts.googleapis.com/css2?family=Poppins'},
    });
    renderEditor(draft);
    fireEvent.change(screen.getByLabelText('Font import URL'), {target: {value: ''}});
    expect(typographyOf(draft).fontFamily).toBe('');
    expect(typographyOf(draft).font).toBeUndefined();
  });

  it('applies a font selected from the web-safe dropdown', async () => {
    const user = userEvent.setup();
    const draft = makeDraft({fontFamily: 'Arial'});
    renderEditor(draft);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'Georgia'}));

    expect(typographyOf(draft).fontFamily).toBe('Georgia');
  });

  it('does not touch the draft when clicking the already-active font mode toggle', async () => {
    const user = userEvent.setup();
    const draft = makeDraft({fontFamily: 'Arial'});
    const onUpdate = vi.fn();
    render(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={draft} onUpdate={onUpdate} />
      </OxygenUIThemeProvider>,
    );

    await user.click(screen.getByRole('button', {name: 'Use a web-safe font'}));

    expect(onUpdate).not.toHaveBeenCalled();
  });
});

describe('TypographyBuilderContent font weights', () => {
  function renderWithFullTypography() {
    const draft = {typography: makeFullTypography()} as unknown as Theme;
    const onUpdate = vi.fn((updater: (d: Theme) => void) => updater(draft));
    render(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={draft} onUpdate={onUpdate} />
      </OxygenUIThemeProvider>,
    );
    return {draft, onUpdate};
  }

  function typographyRecord(draft: Theme): Record<string, unknown> {
    return draft.typography as unknown as Record<string, unknown>;
  }

  it('resets all font weights and propagates them to every variant', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithFullTypography();

    await user.click(screen.getByText('Font weights'));
    // Font Weights is the first of the three ConfigCards that render a "Reset" action.
    await user.click(screen.getAllByText('Reset')[0]);

    const typo = typographyRecord(draft);
    expect(typo['fontWeightLight']).toBe(300);
    expect(typo['fontWeightRegular']).toBe(400);
    expect(typo['fontWeightMedium']).toBe(500);
    expect(typo['fontWeightBold']).toBe(700);
    // h1 references fontWeightLight per VARIANT_WEIGHT_REF
    expect((typo['h1'] as Record<string, unknown>)['fontWeight']).toBe(300);
    // h3 references fontWeightRegular
    expect((typo['h3'] as Record<string, unknown>)['fontWeight']).toBe(400);
  });

  it('updates the Light weight and propagates it to variants referencing fontWeightLight', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithFullTypography();

    await user.click(screen.getByText('Font weights'));
    // Index 0 is the Font Family autocomplete's own combobox; the weight Selects follow it.
    const comboboxes = screen.getAllByRole('combobox');
    await user.click(comboboxes[1]);
    await user.click(screen.getByRole('option', {name: '100 — Thin'}));

    const typo = typographyRecord(draft);
    expect(typo['fontWeightLight']).toBe(100);
    expect((typo['h1'] as Record<string, unknown>)['fontWeight']).toBe(100);
    expect((typo['h2'] as Record<string, unknown>)['fontWeight']).toBe(100);
  });

  it('updates the Regular weight and propagates it to variants referencing fontWeightRegular', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithFullTypography();

    await user.click(screen.getByText('Font weights'));
    const comboboxes = screen.getAllByRole('combobox');
    await user.click(comboboxes[2]);
    await user.click(screen.getByRole('option', {name: '200 — Extra Light'}));

    const typo = typographyRecord(draft);
    expect(typo['fontWeightRegular']).toBe(200);
    expect((typo['h3'] as Record<string, unknown>)['fontWeight']).toBe(200);
  });

  it('updates the Medium weight and propagates it to variants referencing fontWeightMedium', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithFullTypography();

    await user.click(screen.getByText('Font weights'));
    const comboboxes = screen.getAllByRole('combobox');
    await user.click(comboboxes[3]);
    await user.click(screen.getByRole('option', {name: '600 — Semi Bold'}));

    const typo = typographyRecord(draft);
    expect(typo['fontWeightMedium']).toBe(600);
    expect((typo['h6'] as Record<string, unknown>)['fontWeight']).toBe(600);
  });

  it('updates the Bold weight and propagates it to variants referencing fontWeightBold', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithFullTypography();

    await user.click(screen.getByText('Font weights'));
    const comboboxes = screen.getAllByRole('combobox');
    await user.click(comboboxes[4]);
    // There is no variant mapped to fontWeightBold by default, so only the base value changes.
    await user.click(screen.getByRole('option', {name: '900 — Black'}));

    const typo = typographyRecord(draft);
    expect(typo['fontWeightBold']).toBe(900);
  });
});

describe('TypographyBuilderContent base sizes', () => {
  function renderWithFullTypography() {
    const draft = {typography: makeFullTypography()} as unknown as Theme;
    const onUpdate = vi.fn((updater: (d: Theme) => void) => updater(draft));
    render(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={draft} onUpdate={onUpdate} />
      </OxygenUIThemeProvider>,
    );
    return {draft, onUpdate};
  }

  function typographyRecord(draft: Theme): Record<string, unknown> {
    return draft.typography as unknown as Record<string, unknown>;
  }

  it('resets base sizes and clears computed variant sizes', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithFullTypography();

    await user.click(screen.getByText('Base sizes'));
    // Base Sizes is the second of the three ConfigCards that render a "Reset" action.
    await user.click(screen.getAllByText('Reset')[1]);

    const typo = typographyRecord(draft);
    expect(typo['fontSize']).toBe(14);
    expect(typo['htmlFontSize']).toBe(16);
    expect((typo['h1'] as Record<string, unknown>)['fontSize']).toBeUndefined();
    expect((typo['h1'] as Record<string, unknown>)['lineHeight']).toBeUndefined();
    expect((typo['h1'] as Record<string, unknown>)['letterSpacing']).toBeUndefined();
  });

  it('changes the base font size slider and clears computed variant sizes', () => {
    const {draft} = renderWithFullTypography();
    fireEvent.click(screen.getByText('Base sizes'));

    const sliders = screen.getAllByRole('slider');
    fireEvent.change(sliders[0], {target: {value: 18}});

    const typo = typographyRecord(draft);
    expect(typo['fontSize']).toBe(18);
    expect((typo['h1'] as Record<string, unknown>)['fontSize']).toBeUndefined();
  });

  it('changes the html font size slider and clears computed variant sizes', () => {
    const {draft} = renderWithFullTypography();
    fireEvent.click(screen.getByText('Base sizes'));

    const sliders = screen.getAllByRole('slider');
    fireEvent.change(sliders[1], {target: {value: 20}});

    const typo = typographyRecord(draft);
    expect(typo['htmlFontSize']).toBe(20);
    expect((typo['h1'] as Record<string, unknown>)['fontSize']).toBeUndefined();
  });
});

describe('TypographyBuilderContent type scale', () => {
  function renderWithFullTypography() {
    const draft = {typography: makeFullTypography()} as unknown as Theme;
    const onUpdate = vi.fn((updater: (d: Theme) => void) => updater(draft));
    render(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={draft} onUpdate={onUpdate} />
      </OxygenUIThemeProvider>,
    );
    return {draft, onUpdate};
  }

  function typographyRecord(draft: Theme): Record<string, unknown> {
    return draft.typography as unknown as Record<string, unknown>;
  }

  it('resets the type scale, clearing computed size/lineHeight/letterSpacing for every variant', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithFullTypography();

    await user.click(screen.getByText('Type scale'));
    // Type Scale is the third of the three ConfigCards that render a "Reset" action.
    await user.click(screen.getAllByText('Reset')[2]);

    const typo = typographyRecord(draft);
    for (const key of VARIANT_KEYS) {
      expect((typo[key] as Record<string, unknown>)['fontSize']).toBeUndefined();
    }
  });

  it('edits a variant font size directly via its text field', () => {
    const {draft} = renderWithFullTypography();
    fireEvent.click(screen.getByText('Type scale'));

    // h1 is the first entry in TYPE_SCALE_VARIANTS, so its text field is the first textbox.
    const input = screen.getAllByRole('textbox')[0];
    fireEvent.change(input, {target: {value: '2.5rem'}});

    const typo = typographyRecord(draft);
    expect((typo['h1'] as Record<string, unknown>)['fontSize']).toBe('2.5rem');
  });
});

describe('TypographyBuilderContent defensive guards', () => {
  function renderWithoutTypography() {
    const draft = {} as Theme;
    const onUpdate = vi.fn((updater: (d: Theme) => void) => updater(draft));
    render(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={draft} onUpdate={onUpdate} />
      </OxygenUIThemeProvider>,
    );
    return {draft, onUpdate};
  }

  it("applyFont's guard no-ops the font-mode toggle when there is no typography object", async () => {
    const user = userEvent.setup();
    const {draft} = renderWithoutTypography();

    await user.click(screen.getByRole('button', {name: 'Use a custom font'}));

    expect(draft.typography).toBeUndefined();
  });

  it('the Font Weights reset no-ops when there is no typography object', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithoutTypography();

    await user.click(screen.getByText('Font weights'));
    await user.click(screen.getAllByText('Reset')[0]);

    expect(draft.typography).toBeUndefined();
  });

  it('the Light weight select no-ops when there is no typography object', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithoutTypography();

    await user.click(screen.getByText('Font weights'));
    await user.click(screen.getAllByRole('combobox')[1]);
    await user.click(screen.getByRole('option', {name: '100 — Thin'}));

    expect(draft.typography).toBeUndefined();
  });

  it('the Regular weight select no-ops when there is no typography object', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithoutTypography();

    await user.click(screen.getByText('Font weights'));
    await user.click(screen.getAllByRole('combobox')[2]);
    await user.click(screen.getByRole('option', {name: '200 — Extra Light'}));

    expect(draft.typography).toBeUndefined();
  });

  it('the Medium weight select no-ops when there is no typography object', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithoutTypography();

    await user.click(screen.getByText('Font weights'));
    await user.click(screen.getAllByRole('combobox')[3]);
    await user.click(screen.getByRole('option', {name: '600 — Semi Bold'}));

    expect(draft.typography).toBeUndefined();
  });

  it('the Bold weight select no-ops when there is no typography object', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithoutTypography();

    await user.click(screen.getByText('Font weights'));
    await user.click(screen.getAllByRole('combobox')[4]);
    await user.click(screen.getByRole('option', {name: '900 — Black'}));

    expect(draft.typography).toBeUndefined();
  });

  it('the Base Sizes reset no-ops when there is no typography object', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithoutTypography();

    await user.click(screen.getByText('Base sizes'));
    await user.click(screen.getAllByText('Reset')[1]);

    expect(draft.typography).toBeUndefined();
  });

  it('the base font size slider no-ops when there is no typography object', () => {
    const {draft} = renderWithoutTypography();

    fireEvent.click(screen.getByText('Base sizes'));
    fireEvent.change(screen.getAllByRole('slider')[0], {target: {value: 18}});

    expect(draft.typography).toBeUndefined();
  });

  it('the HTML font size slider no-ops when there is no typography object', () => {
    const {draft} = renderWithoutTypography();

    fireEvent.click(screen.getByText('Base sizes'));
    fireEvent.change(screen.getAllByRole('slider')[1], {target: {value: 20}});

    expect(draft.typography).toBeUndefined();
  });

  it('the Type Scale reset no-ops when there is no typography object', async () => {
    const user = userEvent.setup();
    const {draft} = renderWithoutTypography();

    await user.click(screen.getByText('Type scale'));
    await user.click(screen.getAllByText('Reset')[2]);

    expect(draft.typography).toBeUndefined();
  });

  it('clearVariantSizes skips variants that are not present on the typography object', async () => {
    const user = userEvent.setup();
    // Only fontSize/htmlFontSize are set; every per-variant object (h1, h2, ...) is absent.
    const draft = {typography: {fontSize: 14, htmlFontSize: 16}} as unknown as Theme;
    const onUpdate = vi.fn((updater: (d: Theme) => void) => updater(draft));
    render(
      <OxygenUIThemeProvider>
        <TypographyBuilderContent draft={draft} onUpdate={onUpdate} />
      </OxygenUIThemeProvider>,
    );

    await user.click(screen.getByText('Base sizes'));
    await user.click(screen.getAllByText('Reset')[1]);

    const typo = draft.typography as unknown as Record<string, unknown>;
    expect(typo['fontSize']).toBe(14);
    expect(typo['htmlFontSize']).toBe(16);
    expect(typo['h1']).toBeUndefined();
  });
});
