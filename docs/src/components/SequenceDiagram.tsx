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

/* ── Types ──────────────────────────────────────────────────────────────── */

export interface MessageRow {
  from: number;
  to: number;
  label: string | string[];
  sublabel?: string | string[];
}

export interface NoteRow {
  note: string;
  between?: [number, number];
}

export type Row = MessageRow | NoteRow;

function isNote(row: Row): row is NoteRow {
  return 'note' in row;
}

/* ── Layout constants ───────────────────────────────────────────────────── */

const ACTOR_Y = 14;
const ACTOR_H = 36;
const ACTOR_W = 175;
const LABEL_LINE_H = 22;
const NOTE_ROW_H = 48;
const LEFT_PAD = 40;
const RIGHT_PAD = 40;
const TOP_GAP = 28; // min gap between the actor boxes and the first label block
const INTER_ROW_GAP = 18; // gap between one arrow line and the next row's label block
// Gap between a label block's bottom and its arrow line. Small, so a label hugs its own line.
const LABEL_LINE_GAP = 8;

/* ── Generic N-actor sequence diagram ───────────────────────────────────── */

export interface SequenceDiagramProps {
  actors: string[];
  gaps?: number[];
  rows: Row[];
  ariaLabel: string;
}

export function SequenceDiagram({ actors, gaps = [], rows, ariaLabel }: SequenceDiagramProps) {
  const actorCount = actors.length;
  const defaultGap = actorCount <= 2 ? 400 : actorCount === 3 ? 260 : 210;
  const firstActorX = LEFT_PAD + ACTOR_W / 2;
  const actorXPositions: number[] = [firstActorX];
  for (let i = 1; i < actorCount; i++) {
    actorXPositions.push(actorXPositions[i - 1] + (gaps?.[i - 1] ?? defaultGap));
  }
  const svgW = actorXPositions[actorCount - 1] + ACTOR_W / 2 + RIGHT_PAD;

  // Helpers for multi-line labels.
  function labelLines(label: string | string[]): string[] {
    return Array.isArray(label) ? label : [label];
  }
  function blockHeight(row: MessageRow): number {
    const labelH = labelLines(row.label).length * LABEL_LINE_H;
    const subH = row.sublabel ? labelLines(row.sublabel).length * LABEL_LINE_H : 0;
    return labelH + subH;
  }

  // Uniform row pitch: every arrow line is spaced the same distance, sized to fit the
  // tallest label block so no row overlaps another. Gives consistent vertical rhythm.
  const maxBlockH = rows.reduce(
    (m, r) => (isNote(r) ? m : Math.max(m, blockHeight(r))),
    LABEL_LINE_H,
  );
  const rowPitch = maxBlockH + LABEL_LINE_GAP + INTER_ROW_GAP;

  // First arrow sits low enough that the tallest label block clears the actor boxes.
  const firstMsgY = ACTOR_Y + ACTOR_H + maxBlockH + LABEL_LINE_GAP + TOP_GAP;

  // Compute Y positions.
  let y = firstMsgY;
  const positions: number[] = [];
  for (const row of rows) {
    positions.push(y);
    y += isNote(row) ? NOTE_ROW_H : rowPitch;
  }
  const totalH = y + 16;

  return (
    <figure className="seq-diagram" role="img" aria-label={ariaLabel}>
      <svg
        viewBox={`0 0 ${svgW} ${totalH}`}
        style={{ width: '100%', overflow: 'visible', display: 'block', fontFamily: 'inherit' }}
        aria-hidden="true"
      >
        <defs>
          <marker id="seq-arrow" markerWidth="8" markerHeight="6" refX="7" refY="3" orient="auto-start-reverse">
            <polygon points="0 0, 8 3, 0 6" className="seq-arrowhead" />
          </marker>
        </defs>

        {/* Actors */}
        {actors.map((name, i) => {
          const cx = actorXPositions[i];
          return (
            <g key={name}>
              <rect x={cx - ACTOR_W / 2} y={ACTOR_Y} width={ACTOR_W} height={ACTOR_H} rx="6" className="seq-actor" />
              <text x={cx} y={ACTOR_Y + ACTOR_H / 2} textAnchor="middle" dominantBaseline="central" className="seq-actor-label">
                {name}
              </text>
              <line x1={cx} y1={ACTOR_Y + ACTOR_H} x2={cx} y2={totalH} className="seq-lifeline" />
            </g>
          );
        })}

        {/* Rows */}
        {rows.map((row, i) => {
          const rowY = positions[i];

          if (isNote(row)) {
            const [a, b] = row.between ?? [0, actorCount - 1];
            const noteLeft = actorXPositions[a];
            const noteRight = actorXPositions[b];
            const noteMid = (noteLeft + noteRight) / 2;
            // Text-aware width: ~6.5px per char + 24px padding, with a sensible min.
            const noteW = Math.max(180, row.note.length * 6.5 + 24);

            return (
              <g key={row.note}>
                <rect x={noteMid - noteW / 2} y={rowY - 14} width={noteW} height="24" rx="4" className="seq-note-bg" />
                <text x={noteMid} y={rowY} textAnchor="middle" dominantBaseline="central" className="seq-note">
                  {row.note}
                </text>
              </g>
            );
          }

          const fromX = actorXPositions[row.from];
          const toX = actorXPositions[row.to];
          const goingRight = toX > fromX;
          const x1 = goingRight ? fromX + 1 : fromX - 1;
          const x2 = goingRight ? toX - 1 : toX + 1;

          const lines = labelLines(row.label);
          const sublabelLines = row.sublabel ? labelLines(row.sublabel) : [];
          const lineCount = lines.length;
          const totalLabelH = lineCount * LABEL_LINE_H;
          const sublabelH = sublabelLines.length * LABEL_LINE_H;
          const blockH = totalLabelH + sublabelH;
          const blockTopY = rowY - blockH - LABEL_LINE_GAP;

          const midX = (fromX + toX) / 2;
          return (
            <g key={`${row.from}-${row.to}-${Array.isArray(row.label) ? row.label[0] : row.label}`}>
              <line x1={x1} y1={rowY} x2={x2} y2={rowY} className="seq-message" markerEnd="url(#seq-arrow)" />
              {lines.length > 0 && lines[0] && (
                <text x={midX} y={blockTopY + LABEL_LINE_H * 0.75} textAnchor="middle" className="seq-message-label">
                  {lines.map((line, li) => (
                    <tspan key={line} x={midX} dy={li === 0 ? 0 : LABEL_LINE_H}>
                      {line}
                    </tspan>
                  ))}
                </text>
              )}
              {sublabelLines.length > 0 && (
                <text x={midX} y={blockTopY + totalLabelH + LABEL_LINE_H * 0.75} textAnchor="middle" className="seq-message-sublabel">
                  {sublabelLines.map((line, li) => (
                    <tspan key={line} x={midX} dy={li === 0 ? 0 : LABEL_LINE_H}>
                      {line}
                    </tspan>
                  ))}
                </text>
              )}
            </g>
          );
        })}
      </svg>
    </figure>
  );
}
