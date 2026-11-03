// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The schema of the flight booking type the Wayfinder sample registers, as the server returns it. Its
 * lists require at least one item, and its numbers carry limits.
 */
export const flightBookingSchema = {
  properties: {
    actions: {items: {enum: ['book'], type: 'string'}, minItems: 1, title: 'Actions', type: 'array'},
    flight: {
      properties: {
        from: {maxLength: 64, title: 'From', type: 'string'},
        id: {maxLength: 64, title: 'Flight', type: 'string'},
        to: {maxLength: 64, title: 'To', type: 'string'},
      },
      required: ['id', 'from', 'to'],
      type: 'object',
    },
    intent: {maxLength: 200, title: 'Goal', type: 'string'},
    locations: {items: {type: 'string'}, minItems: 1, title: 'Applies to', type: 'array'},
    maxPrice: {
      properties: {
        amount: {
          minimum: 0,
          policy: {modifiableBy: ['user'], modification: 'decrease'},
          title: 'Spend up to',
          type: 'number',
        },
        currency: {maxLength: 3, title: 'Currency', type: 'string'},
      },
      required: ['amount', 'currency'],
      type: 'object',
    },
    travelers: {
      maximum: 9,
      minimum: 1,
      policy: {modifiableBy: ['user'], modification: 'decrease'},
      title: 'Travelers',
      type: 'number',
    },
  },
  required: ['intent', 'actions', 'locations', 'flight', 'travelers', 'maxPrice'],
  type: 'object',
};

/** The flight booking type's consent summary. */
export const flightBookingTemplate =
  'Book flight {{flight.id}} from {{flight.from}} to {{flight.to}}, spending up to {{maxPrice.amount}} {{maxPrice.currency}}';
