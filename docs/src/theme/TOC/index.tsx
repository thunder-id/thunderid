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

import BrowserOnly from '@docusaurus/BrowserOnly';
import {useDoc} from '@docusaurus/plugin-content-docs/client';
import OriginalTOC from '@theme-original/TOC';
import React, {Component, useEffect, useRef, useState} from 'react';
import {createPortal} from 'react-dom';

type OriginalTOCProps = React.ComponentProps<typeof OriginalTOC>;

// Error boundary so useDoc() crashing on non-doc pages (blog, policy pages) doesn't break the build
class DocContextGuard extends Component<{children: React.ReactNode}, {failed: boolean}> {
  state = {failed: false};
  static getDerivedStateFromError() { return {failed: true}; }
  render() { return this.state.failed ? null : this.props.children; }
}

function StepperPageDetector({onDetect}: {onDetect: (v: boolean) => void}) {
  const doc = useDoc();
  const isStepper = (doc?.frontMatter as Record<string, unknown>)?.toc_progress === 'quickstart';
  useEffect(() => { onDetect(isStepper); }, [isStepper, onDetect]);
  return null;
}

// ── Normal-page constants ─────────────────────────────────────────────────────
const X_OUTER = 1;
const X_INNER = 8;
const CURVE_R  = 6;

// ── Stepper constants ─────────────────────────────────────────────────────────
const S_CX     = 12;  // main line / circle center x
const S_R      = 12;  // numbered circle radius (24px diameter)
const S_DR     = 3.5; // nested dot radius
const S_INDENT = 13;  // how far nested items branch to the right
const S_W      = 30;  // SVG width
const S_LEFT   = 4;   // SVG left offset in px

interface LinkItem { y: number; isNested: boolean; }

/** Vertical connecting lines only — used in both gray base and blue clip overlay */
function stepperLines(
  itemYs: number[],
  itemNested: boolean[],
  color: string,
): React.ReactElement[] {
  return itemYs.slice(1).map((cy, idx) => {
    const i  = idx + 1;
    const py = itemYs[i - 1];
    const y1 = py + (itemNested[i - 1] ? 0 : S_R);
    const y2 = cy  - (itemNested[i]    ? 0 : S_R);
    return <line key={`l${cy}`} x1={S_CX} y1={y1} x2={S_CX} y2={y2} stroke={color} strokeWidth="1.5" />;
  });
}

/** Circles + dots — rendered outside the clip with per-element CSS transitions.
 *  Dots use a transition-delay so they light up just as the line arrives. */
function stepperNodes(
  itemYs: number[],
  itemNested: boolean[],
  stepNumbers: (number | null)[],
  activeIdx: number,
  fillPct: number,
  primary: string,
  muted: string,
): React.ReactElement[] {
  return itemYs.flatMap((cy, i) => {
    const num    = stepNumbers[i];
    const active = i <= activeIdx && fillPct > 0;
    const color  = active ? primary : muted;

    if (itemNested[i]) {
      // H3 branch: delay so it lights up when the line arrives
      return [(
        <g key={`n${cy}`}>
          <line x1={S_CX} y1={cy} x2={S_CX + S_INDENT} y2={cy}
            stroke={color} strokeWidth="1"
            style={{transition: 'stroke 150ms ease', transitionDelay: active ? '200ms' : '0ms'}} />
          <circle cx={S_CX + S_INDENT} cy={cy} r={S_DR} fill={color}
            style={{transition: 'fill 150ms ease', transitionDelay: active ? '200ms' : '0ms'}} />
        </g>
      )];
    }
    if (num === null) {
      return [(
        <circle key={`d${cy}`} cx={S_CX} cy={cy} r={S_DR + 0.5} fill={color}
          style={{transition: 'fill 150ms ease', transitionDelay: active ? '200ms' : '0ms'}} />
      )];
    }
    // Numbered circle
    return [(
      <g key={`c${cy}`}>
        <circle cx={S_CX} cy={cy} r={S_R}
          fill={active ? primary : 'transparent'} stroke={color} strokeWidth="1.5"
          style={{transition: 'fill 200ms ease, stroke 200ms ease'}} />
        <text x={S_CX} y={cy} textAnchor="middle" dominantBaseline="central"
          fontSize="11" fontWeight="700" fill={active ? '#fff' : color}
          style={{transition: 'fill 200ms ease', userSelect: 'none'}}>
          {num}
        </text>
      </g>
    )];
  });
}

function buildPath(items: LinkItem[]): string {
  if (!items.length) return '';
  const gx = (n: boolean) => (n ? X_INNER : X_OUTER);
  let x = gx(items[0].isNested);
  let d = `M ${x} ${items[0].y}`;
  for (let i = 1; i < items.length; i++) {
    const nx = gx(items[i].isNested);
    const py = items[i - 1].y;
    const cy = items[i].y;
    if (nx === x) {
      d += ` L ${x} ${cy}`;
    } else {
      const mid = (py + cy) / 2;
      const r   = Math.min(CURVE_R, (cy - py) * 0.4);
      d += ` L ${x} ${mid - r} C ${x} ${mid} ${nx} ${mid} ${nx} ${mid + r} L ${nx} ${cy}`;
      x = nx;
    }
  }
  return d;
}

// eslint-disable-next-line react/require-default-props
export default function TOC(props: OriginalTOCProps): React.ReactElement {
  const [isStepperPage, setIsStepperPage] = useState(false);

  const [tocEl,      setTocEl]      = useState<HTMLElement | null>(null);
  const resetTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [fillPct,    setFillPct]    = useState(0);
  const [pathD,      setPathD]      = useState('');
  const [svgTop,     setSvgTop]     = useState(0);
  const [svgHeight,  setSvgHeight]  = useState(0);
  const [linkCount,  setLinkCount]  = useState(1);
  const [itemYs,     setItemYs]     = useState<number[]>([]);
  const [itemNested, setItemNested] = useState<boolean[]>([]);
  const [itemIsStep, setItemIsStep] = useState<boolean[]>([]);

  useEffect(() => {
    const toc = document.querySelector<HTMLElement>('.theme-doc-toc-desktop');
    if (!toc) return;
    toc.style.setProperty('--ifm-toc-border-color', 'transparent');

    // Apply stepper padding BEFORE measuring geometry so positions are correct
    if (isStepperPage) {
      toc.querySelectorAll<HTMLElement>('a.table-of-contents__link').forEach(link => {
        link.style.paddingTop = '7px';
        link.style.paddingBottom = '7px';
      });
    }

    setTocEl(toc);

    const getVisibleLinks = () =>
      Array.from(toc.querySelectorAll<HTMLAnchorElement>('a.table-of-contents__link')).filter(
        link => ((link.closest('li') as HTMLElement | null)?.offsetHeight ?? 0) > 0,
      );

    const updateProgress = () => {
      const atBottom = window.scrollY + window.innerHeight >= document.documentElement.scrollHeight - 50;
      if (atBottom) {
        setFillPct(1);
        // Force-highlight the last visible link when Docusaurus won't reach it
        const allLinks = getVisibleLinks();
        if (allLinks.length && !allLinks[allLinks.length - 1].classList.contains('table-of-contents__link--active')) {
          allLinks.forEach(l => l.classList.remove('table-of-contents__link--active'));
          allLinks[allLinks.length - 1].classList.add('table-of-contents__link--active');
        }
        return;
      }
      const links = getVisibleLinks();
      if (!links.length) return;
      const ai = links.findIndex(l => l.classList.contains('table-of-contents__link--active'));
      if (ai === -1) {
        // Check if a *hidden* link (e.g. inactive tab heading) has the active class.
        const hiddenActive = toc.querySelector('a.table-of-contents__link--active');
        if (hiddenActive) return;

        // Fallback for headings Docusaurus doesn't track (e.g. H4 from Stepper).
        // Compute active index from scroll position using the heading elements directly.
        const anchors = links
          .map(link => {
            const id = link.getAttribute('href')?.slice(1);
            return id ? document.getElementById(id) : null;
          })
          .filter((el): el is HTMLElement => el !== null);
        if (anchors.length) {
          let activeI = 0;
          for (let i = 0; i < anchors.length; i++) {
            if (anchors[i].getBoundingClientRect().top - 100 <= 0) activeI = i;
          }
          if (anchors[activeI].getBoundingClientRect().top - 100 <= 0) {
            if (resetTimerRef.current !== null) {
              clearTimeout(resetTimerRef.current);
              resetTimerRef.current = null;
            }
            setFillPct((activeI + 0.5) / links.length);
            return;
          }
        }

        // Genuinely at top — debounce reset
        resetTimerRef.current ??= setTimeout(() => {
          setFillPct(0);
          resetTimerRef.current = null;
        }, 60);
        return;
      }
      if (resetTimerRef.current !== null) {
        clearTimeout(resetTimerRef.current);
        resetTimerRef.current = null;
      }
      setFillPct((ai + 0.5) / links.length);
    };

    const updateGeometry = () => {
      const tocRect = toc.getBoundingClientRect();
      const ul = toc.querySelector<HTMLElement>('ul.table-of-contents');
      if (!ul) return;
      const allLinks = getVisibleLinks();
      if (!allLinks.length) return;

      // Use content-relative y (add scrollTop) so the SVG covers the full
      // scrollable height, not just the visible viewport slice.
      const tocScrollTop = toc.scrollTop;
      const items: LinkItem[] = allLinks.map(link => {
        const r = link.getBoundingClientRect();
        return {
          y: r.top + r.height / 2 - tocRect.top + tocScrollTop,
          isNested: link.closest('ul') !== ul,
        };
      });

      setLinkCount(items.length);
      // Stepper headings have MuiTypography-root; plain markdown headings don't
      setItemIsStep(allLinks.map(link => {
        const id = link.getAttribute('href')?.slice(1);
        const el = id ? document.getElementById(id) : null;
        return el?.classList.contains('MuiTypography-root') ?? false;
      }));
      const minOffset = parseFloat(window.getComputedStyle(ul).paddingTop) || 0;
      const svgT = Math.max(minOffset, items[0].y);
      // No height cap — SVG covers the full scrollable content so it isn't clipped
      const lastBottom = allLinks[allLinks.length - 1].getBoundingClientRect().bottom - tocRect.top + tocScrollTop;
      const svgB = Math.max(items[items.length - 1].y, lastBottom);

      setSvgTop(svgT);
      setSvgHeight(Math.max(0, svgB - svgT));
      const translated = items.map(item => ({...item, y: item.y - svgT}));
      setItemYs(translated.map(i => i.y));
      setItemNested(translated.map(i => i.isNested));
      setPathD(buildPath(translated));
    };

    updateProgress();
    updateGeometry();
    // Re-run after tabs finish hiding inactive items
    const initTimer = setTimeout(() => { updateGeometry(); updateProgress(); }, 150);

    // Re-measure whenever tab switches change which TOC items are visible
    const ul = toc.querySelector<HTMLElement>('ul.table-of-contents');
    const resizeObserver = new ResizeObserver(() => { updateGeometry(); updateProgress(); });
    if (ul) resizeObserver.observe(ul);

    const observer = new MutationObserver(updateProgress);
    observer.observe(toc, {attributeFilter: ['class'], attributes: true, subtree: true});
    const onScroll = () => { updateGeometry(); updateProgress(); };
    window.addEventListener('scroll', onScroll, {passive: true});
    window.addEventListener('resize', updateGeometry);
    toc.addEventListener('scroll', updateGeometry, {passive: true});

    return () => {
      clearTimeout(initTimer);
      resizeObserver.disconnect();
      observer.disconnect();
      window.removeEventListener('scroll', onScroll);
      window.removeEventListener('resize', updateGeometry);
      toc.removeEventListener('scroll', updateGeometry);
    };
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // Active index derived from fillPct
  const activeIdx = Math.max(0, Math.min(Math.round(fillPct * linkCount - 0.5), linkCount - 1));

  // ── Normal-page indicator bounds ──────────────────────────────────────────
  const fallbackSeg = linkCount > 0 ? svgHeight / linkCount : svgHeight;
  const currY = itemYs[activeIdx] ?? fallbackSeg * (activeIdx + 0.5);
  const prevY = activeIdx > 0 ? (itemYs[activeIdx - 1] ?? currY - fallbackSeg) : currY - fallbackSeg;
  const nextY = activeIdx < linkCount - 1 ? (itemYs[activeIdx + 1] ?? currY + fallbackSeg) : currY + fallbackSeg;
  const indicatorTop    = fillPct === 0 ? 0 : Math.max(0, (prevY + currY) / 2);
  const indicatorHeight = fillPct === 0 ? 0 : Math.min((currY + nextY) / 2, svgHeight) - indicatorTop;

  // ── Stepper: number only items that are actual Stepper steps ─────────────
  let stepNum = 0;
  const stepNumbers = itemNested.map((nested, i) =>
    nested || !itemIsStep[i] ? null : ++stepNum,
  );

  const primary = 'var(--ifm-color-primary)';
  const muted   = 'var(--oxygen-palette-divider)';

  // Clip stops at: top of active circle (circles light up via per-element transition),
  // or at the dot center (dot lights up via per-element transition-delay after line arrives).
  const activeIsCircle = !itemNested[activeIdx] && stepNumbers[activeIdx] !== null;
  const stepperClipH = fillPct === 0
    ? 0
    : activeIsCircle
      ? Math.max(0, (itemYs[activeIdx] ?? 0) - S_R)  // stop at top of circle
      : (itemYs[activeIdx] ?? 0);                      // stop at dot center

  return (
    <>
      <BrowserOnly>
        {() => (
          <DocContextGuard>
            <StepperPageDetector onDetect={setIsStepperPage} />
          </DocContextGuard>
        )}
      </BrowserOnly>
      {tocEl && createPortal(
        isStepperPage ? (
          // ── Stepper design: two-layer clip — single rect drives all sync ────
          <svg
            aria-hidden="true"
            style={{
              height: svgHeight, left: `${S_LEFT}px`, overflow: 'visible',
              pointerEvents: 'none', position: 'absolute', top: svgTop, width: `${S_W}px`,
            }}
          >
            <defs>
              <clipPath id="stepper-fill-clip">
                <rect x={0} y={0} width={S_W} height={stepperClipH}
                  style={{transition: fillPct === 0 ? 'none' : 'height 250ms ease'}}
                />
              </clipPath>
            </defs>

            {/* Gray base lines */}
            {stepperLines(itemYs, itemNested, muted)}
            {/* Blue lines — clipped; line travels to dot center, then dot lights up with delay */}
            <g clipPath="url(#stepper-fill-clip)">
              {stepperLines(itemYs, itemNested, primary)}
            </g>
            {/* All circles + dots — per-element transitions; dots have delay so line arrives first */}
            {stepperNodes(itemYs, itemNested, stepNumbers, activeIdx, fillPct, primary, muted)}
          </svg>
        ) : (
          // ── Normal design: snake path + moving indicator ─────────────────
          <svg
            aria-hidden="true"
            style={{
              height: svgHeight, left: '24px', overflow: 'visible',
              pointerEvents: 'none', position: 'absolute', top: svgTop,
              width: `${X_INNER + 2}px`,
            }}
          >
            <defs>
              <clipPath id="toc-fill-clip">
                <rect
                  x={X_OUTER - 2} y={0} width={X_INNER + 4} height={indicatorHeight}
                  style={{
                    transform: `translateY(${indicatorTop}px)`,
                    transition: fillPct === 0 ? 'none' : 'transform 250ms ease, height 250ms ease',
                  }}
                />
              </clipPath>
            </defs>
            <path d={pathD} fill="none" stroke={muted} strokeLinecap="round" strokeWidth="1.5" />
            <path d={pathD} fill="none" stroke={primary} strokeLinecap="round" strokeWidth="2"
              clipPath="url(#toc-fill-clip)" />
          </svg>
        ),
        tocEl,
      )}
      <OriginalTOC {...props} />
    </>
  );
}
