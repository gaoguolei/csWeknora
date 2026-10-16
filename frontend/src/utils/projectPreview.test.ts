import assert from 'node:assert/strict'
import test from 'node:test'
import { resolveProjectPreviewUrl as resolve } from './projectPreview'

const app = 'https://app.example.com/chat'
test('project URLs support HTTPS, published ports, and relative routes', () => {
  assert.equal(resolve(' https://preview.example.com/path?q=1#demo ', app), 'https://preview.example.com/path?q=1#demo')
  assert.equal(resolve('http://localhost:5173', app), 'http://localhost:5173/')
  assert.equal(resolve('/about?q=2', app, 'https://preview.example.com/path'), 'https://preview.example.com/about?q=2')
  assert.equal(resolve('http://sandbox.example.com:3000', 'http://app.example.com'), 'http://sandbox.example.com:3000/')
})
test('invalid or privileged addresses cannot become iframe navigations', () => {
  for (const url of ['', '/about', 'localhost:5173', 'javascript:alert(1)', 'data:text/html,hi', 'file:///etc/passwd', 'https://user:pass@example.com', 'https://example.com/\nfoo', 'https:\\example.com']) {
    assert.throws(() => resolve(url, app), /^Error: invalid$/)
  }
  assert.throws(() => resolve('https://app.example.com/other', app), /^Error: sameOrigin$/)
  assert.throws(() => resolve('http://remote.example.com:3000', app), /^Error: mixedContent$/)
})
