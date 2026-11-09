import React from 'react'
import { LinkOutlined } from '@ant-design/icons'

export interface TextSegment {
  url: string
  text: string
}

/**
 * Utility: detect and extract URLs from text.
 * Returns an array of { url: string, text: string } segments.
 * Only http/https URLs are matched; trailing punctuation is cleanly stripped
 * from both the URL and the link text, placing it into the following text segment.
 */
export function extractUrls(text: string): TextSegment[] {
  if (!text) return []
  const urlRegex = /https?:\/\/[^\s<>"{}|\\^`\[\]]+/g
  const trailingPunctRegex = /[.,;:!?)\]>}]+$/
  const segments: TextSegment[] = []
  let lastIndex = 0
  let match: RegExpExecArray | null

  while ((match = urlRegex.exec(text)) !== null) {
    // Text before URL
    if (match.index > lastIndex) {
      segments.push({ url: '', text: text.slice(lastIndex, match.index) })
    }
    const raw = match[0]
    const punctMatch = trailingPunctRegex.exec(raw)
    let cleanUrl = raw
    let trailingPunct = ''
    if (punctMatch) {
      cleanUrl = raw.slice(0, punctMatch.index)
      trailingPunct = raw.slice(punctMatch.index)
    }

    if (cleanUrl.startsWith('http://') || cleanUrl.startsWith('https://')) {
      segments.push({ url: cleanUrl, text: cleanUrl })
    } else {
      segments.push({ url: '', text: cleanUrl })
    }

    if (trailingPunct) {
      segments.push({ url: '', text: trailingPunct })
    }
    lastIndex = match.index + raw.length
  }
  if (lastIndex < text.length) {
    segments.push({ url: '', text: text.slice(lastIndex) })
  }
  return segments
}

/**
 * Render notification body text with clickable URLs.
 * Extracts http/https URLs from the body and renders them as anchors.
 * Preserves newlines and spaces via whiteSpace: 'pre-wrap'.
 */
export function renderNotificationBody(body: string): React.ReactNode {
  const segments = extractUrls(body)
  if (segments.length === 0) {
    return React.createElement('span', { style: { whiteSpace: 'pre-wrap' } }, body)
  }
  return React.createElement(
    'span',
    { style: { whiteSpace: 'pre-wrap' } },
    segments.map((seg, idx) =>
      seg.url
        ? React.createElement(
            'a',
            {
              key: idx,
              href: seg.url,
              target: '_blank',
              rel: 'noopener noreferrer',
              style: {
                color: '#A970FF',
                textDecoration: 'underline',
                wordBreak: 'break-all',
              },
            },
            seg.text,
            React.createElement(LinkOutlined, { style: { marginLeft: 4, fontSize: 11 } }),
          )
        : React.createElement('span', { key: idx }, seg.text),
    ),
  )
}
