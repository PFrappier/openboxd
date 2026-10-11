// Dates are read as UTC so the day never shifts with the time zone.
const dateFormat = new Intl.DateTimeFormat('fr-FR', { dateStyle: 'medium', timeZone: 'UTC' })
const monthFormat = new Intl.DateTimeFormat('fr-FR', {
  month: 'long',
  year: 'numeric',
  timeZone: 'UTC',
})
const shortMonthFormat = new Intl.DateTimeFormat('fr-FR', {
  month: 'short',
  year: 'numeric',
  timeZone: 'UTC',
})
const weekdayFormat = new Intl.DateTimeFormat('fr-FR', { weekday: 'short', timeZone: 'UTC' })

/** Formats a YYYY-MM-DD date with format, or returns it as is when it isn't a date. */
function formatWith(format: Intl.DateTimeFormat, isoDate: string) {
  const date = new Date(`${isoDate}T00:00:00Z`)
  return Number.isNaN(date.getTime()) ? isoDate : format.format(date)
}

/** Formats a YYYY-MM-DD date, e.g. "5 mars 2024". */
export const formatDate = (isoDate: string) => formatWith(dateFormat, isoDate)

/** Returns the month of a YYYY-MM-DD date, e.g. "mars 2024". */
export const formatMonth = (isoDate: string) => formatWith(monthFormat, isoDate)

/** Returns the month of a YYYY-MM-DD date in short, e.g. "sept. 2024". */
export const formatShortMonth = (isoDate: string) => formatWith(shortMonthFormat, isoDate)

/** Returns the day of the week of a YYYY-MM-DD date, e.g. "mar.". */
export const formatWeekday = (isoDate: string) => formatWith(weekdayFormat, isoDate)
