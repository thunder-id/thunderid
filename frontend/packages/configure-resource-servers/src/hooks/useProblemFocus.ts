// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, useState} from 'react';
import type {SaveErrorTarget} from './useAuthorizationDetailTypeForm';

/** A problem to bring into view and move focus to, in the part of the form shown with the given id. */
export interface ProblemFocusRequest {
  containerId: string;
  /** The value a failed save is about, or null for the first value marked invalid. */
  target: SaveErrorTarget | null;
}

const FOCUSABLE = 'input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

/**
 * Brings a problem into view and moves focus to it once the part of the form it is in is shown, as a
 * tab or a step opened for it shows only after the next render. The value a failed save is about is
 * marked with `data-save-error-target`; without a value to show, the part itself takes focus.
 */
export default function useProblemFocus(): (request: ProblemFocusRequest) => void {
  const [request, setRequest] = useState<ProblemFocusRequest | null>(null);

  useEffect(() => {
    if (!request) return;
    const container = document.getElementById(request.containerId);
    if (!container) return;
    const marked = container.querySelector<HTMLElement>(
      request.target ? `[data-save-error-target="${request.target}"]` : '[aria-invalid="true"]',
    );
    if (!marked) {
      container.scrollIntoView({block: 'start'});
      container.focus({preventScroll: true});
      return;
    }
    // Centred, so the save bar fixed at the bottom of the page does not cover it.
    marked.scrollIntoView({block: 'center'});
    const focusable = marked.matches(FOCUSABLE)
      ? marked
      : (marked.querySelector<HTMLElement>('[aria-invalid="true"]') ?? marked.querySelector<HTMLElement>(FOCUSABLE));
    focusable?.focus({preventScroll: true});
  }, [request]);

  return setRequest;
}
