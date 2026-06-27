import DOMPurify from 'dompurify'
import MarkdownIt from 'markdown-it'

const md = new MarkdownIt({
  breaks: true,
  html: false,
  linkify: true,
})

export function renderMarkdown(markdown: string): string {
  return DOMPurify.sanitize(md.render(markdown || ''))
}
