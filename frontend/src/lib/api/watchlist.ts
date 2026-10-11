import { toApiError } from './client'

/** A film the user wants to watch. */
export interface WatchlistFilm {
  id: number
  name: string
  /** Null for films without a release year on Letterboxd. */
  year: number | null
  letterboxdUri: string
  /** Day the film was added to the watchlist, as YYYY-MM-DD. */
  addedOn: string
  /** Path of the poster on TMDB's image CDN; null until fetched or without one. */
  posterPath: string | null
  /** In minutes; null until fetched or when TMDB lacks it. */
  runtime: number | null
}

export interface WatchlistPage {
  /** Number of films the filter keeps, not just in this page. */
  total: number
  films: WatchlistFilm[]
}

/** Narrows the watchlist down; a filter left out keeps it all. */
export interface WatchlistFilter {
  /** First year of the decade the film was released in, e.g. "1990". */
  decade?: string
}

/** Orders to list the watchlist in: by what, then in which direction. */
export const WATCHLIST_SORTS = [
  // The day the film was added to the watchlist.
  'added-desc',
  'added-asc',
  'name-asc',
  'name-desc',
  // The year the film was released.
  'release-desc',
  'release-asc',
  'runtime-desc',
  'runtime-asc',
] as const

export type WatchlistSort = (typeof WATCHLIST_SORTS)[number]

/**
 * Returns a page of the films of the watchlist the filter keeps, in the order of the
 * sort: most recently added first by default.
 */
export async function getWatchlist(
  page: { limit: number; offset: number },
  filter: WatchlistFilter = {},
  sort: WatchlistSort = 'added-desc',
): Promise<WatchlistPage> {
  const query = new URLSearchParams({
    ...filter,
    sort,
    limit: String(page.limit),
    offset: String(page.offset),
  })
  const response = await fetch(`/api/watchlist?${query}`)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as WatchlistPage
}
