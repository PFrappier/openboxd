// From the current decade back to the one of the first films.
const FIRST_DECADE = 1870
const CURRENT_DECADE = Math.floor(new Date().getFullYear() / 10) * 10

/** The first year of the decades films were released in, latest first. */
export const DECADES = Array.from(
  { length: (CURRENT_DECADE - FIRST_DECADE) / 10 + 1 },
  (_, index) => CURRENT_DECADE - index * 10,
)
