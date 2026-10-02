export type SpreadsheetRow = Record<string, string>

type ZipEntry = { name: string; compression: number; compressed: Uint8Array; uncompressedSize: number }

function u16(view: DataView, offset: number) { return view.getUint16(offset, true) }
function u32(view: DataView, offset: number) { return view.getUint32(offset, true) }

async function inflate(data: Uint8Array): Promise<Uint8Array> {
  if (typeof DecompressionStream === 'undefined') throw new Error('This browser does not support XLSX decompression. Use CSV export.')
  const stream = new Blob([data]).stream().pipeThrough(new DecompressionStream('deflate-raw'))
  return new Uint8Array(await new Response(stream).arrayBuffer())
}

async function readZip(buffer: ArrayBuffer): Promise<Map<string, Uint8Array>> {
  const bytes = new Uint8Array(buffer)
  const view = new DataView(buffer)
  let eocd = -1
  for (let i = bytes.length - 22; i >= Math.max(0, bytes.length - 65557); i--) {
    if (u32(view, i) === 0x06054b50) { eocd = i; break }
  }
  if (eocd < 0) throw new Error('Invalid XLSX/ZIP file')
  const count = u16(view, eocd + 10)
  const centralOffset = u32(view, eocd + 16)
  let cursor = centralOffset
  const entries: ZipEntry[] = []
  for (let i = 0; i < count; i++) {
    if (u32(view, cursor) !== 0x02014b50) throw new Error('Invalid XLSX central directory')
    const compression = u16(view, cursor + 10)
    const compressedSize = u32(view, cursor + 20)
    const uncompressedSize = u32(view, cursor + 24)
    const nameLen = u16(view, cursor + 28)
    const extraLen = u16(view, cursor + 30)
    const commentLen = u16(view, cursor + 32)
    const localOffset = u32(view, cursor + 42)
    const name = new TextDecoder().decode(bytes.slice(cursor + 46, cursor + 46 + nameLen))
    const localNameLen = u16(view, localOffset + 26)
    const localExtraLen = u16(view, localOffset + 28)
    const start = localOffset + 30 + localNameLen + localExtraLen
    entries.push({ name, compression, compressed: bytes.slice(start, start + compressedSize), uncompressedSize })
    cursor += 46 + nameLen + extraLen + commentLen
  }
  const out = new Map<string, Uint8Array>()
  for (const entry of entries) {
    let data: Uint8Array
    if (entry.compression === 0) data = entry.compressed
    else if (entry.compression === 8) data = await inflate(entry.compressed)
    else throw new Error('Unsupported XLSX compression method')
    if (entry.uncompressedSize && data.length !== entry.uncompressedSize) throw new Error('Corrupt XLSX entry: ' + entry.name)
    out.set(entry.name, data)
  }
  return out
}

function xmlText(data: Uint8Array): string {
  return new TextDecoder('utf-8').decode(data)
}
function esc(value: string): string {
  return value.replace(/&amp;/g, '&').replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&quot;/g, '"').replace(/&apos;/g, "'")
}
function attr(tag: string, name: string): string {
  const m = tag.match(new RegExp(name + '="([^"]*)"'))
  return m ? esc(m[1]) : ''
}
function columnNumber(ref: string): number {
  const letters = (ref.match(/^[A-Z]+/i)?.[0] ?? '').toUpperCase()
  let n = 0
  for (const ch of letters) n = n * 26 + ch.charCodeAt(0) - 64
  return n
}
function cellValue(xml: string, shared: string[], type: string, value: string, inline: string): string {
  if (type === 's') return shared[Number(value)] ?? ''
  if (type === 'inlineStr') return inline
  if (type === 'b') return value === '1' ? 'TRUE' : 'FALSE'
  return value
}

async function parseXlsx(file: File): Promise<{ sheets: string[]; rowsBySheet: Record<string, SpreadsheetRow[]> }> {
  const zip = await readZip(await file.arrayBuffer())
  const workbook = xmlText(zip.get('xl/workbook.xml') ?? new Uint8Array())
  const rels = xmlText(zip.get('xl/_rels/workbook.xml.rels') ?? new Uint8Array())
  const sharedXml = xmlText(zip.get('xl/sharedStrings.xml') ?? new Uint8Array())
  const shared = [...sharedXml.matchAll(/<si[\s\S]*?<t[^>]*>([\s\S]*?)<\/t>[\s\S]*?<\/si>/g)].map(m => esc(m[1]))
  const relationMap: Record<string, string> = {}
  for (const m of rels.matchAll(/<Relationship\b[^>]*Id="([^"]+)"[^>]*Target="([^"]+)"[^>]*\/>/g)) {
    relationMap[m[1]] = m[2].replace(/^\//, '')
  }
  const sheets: string[] = []
  const rowsBySheet: Record<string, SpreadsheetRow[]> = {}
  for (const m of workbook.matchAll(/<sheet\b([^>]*)\/>/g)) {
    const tag = m[1]
    const name = attr(tag, 'name')
    const rid = attr(tag, 'r:id')
    let target = relationMap[rid] ?? ''
    if (target && !target.startsWith('xl/')) target = 'xl/' + target
    if (!target) continue
    const sheetXml = xmlText(zip.get(target) ?? new Uint8Array())
    const rawRows: Record<number, string>[] = []
    for (const rowMatch of sheetXml.matchAll(/<row\b[^>]*>([\s\S]*?)<\/row>/g)) {
      const row: Record<number, string> = {}
      for (const c of rowMatch[1].matchAll(/<c\b([^>]*)>([\s\S]*?)<\/c>/g)) {
        const tagText = c[1]
        const body = c[2]
        const ref = attr(tagText, 'r')
        const col = columnNumber(ref)
        const type = attr(tagText, 't')
        const value = (body.match(/<v>([\s\S]*?)<\/v>/)?.[1] ?? '').trim()
        const inline = (body.match(/<t[^>]*>([\s\S]*?)<\/t>/)?.[1] ?? '').trim()
        row[col] = cellValue(sheetXml, shared, type, value, inline)
      }
      rawRows.push(row)
    }
    const header = rawRows.shift() ?? {}
    const headers = Object.keys(header).map(Number).sort((a, b) => a - b).map(n => header[n].trim() || 'Column ' + n)
    rowsBySheet[name] = rawRows.map(row => {
      const out: SpreadsheetRow = {}
      headers.forEach((key, index) => { out[key] = row[index + 1] ?? '' })
      return out
    }).filter(row => Object.values(row).some(value => value.trim() !== ''))
    sheets.push(name)
  }
  return { sheets, rowsBySheet }
}

function parseDelimited(text: string): { sheets: string[]; rowsBySheet: Record<string, SpreadsheetRow[]> } {
  const lines = text.replace(/^\uFEFF/, '').split(/\r?\n/).filter(line => line.trim())
  if (!lines.length) return { sheets: ['Sheet1'], rowsBySheet: { Sheet1: [] } }
  const delimiter = lines[0].includes('\t') ? '\t' : ','
  const parseLine = (line: string) => {
    const out: string[] = []
    let current = '', quoted = false
    for (let i = 0; i < line.length; i++) {
      const ch = line[i]
      if (ch === '"') {
        if (quoted && line[i + 1] === '"') { current += '"'; i++ } else quoted = !quoted
      } else if (ch === delimiter && !quoted) { out.push(current); current = '' } else current += ch
    }
    out.push(current)
    return out
  }
  const headers = parseLine(lines[0]).map((x, i) => x.trim() || 'Column ' + (i + 1))
  const rows = lines.slice(1).map(line => {
    const values = parseLine(line)
    const row: SpreadsheetRow = {}
    headers.forEach((key, i) => { row[key] = (values[i] ?? '').trim() })
    return row
  }).filter(row => Object.values(row).some(value => value))
  return { sheets: ['Sheet1'], rowsBySheet: { Sheet1: rows } }
}

export async function parseSpreadsheet(file: File) {
  const name = file.name.toLowerCase()
  if (name.endsWith('.csv') || name.endsWith('.tsv')) return parseDelimited(await file.text())
  if (name.endsWith('.xlsx')) return parseXlsx(file)
  throw new Error('Supported spreadsheet formats: .xlsx, .csv, .tsv')
}
