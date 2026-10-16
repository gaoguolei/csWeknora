export type PreviewUrlError = 'invalid' | 'sameOrigin' | 'mixedContent'

/** URL previews are browser navigations, never server-side proxy requests. */
export function resolveProjectPreviewUrl(raw: string, appUrl: string, currentUrl = ''): string {
  const value = raw.trim()
  if (!value || /[\u0000-\u001f\u007f\\]/.test(value)) throw new Error('invalid' satisfies PreviewUrlError)
  let url: URL
  try {
    // Relative navigation is allowed only after an explicit project URL.
    url = new URL(value, currentUrl || undefined)
  } catch { throw new Error('invalid' satisfies PreviewUrlError) }
  if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) throw new Error('invalid' satisfies PreviewUrlError)
  const app = new URL(appUrl)
  // Scripted projects must run on a separate origin from the authenticated app.
  if (url.origin === app.origin) throw new Error('sameOrigin' satisfies PreviewUrlError)
  const loopback = ['localhost', '127.0.0.1', '[::1]'].includes(url.hostname)
  if (app.protocol === 'https:' && url.protocol === 'http:' && !loopback) throw new Error('mixedContent' satisfies PreviewUrlError)
  return url.href
}
