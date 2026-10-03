export type CardTypeKey = string

/**
 * Field metadata is intentionally optional. Card data is JSON-like and may contain
 * arbitrary columns from a spreadsheet. Legacy field types remain supported only
 * as UI/validation hints for existing schemas.
 */
export interface CardFieldSchema {
  key: string
  label: string
  type?: string
  required?: boolean
  options?: string[]
  defaultValue?: unknown
  aliases?: string[]
  min?: number
  max?: number
  includeInPrompt?: boolean
}

export interface CardTypeDefinition {
  key: CardTypeKey
  name: string
  description: string
  version: number
  fields: CardFieldSchema[]
  promptInstructions: string
  recipeKeywords: string[]
  defaultTemplateId: string
  defaultWidth?: number
  defaultHeight?: number
  defaultAspectRatio?: string
  defaultNegativePrompt?: string
  defaultGenerationParameters?: Record<string, unknown>
  defaultReferenceIds?: string[]
}

export interface CardTemplateDefinition {
  id: string
  name: string
  description: string
  typeKeys?: string[]
}

export const CARD_TEMPLATES: CardTemplateDefinition[] = [
  { id: 'standard-role', name: 'Standard Character', description: 'Standard character composition.', typeKeys: ['role'] },
  { id: 'standard-item', name: 'Standard Item', description: 'Standard centered object composition.', typeKeys: ['item', 'resource'] },
  { id: 'standard-world', name: 'Standard Location', description: 'Environment-focused composition.', typeKeys: ['world'] },
  { id: 'standard-event', name: 'Standard Event', description: 'Narrative scene composition.', typeKeys: ['event'] },
  { id: 'standard-faction', name: 'Standard Faction', description: 'Faction identity composition.', typeKeys: ['faction'] },
  { id: 'custom', name: 'Custom Template', description: 'No type-specific template.' },
]

const legacy = (key: string, name: string, description: string, fields: CardFieldSchema[], promptInstructions: string, recipeKeywords: string[], template: string, width?: number, height?: number, aspectRatio?: string): CardTypeDefinition => ({
  key, name, description, version: 1, fields, promptInstructions, recipeKeywords, defaultTemplateId: template,
  defaultWidth: width, defaultHeight: height, defaultAspectRatio: aspectRatio,
})

const commonName: CardFieldSchema = { key: 'name', label: 'Name', aliases: ['card name', 'название', 'имя'] }
const commonDescription: CardFieldSchema = { key: 'description', label: 'Description', aliases: ['desc', 'описание'] }
const commonPrompt: CardFieldSchema = { key: 'prompt', label: 'Prompt', required: true, aliases: ['prompt', 'промт', 'image prompt'], includeInPrompt: false }

export const CARD_TYPES: CardTypeDefinition[] = [
  legacy('role', 'Role / Character', 'Персонажи, роли, классы и способности.', [commonName, { ...commonDescription }, { ...commonPrompt }], 'Board-game character illustration. Make the character identity, silhouette and readable role cues clear.', ['character', 'role', 'персонаж'], 'standard-role', 750, 1050, '5:7'),
  legacy('item', 'Item / Equipment', 'Предметы, оружие, экипировка и артефакты.', [commonName, { key: 'rarity', label: 'Rarity', aliases: ['редкость', 'уровень'], options: ['common', 'uncommon', 'rare', 'epic', 'legendary'] }, { ...commonDescription }, { ...commonPrompt }], 'Board-game item illustration. Keep the object as the unmistakable visual subject with a readable silhouette.', ['item', 'equipment', 'object', 'предмет'], 'standard-item', 750, 1050, '5:7'),
  legacy('world', 'World / Location', 'Локации, места и окружение.', [commonName, { key: 'region', label: 'Region', aliases: ['район', 'регион', 'местность'] }, { ...commonDescription }, { ...commonPrompt }], 'Board-game location illustration. Establish a clear place, spatial depth and environmental storytelling.', ['world', 'location', 'environment', 'landscape', 'локация'], 'standard-world', 1200, 800, '3:2'),
  legacy('event', 'Event', 'События, происшествия и условия раунда.', [commonName, { ...commonDescription }, { key: 'effect', label: 'Effect', aliases: ['эффект', 'effect text'] }, { ...commonPrompt }], 'Board-game event illustration. Show the event as a readable narrative moment rather than an abstract symbol.', ['event', 'scene', 'событие'], 'standard-event', 1200, 800, '3:2'),
  legacy('faction', 'Faction', 'Фракции, группы и стороны.', [commonName, { ...commonDescription }, { key: 'ability', label: 'Ability', aliases: ['способность', 'ability text'] }, { ...commonPrompt }], 'Board-game faction illustration. Make faction identity and visual symbols coherent and immediately recognizable.', ['faction', 'group', 'фракция'], 'standard-faction', 750, 1050, '5:7'),
  legacy('resource', 'Resource', 'Ресурсы, жетоны и игровые материалы.', [commonName, { key: 'rarity', label: 'Rarity', aliases: ['редкость', 'уровень'], options: ['common', 'uncommon', 'rare', 'epic', 'legendary'] }, { ...commonDescription }, { ...commonPrompt }], 'Board-game resource illustration. Keep the resource visually distinct and easy to identify at card size.', ['resource', 'token', 'material', 'ресурс'], 'standard-item', 750, 1050, '5:7'),
  legacy('custom', 'Custom', 'Произвольный тип карточек.', [commonName, { ...commonDescription }, { ...commonPrompt }], 'Board-game card illustration with a clear central subject and readable silhouette.', [], 'custom'),
]

const runtimeDefinitions = new Map<string, CardTypeDefinition>()
for (const definition of CARD_TYPES) runtimeDefinitions.set(definition.key, definition)

export function registerCardTypeDefinition(definition: CardTypeDefinition): void {
  if (!definition.key.trim()) return
  runtimeDefinitions.set(definition.key, {
    ...definition,
    fields: (definition.fields || []).map(field => ({ ...field })),
  })
}

export function registerCardTypeDefinitions(definitions: CardTypeDefinition[]): void {
  definitions.forEach(registerCardTypeDefinition)
}

export function cardTypeDefinitions(): CardTypeDefinition[] {
  return [...runtimeDefinitions.values()]
}

export function cardTypeDefinition(key?: CardTypeKey): CardTypeDefinition {
  const normalized = String(key || 'custom').trim() || 'custom'
  return runtimeDefinitions.get(normalized) || {
    key: normalized,
    name: normalized,
    description: 'Custom card type. Fields are data columns; no field type is required.',
    version: 1,
    fields: [],
    promptInstructions: 'Board-game card illustration with a clear central subject and readable silhouette.',
    recipeKeywords: [],
    defaultTemplateId: 'custom',
  }
}

export function ensureCardTypeDefinition(key: string, columns: string[] = []): CardTypeDefinition {
  const existing = runtimeDefinitions.get(key)
  if (existing) return existing
  const excluded = new Set(['card_number', 'card number', 'number', 'prompt', 'image prompt', 'промт', 'номер'])
  const fields = columns
    .filter(column => !excluded.has(column.trim().toLowerCase()))
    .map(column => ({
      key: column.trim(),
      label: column.trim(),
      aliases: [column.trim()],
    }))
  const definition: CardTypeDefinition = {
    key,
    name: key,
    description: 'Custom card type created from spreadsheet columns.',
    version: 1,
    fields,
    promptInstructions: 'Board-game card illustration with a clear central subject and readable silhouette.',
    recipeKeywords: [],
    defaultTemplateId: 'custom',
  }
  registerCardTypeDefinition(definition)
  return definition
}

export const REJECT_REASONS_BY_TYPE: Record<string, string[]> = {
  role: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_detail', 'technical', 'other'],
  item: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_color', 'wrong_detail', 'technical', 'other'],
  world: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_color', 'wrong_detail', 'technical', 'other'],
  event: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_color', 'wrong_detail', 'technical', 'other'],
  faction: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_detail', 'technical', 'other'],
  resource: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_color', 'wrong_detail', 'technical', 'other'],
  custom: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_color', 'wrong_detail', 'technical', 'other'],
}

export function rejectReasonsForType(type: string): string[] {
  return REJECT_REASONS_BY_TYPE[type] || REJECT_REASONS_BY_TYPE.custom
}

export function templateForType(key: CardTypeKey, templateId?: string): CardTemplateDefinition {
  const definition = cardTypeDefinition(key)
  if (templateId) {
    const exact = CARD_TEMPLATES.find(template => template.id === templateId)
    if (exact && (!exact.typeKeys?.length || exact.typeKeys.includes(key) || exact.id === 'custom')) return exact
  }
  return CARD_TEMPLATES.find(template => template.id === definition.defaultTemplateId) || CARD_TEMPLATES[CARD_TEMPLATES.length - 1]
}

function normalizeHeader(value: string): string {
  return value.trim().toLowerCase().replace(/[._-]+/g, ' ').replace(/\s+/g, ' ')
}

export function suggestColumnMapping(columns: string[], type: CardTypeKey): Record<string, string> {
  const definition = cardTypeDefinition(type)
  const used = new Set<string>()
  const mapping: Record<string, string> = {}
  const score = (column: string, field: CardFieldSchema) => {
    const normalized = normalizeHeader(column)
    const aliases = [field.key, field.label, ...(field.aliases || [])].map(normalizeHeader)
    if (aliases.includes(normalized)) return 100
    const compact = normalized.replace(/\s+/g, '')
    if (aliases.some(alias => compact === alias.replace(/\s+/g, ''))) return 90
    if (aliases.some(alias => normalized.includes(alias) || alias.includes(normalized))) return 60
    return 0
  }
  for (const field of definition.fields) {
    let best = ''
    let bestScore = 0
    for (const column of columns) {
      if (used.has(column)) continue
      const current = score(column, field)
      if (current > bestScore) { best = column; bestScore = current }
    }
    if (best) { mapping[field.key] = best; used.add(best) }
  }
  return mapping
}

export function fieldsFromRow(row: Record<string, string>, type: CardTypeKey, mapping?: Record<string, string>): Record<string, string> {
  const definition = cardTypeDefinition(type)
  const result: Record<string, string> = {}
  const effectiveMapping = mapping || suggestColumnMapping(Object.keys(row), type)
  const mappedColumns = new Set(Object.values(effectiveMapping))
  for (const field of definition.fields) {
    if (field.key === 'prompt') continue
    const column = effectiveMapping[field.key] || (Object.prototype.hasOwnProperty.call(row, field.key) ? field.key : undefined)
    const value = column ? String(row[column] ?? '').trim() : ''
    if (value) result[field.key] = value
  }
  // Dynamic types may have no schema fields yet. Preserve every unmapped spreadsheet
  // column as card data rather than silently dropping it.
  if (!definition.fields.length) {
    for (const [column, value] of Object.entries(row)) {
      if (mappedColumns.has(column)) continue
      const normalized = normalizeHeader(column)
      if (['prompt', 'image prompt', 'промт', 'card number', 'card_number', 'number', 'номер'].includes(normalized)) continue
      const text = String(value ?? '').trim()
      if (text) result[column] = text
    }
  }
  return result
}

export interface CardValidation {
  field: string
  message: string
}

export function validateCardData(cardNumber: string, prompt: string, fields: Record<string, unknown> | undefined, type: CardTypeKey): CardValidation[] {
  const definition = cardTypeDefinition(type)
  const values = fields || {}
  const errors: CardValidation[] = []
  if (!cardNumber.trim()) errors.push({ field: 'cardNumber', message: 'Card number is required.' })
  for (const field of definition.fields) {
    const value = field.key === 'prompt' ? prompt : values[field.key]
    if (field.required && !String(value ?? '').trim()) errors.push({ field: field.key, message: field.label + ' is required.' })
    // Validation is opt-in legacy metadata. Arbitrary field kinds are accepted.
    if (field.type === 'select' && value && field.options && !field.options.includes(String(value).trim().toLowerCase())) {
      errors.push({ field: field.key, message: field.label + ' must be one of: ' + field.options.join(', ') })
    }
    if (field.type === 'number' && value !== undefined && value !== null && String(value) !== '') {
      const n = Number(value)
      if (!Number.isFinite(n) || (field.min !== undefined && n < field.min) || (field.max !== undefined && n > field.max)) {
        errors.push({ field: field.key, message: field.label + ' is outside its allowed range.' })
      }
    }
  }
  return errors
}

export function resolveDefaultRecipeId(recipes: Array<{ id: string; name: string }>, type: CardTypeKey): string | undefined {
  const keywords = cardTypeDefinition(type).recipeKeywords
  if (!keywords.length) return undefined
  const normalized = (value: string) => normalizeHeader(value)
  const scored = recipes.map(recipe => {
    const name = normalized(recipe.name)
    const score = Math.max(...keywords.map(keyword => {
      const key = normalized(keyword)
      if (name === key) return 100
      if (name.startsWith(key + ' ')) return 80
      if (name.includes(key)) return 60
      return 0
    }))
    return { recipe, score }
  }).filter(item => item.score > 0).sort((a, b) => b.score - a.score)
  if (!scored.length) return undefined
  if (scored.length > 1 && scored[0].score === scored[1].score) return undefined
  return scored[0].recipe.id
}

export function assembleCardPrompt(prompt: string, type: CardTypeKey, fields: Record<string, unknown> | undefined): string {
  const definition = cardTypeDefinition(type)
  const context = definition.fields
    .filter(field => field.key !== 'prompt' && field.includeInPrompt !== false && fields?.[field.key] !== undefined && String(fields[field.key]).trim())
    .map(field => field.label + ': ' + String(fields![field.key]))
  // Dynamic/untyped fields are still useful prompt context.
  if (!definition.fields.length && fields) {
    for (const [key, value] of Object.entries(fields)) {
      if (value !== undefined && value !== null && String(value).trim()) context.push(key + ': ' + String(value))
    }
  }
  const sections = [prompt.trim()]
  if (definition.promptInstructions) sections.push('Type direction: ' + definition.promptInstructions)
  if (context.length) sections.push('Card data:\n' + context.join('\n'))
  return sections.filter(Boolean).join('\n\n')
}

export function productionDefaults(type: CardTypeKey) {
  const definition = cardTypeDefinition(type)
  return {
    width: definition.defaultWidth,
    height: definition.defaultHeight,
    aspectRatio: definition.defaultAspectRatio,
    negativePrompt: definition.defaultNegativePrompt,
    generationParameters: definition.defaultGenerationParameters || {},
    referenceIds: definition.defaultReferenceIds || [],
  }
}

export function cardTypeSchemaVersion(type: CardTypeKey): string {
  const definition = cardTypeDefinition(type)
  return definition.key + '@' + definition.version
}
