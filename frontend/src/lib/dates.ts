const dateFormat = new Intl.DateTimeFormat('fr-FR', { dateStyle: 'medium', timeZone: 'UTC' })

/** Formats a YYYY-MM-DD date, read as UTC so the day never shifts with the time zone. */
export function formatDate(isoDate: string) {
  const date = new Date(`${isoDate}T00:00:00Z`)
  return Number.isNaN(date.getTime()) ? isoDate : dateFormat.format(date)
}
