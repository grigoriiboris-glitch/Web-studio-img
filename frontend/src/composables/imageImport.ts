export const SUPPORTED_SKETCH_MIME_TYPES = new Set([
  'image/jpeg',
  'image/png',
  'image/webp',
])

export const MAX_SKETCH_FILE_SIZE = 10 * 1024 * 1024

export function isSupportedSketchMimeType(mime: string): boolean {
  return SUPPORTED_SKETCH_MIME_TYPES.has(mime)
}

export function isSupportedSketchFile(file: File): boolean {
  return isSupportedSketchMimeType(file.type)
}

export function extractImageFiles(files: FileList | File[]): File[] {
  return Array.from(files).filter(isSupportedSketchFile)
}

export function extractImageFilesFromItems(items: DataTransferItemList): File[] {
  const files: File[] = []
  for (const item of Array.from(items)) {
    if (item.kind !== 'file' || !isSupportedSketchMimeType(item.type)) continue
    const file = item.getAsFile()
    if (file) files.push(file)
  }
  return files
}

export function extractImageUrls(raw: string): string[] {
  const urls = new Set<string>()
  const add = (value: string) => {
    const candidate = value.trim()
    if (!candidate || candidate.startsWith('#')) return
    try {
      const url = new URL(candidate)
      if ((url.protocol === 'http:' || url.protocol === 'https:') && !url.username && !url.password) {
        urls.add(url.href)
      }
    } catch {
      // Ignore clipboard text that is not a URL.
    }
  }

  for (const line of raw.split(/\r?\n/)) add(line)

  const htmlSources = raw.matchAll(/<img[^>]+src=["']([^"']+)["']/gi)
  for (const match of htmlSources) add(match[1])

  return [...urls]
}

export function extractImageUrlsFromDataTransfer(dataTransfer: DataTransfer | null): string[] {
  if (!dataTransfer) return []
  for (const type of ['text/uri-list', 'text/html', 'text/plain']) {
    const value = dataTransfer.getData(type)
    if (value) {
      const urls = extractImageUrls(value)
      if (urls.length) return urls
    }
  }
  return []
}

export function isImageLikeDataTransfer(dataTransfer: DataTransfer | null): boolean {
  if (!dataTransfer) return false
  if (Array.from(dataTransfer.items).some(item => item.kind === 'file' && isSupportedSketchMimeType(item.type))) {
    return true
  }
  return extractImageUrlsFromDataTransfer(dataTransfer).length > 0
}

export function formatSketchFileSize(size: number): string {
  if (size < 1024) return size + ' B'
  if (size < 1024 * 1024) return (size / 1024).toFixed(1) + ' KB'
  return (size / (1024 * 1024)).toFixed(1) + ' MB'
}

export function isEditableEventTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable) return true
  if (target.closest('[contenteditable]:not([contenteditable="false"])')) return true
  return ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}
