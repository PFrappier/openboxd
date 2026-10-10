import { toApiError } from './client'

/** A file of the export, with its path relative to the export root's parent folder. */
export interface ExportFile {
  file: File
  /** e.g. "letterboxd-user-2026-10-10/ratings.csv", or just the name for the ZIP archive. */
  path: string
}

/**
 * What an import brought in, as returned by `GET /api/imports/{id}`.
 *
 *   {
 *     "id": "…",
 *     "username": "someone",        // from profile.csv, null when absent
 *     "counts": {
 *       "watched": 1243,            // watched.csv
 *       "ratings": 1108,            // ratings.csv
 *       "diary": 872,               // diary.csv
 *       "reviews": 41,              // reviews.csv
 *       "watchlist": 215,           // watchlist.csv
 *       "likes": 307,               // likes/films.csv
 *       "lists": 12                 // number of files in lists/
 *     }
 *   }
 */
export interface ImportSummary {
  id: string
  username: string | null
  counts: Record<ImportSection, number>
}

export type ImportSection =
  'watched' | 'ratings' | 'diary' | 'reviews' | 'watchlist' | 'likes' | 'lists'

/** Sends the ZIP archive, or every CSV file of the unzipped folder, and returns the import ID. */
export async function uploadExport(files: ExportFile[]): Promise<string> {
  const body = new FormData()
  // The backend reads each part's filename as its relative path.
  for (const { file, path } of files) body.append('export', file, path)

  const response = await fetch('/api/imports', { method: 'POST', body })
  if (!response.ok) throw await toApiError(response)
  const { id } = (await response.json()) as { id: string }
  return id
}

export async function getImportSummary(id: string): Promise<ImportSummary> {
  const response = await fetch(`/api/imports/${encodeURIComponent(id)}`)
  if (!response.ok) throw await toApiError(response)
  return (await response.json()) as ImportSummary
}
