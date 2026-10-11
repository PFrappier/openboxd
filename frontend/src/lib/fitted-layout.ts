import { onBeforeUnmount, onMounted, ref, type Ref, type ShallowRef } from 'vue'

/**
 * Keeps a layout fitted to the room the screen leaves to an element of a page: `fit` is
 * given its width, and the height of the screen less everything above and under it in the
 * page, its <main>, such as the top bar and the title above, the pagination under.
 *
 * The layout is null until the element is mounted, and only changes when one of its
 * values does.
 */
export function useFittedLayout<L extends object>(
  area: Readonly<ShallowRef<HTMLElement | null>>,
  fit: (width: number, height: number) => L,
): Ref<L | null> {
  let pageElement: HTMLElement | null = null
  const layout = ref<L | null>(null) as Ref<L | null>

  function updateLayout() {
    if (!pageElement || !area.value) return
    const pageBox = pageElement.getBoundingClientRect()
    const areaBox = area.value.getBoundingClientRect()
    const above = areaBox.top + window.scrollY
    const under = pageBox.bottom - areaBox.bottom
    const fitted = fit(area.value.clientWidth, window.innerHeight - above - under)
    const current = layout.value
    const keys = Object.keys(fitted) as (keyof L)[]
    if (!current || keys.some((key) => fitted[key] !== current[key])) layout.value = fitted
  }

  // Also covers what changes the room left without resizing the window, such as the
  // pagination showing up once the number of items is known.
  const pageObserver = new ResizeObserver(updateLayout)

  onMounted(() => {
    pageElement = area.value?.closest('main') ?? null
    updateLayout()
    if (pageElement) pageObserver.observe(pageElement)
    window.addEventListener('resize', updateLayout)
  })

  onBeforeUnmount(() => {
    pageObserver.disconnect()
    window.removeEventListener('resize', updateLayout)
  })

  return layout
}
