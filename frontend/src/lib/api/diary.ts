import { toApiError } from './client'

/** One time a film was watched. */
export interface Viewing {
  id: number
  /** Day the film was watched, as YYYY-MM-DD. */
  watchedOn: string
  /**
   * The rating given along with this viewing, in stars from 0.5 to 5: it can differ from
   * the current rating of the film. Null without one.
   */
  rating: number | null
  /** Whether the film had been watched before. */
  rewatch: boolean
  /** Whether the user wrote a review along with this viewing. */
  reviewed: boolean
}

/** A viewing along with the film it is about. */
export interface DiaryEntry extends Viewing {
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

export interface DiaryPage {
  /** Number of entries the filter keeps, not just in this page. */
  total: number
  /** The years the whole diary has viewings in, latest first, whatever the filter. */
  years: number[]
  entries: DiaryEntry[]
}

/** Narrows the diary down; a filter left out keeps every entry. */
export interface DiaryFilter {
  /** The year of the viewings, e.g. "2024". */
  year?: string
  /** A number of stars from 0.5 to 5, or "none" for the viewings without a rating. */
  rating?: string
  /** "true" for liked films only. */
  liked?: string
  /** "true" for films watched before only. */
  rewatch?: string
}

/** Orders to list the diary in: by what, then in which direction. */
export const DIARY_SORTS = [
  // The day the film was watched.
  'watched-desc',
  'watched-asc',
  // The rating given along with the viewing.
  'rating-desc',
  'rating-asc',
  'name-asc',
  'name-desc',
  // The year the film was released.
  'release-desc',
  'release-asc',
] as const

export type DiarySort = (typeof DIARY_SORTS)[number]

/**
 * Returns a page of the entries of the diary the filter keeps, in the order of the sort:
 * latest viewings first by default.
 */
export async function getDiary(
  page: { limit: number; offset: number },
  filter: DiaryFilter = {},
  sort: DiarySort = 'watched-desc',
): Promise<DiaryPage> {
  const query = new URLSearchParams({
    ...filter,
    sort,
    limit: String(page.limit),
    offset: String(page.offset),
  })
  const response = await fetch(`/api/diary?${query}`)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as DiaryPage
}
