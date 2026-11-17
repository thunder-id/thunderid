// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useEffect, useState} from 'react';

/** The value once it has stayed the same for `delay` milliseconds, so typing asks the server once. */
export default function useDebouncedValue<T>(value: T, delay: number): T {
  const [settled, setSettled] = useState(value);
  useEffect(() => {
    const timer = setTimeout(() => setSettled(value), delay);
    return () => clearTimeout(timer);
  }, [value, delay]);
  return settled;
}
