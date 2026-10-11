<script setup lang="ts">
import { h, type VNodeChild } from 'vue'

const props = defineProps<{
  /** The text of a review, as written on Letterboxd. */
  text: string
}>()

// The tags Letterboxd formats reviews with, and the classes they are drawn with. Any other
// tag is dropped, and what it holds kept.
const TAGS: Record<string, string> = {
  a: 'underline underline-offset-4 transition-colors hover:text-muted-foreground',
  b: 'font-semibold',
  strong: 'font-semibold',
  i: 'italic',
  em: 'italic',
  blockquote: 'my-2 border-l-2 pl-3 text-muted-foreground',
  br: '',
}

/** Whether a link leads to a web page, and not to a script for instance. */
function isWebLink(href: string | null): href is string {
  try {
    return href !== null && ['http:', 'https:'].includes(new URL(href).protocol)
  } catch {
    return false
  }
}

function render(nodes: NodeListOf<ChildNode>): VNodeChild[] {
  return Array.from(nodes, (node): VNodeChild => {
    if (node.nodeType === Node.TEXT_NODE) return node.textContent
    if (!(node instanceof Element)) return null
    const tag = node.tagName.toLowerCase()
    // What a script or a style holds isn't text of the review.
    if (tag === 'script' || tag === 'style') return null
    const children = render(node.childNodes)
    if (!(tag in TAGS)) return children
    if (tag !== 'a') return h(tag, { class: TAGS[tag] }, children)
    const href = node.getAttribute('href')
    if (!isWebLink(href)) return children
    return h('a', { href, target: '_blank', rel: 'noopener nofollow', class: TAGS.a }, children)
  })
}

// The text is never set as HTML: it is parsed in a document of its own, where nothing
// runs nor loads, and only what is known of it is drawn.
const Content = () =>
  render(new DOMParser().parseFromString(props.text, 'text/html').body.childNodes)
</script>

<template>
  <!-- Line breaks are the ones of the text. -->
  <div class="leading-relaxed text-pretty wrap-anywhere whitespace-pre-line">
    <Content />
  </div>
</template>
