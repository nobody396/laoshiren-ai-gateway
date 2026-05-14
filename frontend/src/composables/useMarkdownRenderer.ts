import { ref, watch, type Ref } from 'vue'
import { Marked, type Tokens } from 'marked'
import DOMPurify from 'dompurify'

export interface TocItem {
  id: string
  text: string
  level: number
}

export function useMarkdownRenderer(source: Ref<string>) {
  const renderedHtml = ref('')
  const tocItems = ref<TocItem[]>([])

  watch(
    source,
    (raw) => {
      if (!raw) {
        renderedHtml.value = ''
        tocItems.value = []
        return
      }

      const headings: TocItem[] = []
      const idCounts: Record<string, number> = {}

      const instance = new Marked({
        breaks: true,
        gfm: true,
      })

      instance.use({
        renderer: {
          heading(this: { parser: { parseInline: (tokens: Tokens.Generic[]) => string } }, token: Tokens.Heading) {
            const text = this.parser.parseInline(token.tokens)
            const plainText = text.replace(/<[^>]*>/g, '').trim()
            let id = plainText
              .toLowerCase()
              .replace(/[^\w\u4e00-\u9fff]+/g, '-')
              .replace(/^-|-$/g, '')

            // Deduplicate ids
            if (idCounts[id] !== undefined) {
              idCounts[id]++
              id = `${id}-${idCounts[id]}`
            } else {
              idCounts[id] = 0
            }

            headings.push({ id, text: plainText, level: token.depth })
            return `<h${token.depth} id="${id}">${text}</h${token.depth}>\n`
          },
        },
      })

      const html = instance.parse(raw) as string

      renderedHtml.value = DOMPurify.sanitize(html)
      tocItems.value = headings
    },
    { immediate: true },
  )

  return { renderedHtml, tocItems }
}
