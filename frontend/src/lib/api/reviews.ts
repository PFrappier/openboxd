import { toApiError } from './client'

/** Something the user wrote about a film. */
export interface Review {
  id: number
  /** Day the review was written, as YYYY-MM-DD. */
  reviewedOn: string
  /** Day of the viewing the review is about, as YYYY-MM-DD; null without a logged one. */
  watchedOn: string | null
  /**
   * The rating given along with the review, in stars from 0.5 to 5: it can differ from the
   * current rating of the film. Null without one.
   */
  rating: number | null
  /** Whether the film had been watched before. */
  rewatch: boolean
  /**
   * As written on Letterboxd, which allows a few HTML tags in it: it isn't safe to render
   * as HTML, see `ReviewText`.
   */
  text: string
}

/** A review along with the film it is about. */
export interface ReviewEntry extends Review {
  film: {
    id: number
    name: string
    /** Null for films without a release year on Letterboxd. */
    year: number | null
    /** Path of the poster on TMDB's image CDN; null until fetched or without one. */
    posterPath: string | null
    liked: boolean
  }
}

export interface ReviewsPage {
  /** Number of reviews, not just in this page. */
  total: number
  reviews: ReviewEntry[]
}

/** Orders to list the reviews in: by what, then in which direction. */
export const REVIEWS_SORTS = [
  // The day the review was written.
  'reviewed-desc',
  'reviewed-asc',
  // The rating given along with the review.
  'rating-desc',
  'rating-asc',
  'name-asc',
  'name-desc',
] as const

export type ReviewsSort = (typeof REVIEWS_SORTS)[number]

/** Returns a page of the reviews, in the order of the sort: latest first by default. */
export async function getReviews(
  page: { limit: number; offset: number },
  sort: ReviewsSort = 'reviewed-desc',
): Promise<ReviewsPage> {
  const query = new URLSearchParams({
    sort,
    limit: String(page.limit),
    offset: String(page.offset),
  })
  const response = await fetch(`/api/reviews?${query}`)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as ReviewsPage
}
