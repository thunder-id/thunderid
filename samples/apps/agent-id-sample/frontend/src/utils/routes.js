export function readCriteria(search) {
  const params = new URLSearchParams(search);

  return {
    category: params.get("category") || "flights",
    tripType: params.get("tripType") || "round-trip",
    from: params.get("from") || "",
    to: params.get("to") || "",
    dates: params.get("dates") || "",
    travelers: params.get("travelers") || ""
  };
}

export function buildResultsPath(criteria) {
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(criteria)) {
    if (value) {
      params.set(key, value);
    }
  }

  return `/results?${params.toString()}`;
}

export function buildFlightDetailsPath(flightId, criteria = {}) {
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(criteria)) {
    if (value) {
      params.set(key, value);
    }
  }

  const query = params.toString();

  return `/flights/${encodeURIComponent(flightId)}${query ? `?${query}` : ""}`;
}

export function buildFlightPaymentPath(flightId, criteria = {}) {
  const params = new URLSearchParams();

  for (const [key, value] of Object.entries(criteria)) {
    if (value) {
      params.set(key, value);
    }
  }

  const query = params.toString();

  return `/payment/flight/${encodeURIComponent(flightId)}${query ? `?${query}` : ""}`;
}
