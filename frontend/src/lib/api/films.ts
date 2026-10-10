import { toApiError } from './client'

export interface WatchedFilm {
  id: number
  name: string
  /** Null for films without a release year on Letterboxd. */
  year: number | null
  letterboxdUri: string
  /** Day the film was marked as watched, as YYYY-MM-DD. */
  watchedOn: string

  // The details below come from TMDB: they are null until the server fetched
  // them, and when TMDB lacks them.
  overview: string | null
  /** Path of the poster on TMDB's image CDN, see {@link posterUrl}. */
  posterPath: string | null
  /** In minutes. */
  runtime: number | null
  /** Names of the directors; empty when unknown. */
  directors: string[]
}

/** Widths TMDB serves posters in, among others. */
export type PosterWidth = 92 | 154 | 185 | 342 | 500 | 780

/** Returns the URL of a poster on TMDB's image CDN. */
export function posterUrl(posterPath: string, width: PosterWidth): string {
  return `https://image.tmdb.org/t/p/w${width}${posterPath}`
}

export interface WatchedFilmsPage {
  /** Number of watched films overall, not just in this page. */
  total: number
  films: WatchedFilm[]
}

/** Returns a page of watched films, most recently watched first. */
export async function getWatchedFilms(page: {
  limit: number
  offset: number
}): Promise<WatchedFilmsPage> {
  const query = new URLSearchParams({ limit: String(page.limit), offset: String(page.offset) })
  const response = await fetch(`/api/watched?${query}`)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as WatchedFilmsPage
}
