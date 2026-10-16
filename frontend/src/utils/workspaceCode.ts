import hljs from 'highlight.js'

const languages: Record<string, string> = {
  js: 'javascript', jsx: 'javascript', mjs: 'javascript', cjs: 'javascript',
  ts: 'typescript', tsx: 'typescript', mts: 'typescript', cts: 'typescript',
  vue: 'xml', svelte: 'xml', html: 'xml', htm: 'xml', svg: 'xml',
  py: 'python', pyw: 'python', rb: 'ruby', rs: 'rust', sh: 'bash', zsh: 'bash',
  yml: 'yaml', md: 'markdown', mdx: 'markdown', jsonc: 'json', json5: 'json',
  toml: 'ini', h: 'c', hpp: 'cpp', cc: 'cpp', cs: 'csharp', kt: 'kotlin',
  kts: 'kotlin', ps1: 'powershell', gql: 'graphql', tf: 'hcl',
}

export function workspaceCodeLanguage(path: string): string {
  const name = path.replaceAll('\\', '/').split('/').pop()?.toLowerCase() || ''
  if (/^dockerfile(?:\.|$)/.test(name)) return 'dockerfile'
  if (/^(?:gnumakefile|makefile)(?:\.|$)/.test(name)) return 'makefile'
  if (/^\.env(?:\.|$)/.test(name)) return 'ini'
  const ext = name.split('.').pop() || ''
  return languages[ext] || (hljs.getLanguage(ext) ? ext : 'plaintext')
}

function escapeCode(source: string): string {
  return source.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;')
}

/** Highlight the whole document first so multiline strings/comments keep their grammar.
 * Balance spans on each row so wrapped lines and diff gutters can share a row height. */
export function highlightWorkspaceLines(source: string, path: string): string[] {
  const language = workspaceCodeLanguage(path)
  let html = escapeCode(source)
  try {
    if (source.length <= 200_000 && hljs.getLanguage(language)) html = hljs.highlight(source, { language, ignoreIllegals: true }).value
  } catch { /* Incomplete streamed source still has a readable, escaped fallback. */ }
  const spans: string[] = []
  return html.split('\n').map(line => {
    const prefix = spans.join('')
    for (const match of line.matchAll(/<span\b[^>]*>|<\/span>/g)) {
      if (match[0] === '</span>') spans.pop()
      else spans.push(match[0])
    }
    return prefix + line + '</span>'.repeat(spans.length)
  })
}

/** Remove shared display indentation without changing the highlighted span tree. */
export function trimHighlightedIndent(html: string, count: number): string {
  let remaining = count
  return html.replace(/(<[^>]+>)|([^<]+)/g, (token, tag: string | undefined, text: string | undefined) => {
    if (tag || !remaining || !text) return token
    const whitespace = /^[\t ]*/.exec(text)![0].length
    const removed = Math.min(remaining, whitespace)
    remaining -= removed
    if (whitespace < text.length) remaining = 0
    return text.slice(removed)
  })
}

/** Browser-only formatting of the displayed text; never write back to the workspace.
 * Unsupported languages, oversized files and unfinished edits keep their original text. */
export async function formatWorkspaceCode(source: string, path: string): Promise<string> {
  if (!source.trim() || source.length > 200_000) return source
  const ext = path.split('.').pop()?.toLowerCase() || ''
  const parser = ({
    js: 'babel', jsx: 'babel', mjs: 'babel', cjs: 'babel',
    ts: 'typescript', tsx: 'typescript', mts: 'typescript', cts: 'typescript',
    json: 'json-stringify', jsonc: 'json', json5: 'json5',
    css: 'css', scss: 'scss', less: 'less', html: 'html', htm: 'html', vue: 'vue',
    yaml: 'yaml', yml: 'yaml', md: 'markdown', markdown: 'markdown', mdx: 'mdx',
    graphql: 'graphql', gql: 'graphql',
  } as Record<string, string>)[ext]
  if (!parser) return source
  try {
    const { format } = await import('prettier/standalone')
    const plugins = await (
      ['html', 'vue'].includes(parser)
        ? Promise.all([import('prettier/plugins/html'), import('prettier/plugins/babel'), import('prettier/plugins/typescript'), import('prettier/plugins/estree'), import('prettier/plugins/postcss')])
        : ['babel', 'typescript', 'json', 'json5', 'json-stringify'].includes(parser)
          ? Promise.all([parser === 'typescript' ? import('prettier/plugins/typescript') : import('prettier/plugins/babel'), import('prettier/plugins/estree')])
          : ['css', 'scss', 'less'].includes(parser)
            ? Promise.all([import('prettier/plugins/postcss')])
            : ['markdown', 'mdx'].includes(parser)
              ? Promise.all([import('prettier/plugins/markdown')])
              : parser === 'yaml'
                ? Promise.all([import('prettier/plugins/yaml')])
                : Promise.all([import('prettier/plugins/graphql')])
    )
    return await format(source, { parser, plugins: plugins.map(plugin => plugin.default), printWidth: 80, tabWidth: 2, useTabs: false })
  } catch {
    return source
  }
}
