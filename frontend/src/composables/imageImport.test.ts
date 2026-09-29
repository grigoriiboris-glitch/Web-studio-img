import { describe, expect, it } from 'vitest'
import {
  extractImageFiles,
  extractImageFilesFromItems,
  extractImageUrls,
  formatSketchFileSize,
  isEditableEventTarget,
  isImageLikeDataTransfer,
} from './imageImport'

function createFile(type: string, name = 'sketch'): File {
  return new File(['image'], name, { type })
}

describe('image import helpers', () => {
  it('accepts JPEG, PNG and WebP files', () => {
    const files = [
      createFile('image/png', 'sketch.png'),
      createFile('text/plain', 'notes.txt'),
      createFile('image/jpeg', 'reference.jpg'),
      createFile('image/webp', 'other.webp'),
    ]

    expect(extractImageFiles(files).map(file => file.name)).toEqual([
      'sketch.png',
      'reference.jpg',
      'other.webp',
    ])
  })

  it('extracts image files from clipboard items', () => {
    const png = createFile('image/png', 'clipboard.png')
    const items = [
      { kind: 'string', type: 'text/plain', getAsFile: () => null },
      { kind: 'file', type: 'image/png', getAsFile: () => png },
      { kind: 'file', type: 'image/jpeg', getAsFile: () => null },
    ] as unknown as DataTransferItemList

    expect(extractImageFilesFromItems(items)).toEqual([png])
  })

  it('extracts public image URLs from text and HTML drag payloads', () => {
    expect(extractImageUrls('https://example.com/a.png\nnot-a-url')).toEqual([
      'https://example.com/a.png',
    ])
    expect(extractImageUrls('<img src="https://example.com/b.webp">')).toEqual([
      'https://example.com/b.webp',
    ])
  })

  it('recognizes URL-based image drag payloads', () => {
    const dataTransfer = {
      items: [],
      getData: (type: string) => type === 'text/uri-list' ? 'https://example.com/sketch.png' : '',
    } as unknown as DataTransfer

    expect(isImageLikeDataTransfer(dataTransfer)).toBe(true)
  })

  it('formats useful upload sizes', () => {
    expect(formatSketchFileSize(512)).toBe('512 B')
    expect(formatSketchFileSize(1536)).toBe('1.5 KB')
    expect(formatSketchFileSize(2 * 1024 * 1024)).toBe('2.0 MB')
  })

  it('does not treat editable controls as global paste targets', () => {
    const input = document.createElement('input')
    const div = document.createElement('div')
    div.setAttribute('contenteditable', 'true')

    expect(isEditableEventTarget(input)).toBe(true)
    expect(isEditableEventTarget(div)).toBe(true)
    expect(isEditableEventTarget(document.body)).toBe(false)
  })
})
