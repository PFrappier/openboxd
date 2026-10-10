import { toApiError } from './client'

export interface WatchedFilm {
  id: number
  name: string
  /** Null for films without a release year on Letterboxd. */
  year: number | null
  letterboxdUri: string
  /** Day the film was marked as watched, as YYYY-MM-DD. */
  watchedOn: string
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
