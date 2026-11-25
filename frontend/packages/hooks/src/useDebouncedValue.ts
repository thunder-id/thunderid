// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, useState} from 'react';

/**
 * The value once it has stayed the same for `delay` milliseconds, so typing asks the server once. Both
 * state writes go through a function, so a function value is kept as it is rather than called as an
 * initializer or an updater.
 */
export default function useDebouncedValue<T>(value: T, delay: number): T {
  const [settled, setSettled] = useState<T>(() => value);
  useEffect(() => {
    const timer = setTimeout(() => setSettled(() => value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);
  return settled;
}
