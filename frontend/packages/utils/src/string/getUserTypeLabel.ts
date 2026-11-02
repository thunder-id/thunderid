// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Returns the label to show for a user or agent type handle.
 *
 * The label is the display name. Display names are not unique, so when another type shares the display name, the
 * handle is appended to tell them apart. Falls back to the handle when the type is not in the list.
 *
 * @example
 * getUserTypeLabel([{handle: 'customer', displayName: 'Customer'}], 'customer') // => 'Customer'
 * getUserTypeLabel(
 *   [{handle: 'customer', displayName: 'Customer'}, {handle: 'retail-customer', displayName: 'Customer'}],
 *   'customer',
 * ) // => 'Customer (customer)'
 *
 * @param types - The available types.
 * @param handle - The handle of the type to label.
 * @returns The label for the type.
 */
export default function getUserTypeLabel(
  types: readonly {handle: string; displayName: string}[],
  handle: string,
): string {
  const type = types.find((item) => item.handle === handle);
  if (!type) {
    return handle;
  }

  const isDuplicate = types.some((item) => item.handle !== handle && item.displayName === type.displayName);
  return isDuplicate ? `${type.displayName} (${handle})` : type.displayName;
}
