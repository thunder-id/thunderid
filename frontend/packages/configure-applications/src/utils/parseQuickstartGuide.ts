// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The kind of a GitHub-style alert (`> [!NOTE]`) in a guide.
 *
 * @public
 */
export type QuickstartGuideAlertKind = 'note' | 'tip' | 'important' | 'warning' | 'caution';

/**
 * One block of a quickstart guide step. Text fields hold inline Markdown (code spans, bold, links).
 *
 * @public
 */
export type QuickstartGuideBlock =
  | {type: 'paragraph'; text: string}
  | {type: 'heading'; text: string}
  | {type: 'code'; language?: string; title?: string; code: string}
  | {type: 'codeGroup'; options: {label: string; language?: string; code: string}[]}
  | {type: 'list'; ordered: boolean; items: QuickstartGuideBlock[][]}
  | {type: 'alert'; kind: QuickstartGuideAlertKind; title?: string; blocks: QuickstartGuideBlock[]}
  | {type: 'table'; header: string[]; rows: string[][]};

/**
 * One step of a quickstart guide: a second-level heading and the content under it.
 *
 * @public
 */
export interface QuickstartGuideStep {
  title: string;
  blocks: QuickstartGuideBlock[];
}

/**
 * The ready-to-run sample app a quickstart offers, from its "Run the Sample App" section.
 *
 * @public
 */
export interface QuickstartGuideSample {
  /** The sample's path in the samples repository (e.g. "quickstarts/react"), which identifies its releases */
  id: string;
  /** The section's content: how to configure and run the sample */
  blocks: QuickstartGuideBlock[];
}

/**
 * What the Console shows of a quickstart: the steps that build the app, and the sample app that
 * runs instead, when the page offers one.
 *
 * @public
 */
export interface QuickstartGuideContent {
  steps: QuickstartGuideStep[];
  sample: QuickstartGuideSample | null;
}

/**
 * A piece of a code sample. `filled` marks text that replaced a docs placeholder.
 *
 * @public
 */
export interface QuickstartGuideCodeSegment {
  text: string;
  filled: boolean;
}

// The docs quickstarts all open with the same setup sections, ending with the one that creates the
// application, or registers the agent, in the Console. Someone reading the guide on that resource's
// own page has done all of that, so the steps start after it. A page without this section is not a
// quickstart this parser understands.
const SETUP_END_HEADING = /^(create an application|register an agent)$/i;
const CLOSING_HEADING = /^what['’]s next$/i;
// The section that offers the guide's app ready to run, linked to its source in the samples
// repository. It is an alternative to the steps, not one of them.
const SAMPLE_HEADING = /^run the sample app$/i;
const SAMPLE_SOURCE = /github\.com\/[\w.-]+\/samples\/tree\/[^/\s)]+\/([^\s)#?]+)/;

// The docs export marks content a page shows for one choice only, such as an agent's mode or a code
// sample's language, with the choice it belongs to.
const VARIANT_OPEN = /^<!--\s*variant\s+([\w-]+)=([\w-]+)\s*-->$/;
const VARIANT_CLOSE = /^<!--\s*\/variant\s*-->$/;

const FENCE = /^(`{3,}|~{3,})\s*(.*)$/;
const LIST_ITEM = /^([-*]|\d+\.)\s+(.*)$/;
const ALERT_MARKER = /^\[!(NOTE|TIP|IMPORTANT|WARNING|CAUTION)\]\s*$/i;
const BOLD_ONLY = /^\*\*([^*]+)\*\*$/;
const TABLE_DIVIDER = /^\|?\s*:?-{2,}/;

function closesFence(line: string, fence: string): boolean {
  const trimmed = line.trim();
  return trimmed.startsWith(fence) && trimmed.replace(new RegExp(`^\\${fence[0]}+`), '') === '';
}

/**
 * Keeps the content written for the choices given and drops what is written for another choice.
 * Content marked for a kind of choice that is not given is kept whole.
 *
 * @returns The remaining lines, or null when choices are given and the page marks none, as a page
 * exported before the docs marked them does: its content for every choice runs together.
 */
function selectVariants(lines: string[], variants?: Record<string, string>): string[] | null {
  const kept: string[] = [];
  // Whether each open marker, innermost last, is for a choice that was made.
  const open: boolean[] = [];
  let fence: string | null = null;
  let isMarked = false;

  lines.forEach((line) => {
    const fenceMatch = FENCE.exec(line);
    if (fence) {
      if (closesFence(line, fence)) {
        fence = null;
      }
    } else if (fenceMatch) {
      [, fence] = fenceMatch;
    } else {
      const openMatch = VARIANT_OPEN.exec(line.trim());
      if (openMatch) {
        const chosen = variants?.[openMatch[1]];
        open.push(chosen === undefined || chosen === openMatch[2]);
        isMarked = true;
        return;
      }
      if (VARIANT_CLOSE.test(line.trim())) {
        open.pop();
        return;
      }
    }

    if (open.every(Boolean)) {
      kept.push(line);
    }
  });

  return variants && !isMarked ? null : kept;
}

function dedent(lines: string[]): string[] {
  const indents = lines.filter((line) => line.trim() !== '').map((line) => /^\s*/.exec(line)![0].length);
  const indent = indents.length > 0 ? Math.min(...indents) : 0;
  return lines.map((line) => line.slice(indent));
}

function splitTableRow(line: string): string[] {
  return line
    .trim()
    .replace(/^\||\|$/g, '')
    .split('|')
    .map((cell) => cell.trim());
}

function startsBlock(line: string): boolean {
  return (
    FENCE.test(line) || /^#{1,6}\s/.test(line) || line.startsWith('>') || line.startsWith('|') || LIST_ITEM.test(line)
  );
}

/**
 * Merges runs of a bold label followed by an untitled code sample (how the docs export writes a
 * tabbed code group, such as one sample per package manager) into a single code group.
 */
function groupLabeledCode(blocks: QuickstartGuideBlock[]): QuickstartGuideBlock[] {
  const result: QuickstartGuideBlock[] = [];
  let index = 0;

  while (index < blocks.length) {
    const options: {label: string; language?: string; code: string}[] = [];
    let cursor = index;

    while (cursor + 1 < blocks.length) {
      const label = blocks[cursor];
      const sample = blocks[cursor + 1];
      const labelMatch = label.type === 'paragraph' ? BOLD_ONLY.exec(label.text) : null;
      if (!labelMatch || sample.type !== 'code' || sample.title) {
        break;
      }
      options.push({label: labelMatch[1], language: sample.language, code: sample.code});
      cursor += 2;
    }

    if (options.length > 1) {
      result.push({type: 'codeGroup', options});
      index = cursor;
    } else {
      result.push(blocks[index]);
      index += 1;
    }
  }

  return result;
}

function parseBlocks(lines: string[]): QuickstartGuideBlock[] {
  const blocks: QuickstartGuideBlock[] = [];
  let index = 0;

  while (index < lines.length) {
    const line = lines[index];
    const fenceMatch = FENCE.exec(line);
    const listMatch = LIST_ITEM.exec(line);

    if (line.trim() === '') {
      index += 1;
    } else if (fenceMatch) {
      const [, fence, info] = fenceMatch;
      const code: string[] = [];
      index += 1;
      while (index < lines.length && !closesFence(lines[index], fence)) {
        code.push(lines[index]);
        index += 1;
      }
      index += 1;
      const language = /^[\w+-]+/.exec(info)?.[0];
      const title = /title="([^"]*)"/.exec(info)?.[1];
      blocks.push({type: 'code', language, title, code: code.join('\n')});
    } else if (/^#{1,6}\s/.test(line)) {
      blocks.push({type: 'heading', text: line.replace(/^#{1,6}\s+/, '').trim()});
      index += 1;
    } else if (line.startsWith('>')) {
      const quoted: string[] = [];
      while (index < lines.length && lines[index].startsWith('>')) {
        quoted.push(lines[index].replace(/^> ?/, ''));
        index += 1;
      }
      const kindMatch = ALERT_MARKER.exec(quoted[0]);
      const body = kindMatch ? quoted.slice(1) : quoted;
      const firstLine = body.findIndex((entry) => entry.trim() !== '');
      const titleMatch = firstLine === -1 ? null : BOLD_ONLY.exec(body[firstLine].trim());
      blocks.push({
        type: 'alert',
        kind: (kindMatch?.[1].toLowerCase() ?? 'note') as QuickstartGuideAlertKind,
        title: titleMatch?.[1],
        blocks: parseBlocks(titleMatch ? body.slice(firstLine + 1) : body),
      });
    } else if (line.startsWith('|') && TABLE_DIVIDER.test(lines[index + 1] ?? '')) {
      const header = splitTableRow(line);
      const rows: string[][] = [];
      index += 2;
      while (index < lines.length && lines[index].startsWith('|')) {
        rows.push(splitTableRow(lines[index]));
        index += 1;
      }
      blocks.push({type: 'table', header, rows});
    } else if (listMatch) {
      const ordered = /\d/.test(listMatch[1]);
      const items: QuickstartGuideBlock[][] = [];

      while (index < lines.length) {
        const itemMatch = LIST_ITEM.exec(lines[index]);
        if (!itemMatch || /\d/.test(itemMatch[1]) !== ordered) {
          break;
        }
        const continuation: string[] = [];
        index += 1;
        while (index < lines.length && (lines[index].trim() === '' || /^\s/.test(lines[index]))) {
          continuation.push(lines[index]);
          index += 1;
        }
        items.push(parseBlocks([itemMatch[2], ...dedent(continuation)]));
      }
      blocks.push({type: 'list', ordered, items});
    } else {
      const text: string[] = [line.trim()];
      index += 1;
      while (index < lines.length && lines[index].trim() !== '' && !startsBlock(lines[index])) {
        text.push(lines[index].trim());
        index += 1;
      }
      blocks.push({type: 'paragraph', text: text.join(' ')});
    }
  }

  return groupLabeledCode(blocks);
}

/** The anchor the docs site gives a heading (e.g. "Connect an MCP Client" becomes "connect-an-mcp-client"). */
function headingAnchor(title: string): string {
  return title
    .toLowerCase()
    .replace(/[^a-z0-9\s-]/g, '')
    .trim()
    .replace(/\s+/g, '-');
}

/**
 * The steps of one section of a page: each third-level heading in it and the content under it. A
 * bold-only line between them is a tab label of the docs page, not content, so it is left out.
 */
function parseSectionSteps(lines: string[]): QuickstartGuideStep[] {
  const steps: {title: string; lines: string[]}[] = [];
  let fence: string | null = null;

  lines.forEach((line) => {
    const fenceMatch = FENCE.exec(line);
    if (fence) {
      if (closesFence(line, fence)) {
        fence = null;
      }
    } else if (fenceMatch) {
      [, fence] = fenceMatch;
    } else if (/^###\s/.test(line)) {
      steps.push({title: line.replace(/^###\s+/, '').trim(), lines: []});
      return;
    } else if (BOLD_ONLY.test(line.trim())) {
      return;
    }

    if (steps.length > 0) {
      steps[steps.length - 1].lines.push(line);
    }
  });

  return steps
    .map((step) => ({title: step.title, blocks: parseBlocks(step.lines)}))
    .filter((step) => step.blocks.length > 0);
}

/**
 * Turns the Markdown export of a docs quickstart page into the steps a developer still has to do
 * once the application exists: everything after the "Create an Application" section, up to the
 * closing "What's Next" links. A "Run the Sample App" section is kept apart as the sample app.
 *
 * A page that teaches more than connecting an application (such as building an MCP server, whose
 * clients connect in one section of it) names the section that applies instead. Its third-level
 * headings are the steps.
 *
 * A page written for several choices at once, such as an agent acting on its own or on behalf of
 * a user, marks the content of each. Naming the choices keeps their content and drops the rest.
 *
 * @param markdown - The Markdown export of the quickstart page
 * @param section - The anchor of the section that holds the steps, when it is not the whole page
 * @param variants - The choices to show the page for, by kind (e.g. `{mode: 'own', lang: 'python'}`)
 * @returns The steps and the sample, or null when the page is not a quickstart in the expected shape
 *
 * @public
 */
export default function parseQuickstartGuide(
  markdown: string,
  section?: string,
  variants?: Record<string, string>,
): QuickstartGuideContent | null {
  const lines = selectVariants(markdown.split(/\r?\n/), variants);
  if (!lines) {
    return null;
  }

  const sections: {title: string; lines: string[]}[] = [];
  let fence: string | null = null;
  // The docs export can leave a page component it does not know as raw JSX. Everything from such
  // a tag to the end of its section is dropped rather than shown as text.
  let skipping = false;

  lines.forEach((line) => {
    const fenceMatch = FENCE.exec(line);
    if (fence) {
      if (closesFence(line, fence)) {
        fence = null;
      }
    } else if (fenceMatch) {
      [, fence] = fenceMatch;
    } else if (/^##\s/.test(line)) {
      sections.push({title: line.replace(/^##\s+/, '').trim(), lines: []});
      skipping = false;
      return;
    } else if (/^<[A-Z]/.test(line)) {
      skipping = true;
    }

    if (!skipping && sections.length > 0) {
      sections[sections.length - 1].lines.push(line);
    }
  });

  if (section) {
    const named = sections.find((entry) => headingAnchor(entry.title) === section);
    const namedSteps = named ? parseSectionSteps(named.lines) : [];
    return namedSteps.length > 0 ? {steps: namedSteps, sample: null} : null;
  }

  const setupEnd = sections.findIndex((entry) => SETUP_END_HEADING.test(entry.title));
  if (setupEnd === -1) {
    return null;
  }

  const closing = sections.findIndex((entry, index) => index > setupEnd && CLOSING_HEADING.test(entry.title));
  const steps = sections
    .slice(setupEnd + 1, closing === -1 ? undefined : closing)
    .filter((entry) => !SAMPLE_HEADING.test(entry.title))
    .map((entry) => ({title: entry.title, blocks: parseBlocks(entry.lines)}))
    .filter((step) => step.blocks.length > 0);

  if (steps.length === 0) {
    return null;
  }

  const sampleSection = sections.find((entry) => SAMPLE_HEADING.test(entry.title));
  const sampleId = sampleSection ? SAMPLE_SOURCE.exec(sampleSection.lines.join('\n'))?.[1] : undefined;

  return {
    steps,
    sample:
      sampleSection && sampleId ? {id: sampleId.replace(/\/+$/, ''), blocks: parseBlocks(sampleSection.lines)} : null,
  };
}

/**
 * Replaces the placeholders a docs code sample uses with real values, keeping track of which parts
 * were replaced so they can be highlighted. A placeholder without a value is left as written.
 *
 * @param code - The code sample
 * @param values - Placeholder text mapped to the value that replaces it
 * @returns The code, split into replaced and untouched segments
 *
 * @public
 */
export function fillQuickstartGuideCode(
  code: string,
  values: Record<string, string | undefined>,
): QuickstartGuideCodeSegment[] {
  const placeholders = Object.keys(values).filter((placeholder) => values[placeholder]);
  if (placeholders.length === 0) {
    return [{text: code, filled: false}];
  }

  const pattern = new RegExp(
    `(${placeholders.map((placeholder) => placeholder.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`,
    'g',
  );

  return code
    .split(pattern)
    .filter((part) => part !== '')
    .map((part) => (placeholders.includes(part) ? {text: values[part]!, filled: true} : {text: part, filled: false}));
}
