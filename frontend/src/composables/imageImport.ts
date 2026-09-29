export const SUPPORTED_SKETCH_MIME_TYPES = new Set([
  'image/jpeg',
  'image/png',
])

export function isSupportedSketchFile(file: File): boolean {
  return SUPPORTED_SKETCH_MIME_TYPES.has(file.type)
}

export function extractImageFiles(files: FileList | File[]): File[] {
  return Array.from(files).filter(isSupportedSketchFile)
}

export function extractImageFilesFromItems(items: DataTransferItemList): File[] {
  const files: File[] = []
  for (const item of Array.from(items)) {
    if (item.kind !== 'file' || !SUPPORTED_SKETCH_MIME_TYPES.has(item.type)) continue
    const file = item.getAsFile()
    if (file) files.push(file)
  }
  return files
}

export function isEditableEventTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}
