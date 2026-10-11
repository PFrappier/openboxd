// In pixels: the gap of the grid of films, what a card adds under its poster for its
// footer, and the width under which posters get too small to be told apart.
export const FILM_GRID_GAP = 12
const CARD_FOOTER = 22
const MIN_CARD_WIDTH = 64

export interface FilmGridLayout {
  columns: number
  /** Width of a card, in pixels. */
  cardWidth: number
  /** Height a row is given, in pixels: the one of a card with a footer. */
  rowHeight: number
}

/**
 * Lays a full page of films out in whole rows, with the largest cards that fit both the
 * width and the height given. When the space is too small for that, the height gives in:
 * the cards keep a decent size and the page scrolls.
 *
 * Cards are fitted as if they had a footer even when they have none: every list of films
 * is then as tall, and what sits under the grid doesn't move from one to the other.
 * Posters without a footer grow into its room where the width allows it.
 */
export function fitFilmGrid(
  width: number,
  height: number,
  pageSize: number,
  hasFooter = true,
): FilmGridLayout {
  const widthFor = (columns: number) => (width - FILM_GRID_GAP * (columns - 1)) / columns
  const heightFor = (columns: number) => {
    const rows = pageSize / columns
    return (height - FILM_GRID_GAP * (rows - 1)) / rows
  }
  const candidates: number[] = []
  for (let columns = 1; columns <= pageSize; columns++) {
    if (pageSize % columns === 0) candidates.push(columns)
  }
  const layouts = candidates.map((columns) => ({
    columns,
    // Posters are 2:3.
    cardWidth: Math.floor(
      Math.min(widthFor(columns), ((heightFor(columns) - CARD_FOOTER) * 2) / 3),
    ),
  }))
  let fitted = layouts.reduce((a, b) => (b.cardWidth > a.cardWidth ? b : a))
  if (fitted.cardWidth < MIN_CARD_WIDTH) {
    // The most columns the width allows make for the shortest scroll.
    const columns = Math.max(1, ...candidates.filter((c) => widthFor(c) >= MIN_CARD_WIDTH))
    fitted = { columns, cardWidth: Math.floor(widthFor(columns)) }
  }

  const { columns } = fitted
  const rowHeight = fitted.cardWidth * 1.5 + CARD_FOOTER
  if (hasFooter) return { ...fitted, rowHeight }
  const cardWidth = Math.floor(Math.min(widthFor(columns), (rowHeight * 2) / 3))
  return { columns, cardWidth, rowHeight }
}

/** Height of a full page of films laid out so, in pixels. */
export function filmGridHeight({ columns, rowHeight }: FilmGridLayout, pageSize: number) {
  const rows = pageSize / columns
  return rows * rowHeight + (rows - 1) * FILM_GRID_GAP
}
