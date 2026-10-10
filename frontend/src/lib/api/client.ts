/** A response from the Openboxd API with an error status. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
  ) {
    super(message)
  }
}

/** Reads the `{ "error": "…" }` body the API sends along with error statuses. */
export async function toApiError(response: Response): Promise<ApiError> {
  const body = (await response.json().catch(() => null)) as { error?: string } | null
  return new ApiError(response.status, body?.error ?? response.statusText)
}
