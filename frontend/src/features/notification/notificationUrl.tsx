import React from 'react'

export interface NotificationTextSegment {
  type: 'text' | 'link'
  text: string
  url?: string
}

/**
 * Menentukan apakah notifikasi berasal dari jenis Notifikasi Manual (`manual_push`)
 * berdasarkan sumber data tepercaya (event_type & dedup_key), bukan dari isi teks.
 * Notifikasi yang dibuat oleh Scheduler (`dedup_key` diawali `schedule:`) atau
 * notifikasi sistem lainnya tidak termasuk Notifikasi Manual.
 */
export function isManualNotification(
  eventType?: string | null,
  dedupKey?: string | null,
): boolean {
  const normalizedType = (eventType ?? '').trim()
  if (normalizedType !== 'manual_push') {
    return false
  }
  const normalizedDedup = (dedupKey ?? '').trim()
  if (normalizedDedup.startsWith('schedule:')) {
    return false
  }
  return true
}

/**
 * Memvalidasi apakah string kandidat merupakan URL HTTP/HTTPS yang aman dan valid.
 * Mengembalikan URL asli tanpa mutasi query string/parameter jika valid, atau null jika tidak valid.
 */
export function validateSafeHttpUrl(rawUrl: string): string | null {
  if (!rawUrl || typeof rawUrl !== 'string') {
    return null
  }
  const trimmed = rawUrl.trim()
  if (!trimmed) {
    return null
  }

  // Wajib diawali http:// atau https:// secara eksplisit
  if (!/^https?:\/\//i.test(trimmed)) {
    return null
  }

  // Tolak karakter spasi atau karakter kontrol di dalam URL
  if (/[\s\u0000-\u001F\u007F]/.test(trimmed)) {
    return null
  }

  try {
    const parsed = new URL(trimmed)
    if (parsed.protocol !== 'http:' && parsed.protocol !== 'https:') {
      return null
    }
    const host = parsed.hostname
    if (!host || host.startsWith('.') || host.endsWith('.') || host.includes('..')) {
      return null
    }
    // Pastikan bagian setelah skema memiliki authority yang valid (bukan http:///path atau http://:80)
    const afterScheme = trimmed.replace(/^https?:\/\//i, '')
    if (!afterScheme || afterScheme.startsWith('/') || afterScheme.startsWith(':')) {
      return null
    }
    return trimmed
  } catch {
    return null
  }
}

function countChar(str: string, ch: string): number {
  let count = 0
  for (let i = 0; i < str.length; i++) {
    if (str[i] === ch) count++
  }
  return count
}

/**
 * Memisahkan tanda baca penutup kalimat di akhir URL tanpa merusak query string atau tanda kurung yang seimbang.
 */
function splitTrailingPunctuation(rawToken: string): { candidate: string; trailing: string } {
  let candidate = rawToken
  let trailing = ''

  while (candidate.length > 0) {
    const lastChar = candidate[candidate.length - 1]
    if ('.!,;:?\'"'.includes(lastChar)) {
      trailing = lastChar + trailing
      candidate = candidate.slice(0, -1)
      continue
    }
    if (lastChar === ')' && countChar(candidate, ')') > countChar(candidate, '(')) {
      trailing = lastChar + trailing
      candidate = candidate.slice(0, -1)
      continue
    }
    if (lastChar === ']' && countChar(candidate, ']') > countChar(candidate, '[')) {
      trailing = lastChar + trailing
      candidate = candidate.slice(0, -1)
      continue
    }
    if (lastChar === '}' && countChar(candidate, '}') > countChar(candidate, '{')) {
      trailing = lastChar + trailing
      candidate = candidate.slice(0, -1)
      continue
    }
    break
  }

  return { candidate, trailing }
}

const HTTP_URL_CANDIDATE_REGEX = /(?:^|(?<=[\s([{"'<]))https?:\/\/[^\s<>"'`]+/gi

/**
 * Memecah teks notifikasi menjadi segmen teks biasa dan segmen link HTTP/HTTPS.
 * Jika `isManual` bernilai false, seluruh teks dikembalikan sebagai satu segmen teks biasa.
 */
export function parseNotificationTextSegments(
  text: string,
  isManual: boolean,
): NotificationTextSegment[] {
  if (!text) {
    return [{ type: 'text', text: '' }]
  }
  if (!isManual) {
    return [{ type: 'text', text }]
  }

  const segments: NotificationTextSegment[] = []
  let lastIndex = 0

  const appendText = (chunk: string) => {
    if (!chunk) return
    const prev = segments[segments.length - 1]
    if (prev && prev.type === 'text') {
      prev.text += chunk
    } else {
      segments.push({ type: 'text', text: chunk })
    }
  }

  HTTP_URL_CANDIDATE_REGEX.lastIndex = 0
  let match: RegExpExecArray | null
  while ((match = HTTP_URL_CANDIDATE_REGEX.exec(text)) !== null) {
    const matchStart = match.index
    const rawToken = match[0]

    if (matchStart > lastIndex) {
      appendText(text.slice(lastIndex, matchStart))
    }

    const { candidate, trailing } = splitTrailingPunctuation(rawToken)
    const validUrl = validateSafeHttpUrl(candidate)

    if (validUrl) {
      segments.push({
        type: 'link',
        text: validUrl,
        url: validUrl,
      })
      if (trailing) {
        appendText(trailing)
      }
    } else {
      appendText(rawToken)
    }

    lastIndex = matchStart + rawToken.length
  }

  if (lastIndex < text.length) {
    appendText(text.slice(lastIndex))
  }

  if (segments.length === 0) {
    return [{ type: 'text', text }]
  }

  return segments
}

/**
 * Merender isi notifikasi pada Web.
 * Hanya mengubah URL HTTP/HTTPS menjadi elemen <a> apabila notifikasi merupakan Notifikasi Manual.
 */
export function renderNotificationBody(
  text: string,
  eventType?: string | null,
  dedupKey?: string | null,
): React.ReactNode {
  const isManual = isManualNotification(eventType, dedupKey)
  if (!isManual || !text) {
    return text
  }

  const segments = parseNotificationTextSegments(text, true)
  const hasLink = segments.some((s) => s.type === 'link')
  if (!hasLink) {
    return text
  }

  return segments.map((seg, idx) => {
    if (seg.type === 'link' && seg.url) {
      return (
        <a
          key={`notif-link-${idx}`}
          href={seg.url}
          target="_blank"
          rel="noopener noreferrer"
          onClick={(e) => e.stopPropagation()}
          style={{
            color: 'var(--sf-accent, #3884ff)',
            textDecoration: 'underline',
            wordBreak: 'break-all',
          }}
        >
          {seg.text}
        </a>
      )
    }
    return <React.Fragment key={`notif-text-${idx}`}>{seg.text}</React.Fragment>
  })
}
