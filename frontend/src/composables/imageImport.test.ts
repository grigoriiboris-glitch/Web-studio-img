import { describe, expect, it } from 'vitest'
import {
  extractImageFiles,
  extractImageFilesFromItems,
  isEditableEventTarget,
} from './imageImport'

function createFile(type: string, name = 'sketch'): File {
  return new File(['image'], name, { type })
}

describe('image import helpers', () => {
  it('accepts JPEG and PNG files from drag/drop or file input', () => {
    const files = [
      createFile('image/png', 'sketch.png'),
      createFile('text/plain', 'notes.txt'),
      createFile('image/jpeg', 'reference.jpg'),
      createFile('image/webp', 'other.webp'),
    ]

    expect(extractImageFiles(files).map(file => file.name)).toEqual([
      'sketch.png',
      'reference.jpg',
    ])
  })

  it('extracts image files from clipboard items', () => {
    const png = createFile('image/png', 'clipboard.png')
    const text = createFile('text/plain', 'text.txt')
    const items = [
      { kind: 'string', type: 'text/plain', getAsFile: () => null },
      { kind: 'file', type: 'image/png', getAsFile: () => png },
      { kind: 'file', type: 'image/jpeg', getAsFile: () => null },
      { kind: 'file', type: 'text/plain', getAsFile: () => text },
    ] as unknown as DataTransferItemList

    expect(extractImageFilesFromItems(items)).toEqual([png])
  })

  it('does not treat editable controls as global paste targets', () => {
    const input = document.createElement('input')
    const div = document.createElement('div')
    div.contentEditable = 'true'

    expect(isEditableEventTarget(input)).toBe(true)
    expect(isEditableEventTarget(div)).toBe(true)
    expect(isEditableEventTarget(document.body)).toBe(false)
  })
})
