import { toApiError } from './client'

export interface WatchedFilm {
  id: number
  name: string
  /** Null for films without a release year on Letterboxd. */
  year: number | null
  letterboxdUri: string
  /** Day the film was marked as watched, as YYYY-MM-DD. */
  watchedOn: string
  /** In stars, from 0.5 to 5 by steps of 0.5; null for a film that isn't rated. */
  rating: number | null
  liked: boolean

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

/** A film of the library, as returned by `GET /api/films/{id}`. */
export interface Film extends Omit<WatchedFilm, 'watchedOn'> {
  /** Null for a film that isn't marked as watched. */
  watchedOn: string | null
  /** Null until the film is matched with TMDB, and when TMDB doesn't know it. */
  tmdbId: number | null
}

/** Widths TMDB serves posters in, among others. */
export type PosterWidth = 92 | 154 | 185 | 342 | 500 | 780

/** Returns the URL of a poster on TMDB's image CDN. */
export function posterUrl(posterPath: string, width: PosterWidth): string {
  return `https://image.tmdb.org/t/p/w${width}${posterPath}`
}

export interface WatchedFilmsPage {
  /** Number of watched films the filter keeps, not just in this page. */
  total: number
  films: WatchedFilm[]
}

/** Narrows the watched films down; a filter left out keeps them all. */
export interface WatchedFilmsFilter {
  /** A number of stars from 0.5 to 5, or "none" for the films that aren't rated. */
  rating?: string
  /** "true" for liked films only. */
  liked?: string
  /** First year of the decade the film was released in, e.g. "1990". */
  decade?: string
}

/** Returns a page of the watched films the filter keeps, most recently watched first. */
export async function getWatchedFilms(
  page: { limit: number; offset: number },
  filter: WatchedFilmsFilter = {},
): Promise<WatchedFilmsPage> {
  const query = new URLSearchParams({
    ...filter,
    limit: String(page.limit),
    offset: String(page.offset),
  })
  const response = await fetch(`/api/watched?${query}`)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as WatchedFilmsPage
}

/** Returns a film of the library; the error has status 404 when there is no such film. */
export async function getFilm(id: number | string): Promise<Film> {
  const response = await fetch(`/api/films/${encodeURIComponent(id)}`)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as Film
}
