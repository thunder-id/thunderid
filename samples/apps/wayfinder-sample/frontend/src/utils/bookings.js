/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

export function formatPrice(currency, amount) {
  return `${currency === "USD" ? "$" : `${currency} `}${amount}`;
}

function formatBookingReference(bookingId) {
  const source = String(bookingId || "").replace(/^booking-/i, "");
  const letters = source.replace(/[^a-z]/gi, "").toUpperCase().padEnd(4, "WXYZ");
  const numbers = source.replace(/\D/g, "").padEnd(6, "202600");

  return `${letters.slice(0, 4)}-${numbers.slice(0, 6)}`;
}

export function getBookingReference(booking) {
  return booking?.bookingReference || formatBookingReference(booking?.id);
}

export function isSameFlight(firstFlight, secondFlight) {
  return (
    firstFlight?.from === secondFlight?.from &&
    firstFlight?.to === secondFlight?.to &&
    firstFlight?.departureTime === secondFlight?.departureTime &&
    firstFlight?.arrivalTime === secondFlight?.arrivalTime &&
    firstFlight?.dates === secondFlight?.dates
  );
}
