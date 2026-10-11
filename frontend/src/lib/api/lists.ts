import { toApiError } from './client'

/** A list of films the user made. */
export interface FilmList {
  id: number
  name: string
  letterboxdUri: string
  /** Day the list was made, as YYYY-MM-DD. */
  createdOn: string
  /** Empty for a list without one. */
  description: string
  filmCount: number
  /**
   * Poster paths of the first films of the list, on TMDB's image CDN; null for a film
   * whose poster isn't fetched yet, or that has none.
   */
  posters: (string | null)[]
}

/** A film of a list. */
export interface ListFilm {
  id: number
  name: string
  /** Null for films without a release year on Letterboxd. */
  year: number | null
  letterboxdUri: string
  /** Rank of the film in the list, from 1. */
  position: number
  /** In stars, from 0.5 to 5 by steps of 0.5; null for a film that isn't rated. */
  rating: number | null
  liked: boolean
  /** Path of the poster on TMDB's image CDN; null until fetched or without one. */
  posterPath: string | null
}

export interface FilmListsPage {
  /** Number of lists, not just in this page. */
  total: number
  lists: FilmList[]
}

/** A list along with a page of its films; `list.filmCount` is how many it has in total. */
export interface FilmListPage {
  list: FilmList
  films: ListFilm[]
}

/** Orders to list the lists in: by what, then in which direction. */
export const LISTS_SORTS = [
  // The day the list was made.
  'created-desc',
  'created-asc',
  'name-asc',
  'name-desc',
  // The number of films of the list.
  'films-desc',
  'films-asc',
] as const

export type ListsSort = (typeof LISTS_SORTS)[number]

/** Returns a page of the lists, in the order of the sort: most recently made first by default. */
export async function getLists(
  page: { limit: number; offset: number },
  sort: ListsSort = 'created-desc',
): Promise<FilmListsPage> {
  const query = new URLSearchParams({
    sort,
    limit: String(page.limit),
    offset: String(page.offset),
  })
  const response = await fetch(`/api/lists?${query}`)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as FilmListsPage
}

/**
 * Returns a list and a page of its films, in the order of the list; the error has status
 * 404 when there is no such list.
 */
export async function getList(
  id: number | string,
  page: { limit: number; offset: number },
): Promise<FilmListPage> {
  const query = new URLSearchParams({ limit: String(page.limit), offset: String(page.offset) })
  const response = await fetch(`/api/lists/${encodeURIComponent(id)}?${query}`)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as FilmListPage
}
