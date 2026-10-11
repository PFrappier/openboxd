/** The ratings a film can get, in stars, best first. */
export const RATINGS = [5, 4.5, 4, 3.5, 3, 2.5, 2, 1.5, 1, 0.5]

/** e.g. "★★★½". */
export const ratingLabel = (stars: number) => '★'.repeat(Math.floor(stars)) + (stars % 1 ? '½' : '')
