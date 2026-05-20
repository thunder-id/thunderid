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
