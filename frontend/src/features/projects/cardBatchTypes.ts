export type CardTypeKey = 'role' | 'item' | 'world' | 'event' | 'faction' | 'resource' | 'custom'
export type CardFieldType = 'text' | 'number' | 'select' | 'textarea'

export interface CardFieldSchema {
  key: string
  label: string
  type: CardFieldType
  required?: boolean
  options?: string[]
  defaultValue?: string
  aliases?: string[]
  min?: number
  max?: number
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
}

export interface CardTemplateDefinition {
  id: string
  name: string
  description: string
  typeKeys: CardTypeKey[]
}

export const CARD_TEMPLATES: CardTemplateDefinition[] = [
  { id: 'standard-role', name: 'Standard Character', description: 'Standard character composition.', typeKeys: ['role'] },
  { id: 'standard-item', name: 'Standard Item', description: 'Standard centered object composition.', typeKeys: ['item', 'resource'] },
  { id: 'standard-world', name: 'Standard Location', description: 'Environment-focused composition.', typeKeys: ['world'] },
  { id: 'standard-event', name: 'Standard Event', description: 'Narrative scene composition.', typeKeys: ['event'] },
  { id: 'standard-faction', name: 'Standard Faction', description: 'Faction identity composition.', typeKeys: ['faction'] },
  { id: 'custom', name: 'Custom Template', description: 'No type-specific template.', typeKeys: ['custom'] },
]

export const CARD_TYPES: CardTypeDefinition[] = [
  {
    key: 'role', name: 'Role / Character', version: 1,
    description: 'Персонажи, роли, классы и способности.',
    fields: [
      { key: 'name', label: 'Name', type: 'text', required: true, aliases: ['card name', 'название', 'имя'] },
      { key: 'description', label: 'Description', type: 'textarea', required: true, aliases: ['desc', 'описание'] },
      { key: 'prompt', label: 'Prompt', type: 'textarea', required: true, aliases: ['prompt', 'промт', 'image prompt'] },
    ],
    promptInstructions: 'Board-game character illustration. Make the character identity, silhouette and readable role cues clear.',
    recipeKeywords: ['character', 'role', 'персонаж'],
    defaultTemplateId: 'standard-role',
  },
  {
    key: 'item', name: 'Item / Equipment', version: 1,
    description: 'Предметы, оружие, экипировка и артефакты.',
    fields: [
      { key: 'name', label: 'Name', type: 'text', required: true, aliases: ['card name', 'название', 'имя'] },
      { key: 'rarity', label: 'Rarity', type: 'select', options: ['common', 'uncommon', 'rare', 'epic', 'legendary'], aliases: ['редкость', 'уровень'] },
      { key: 'description', label: 'Description', type: 'textarea', required: true, aliases: ['desc', 'описание'] },
      { key: 'prompt', label: 'Prompt', type: 'textarea', required: true, aliases: ['промт', 'image prompt'] },
    ],
    promptInstructions: 'Board-game item illustration. Keep the object as the unmistakable visual subject with a readable silhouette.',
    recipeKeywords: ['item', 'equipment', 'object', 'предмет'],
    defaultTemplateId: 'standard-item',
  },
  {
    key: 'world', name: 'World / Location', version: 1,
    description: 'Локации, места и окружение.',
    fields: [
      { key: 'name', label: 'Name', type: 'text', required: true, aliases: ['card name', 'название', 'имя'] },
      { key: 'region', label: 'Region', type: 'text', aliases: ['район', 'регион', 'местность'] },
      { key: 'description', label: 'Description', type: 'textarea', required: true, aliases: ['desc', 'описание'] },
      { key: 'prompt', label: 'Prompt', type: 'textarea', required: true, aliases: ['промт', 'image prompt'] },
    ],
    promptInstructions: 'Board-game location illustration. Establish a clear place, spatial depth and environmental storytelling.',
    recipeKeywords: ['world', 'location', 'environment', 'landscape', 'локация'],
    defaultTemplateId: 'standard-world',
  },
  {
    key: 'event', name: 'Event', version: 1,
    description: 'События, происшествия и условия раунда.',
    fields: [
      { key: 'name', label: 'Name', type: 'text', required: true, aliases: ['card name', 'название', 'имя'] },
      { key: 'description', label: 'Description', type: 'textarea', required: true, aliases: ['desc', 'описание'] },
      { key: 'effect', label: 'Effect', type: 'textarea', aliases: ['эффект', 'effect text'] },
      { key: 'prompt', label: 'Prompt', type: 'textarea', required: true, aliases: ['промт', 'image prompt'] },
    ],
    promptInstructions: 'Board-game event illustration. Show the event as a readable narrative moment rather than an abstract symbol.',
    recipeKeywords: ['event', 'scene', 'событие'],
    defaultTemplateId: 'standard-event',
  },
  {
    key: 'faction', name: 'Faction', version: 1,
    description: 'Фракции, группы и стороны.',
    fields: [
      { key: 'name', label: 'Name', type: 'text', required: true, aliases: ['card name', 'название', 'имя'] },
      { key: 'description', label: 'Description', type: 'textarea', required: true, aliases: ['desc', 'описание'] },
      { key: 'ability', label: 'Ability', type: 'textarea', aliases: ['способность', 'ability text'] },
      { key: 'prompt', label: 'Prompt', type: 'textarea', required: true, aliases: ['промт', 'image prompt'] },
    ],
    promptInstructions: 'Board-game faction illustration. Make faction identity and visual symbols coherent and immediately recognizable.',
    recipeKeywords: ['faction', 'group', 'фракция'],
    defaultTemplateId: 'standard-faction',
  },
  {
    key: 'resource', name: 'Resource', version: 1,
    description: 'Ресурсы, жетоны и игровые материалы.',
    fields: [
      { key: 'name', label: 'Name', type: 'text', required: true, aliases: ['card name', 'название', 'имя'] },
      { key: 'rarity', label: 'Rarity', type: 'select', options: ['common', 'uncommon', 'rare', 'epic', 'legendary'], aliases: ['редкость', 'уровень'] },
      { key: 'description', label: 'Description', type: 'textarea', required: true, aliases: ['desc', 'описание'] },
      { key: 'prompt', label: 'Prompt', type: 'textarea', required: true, aliases: ['промт', 'image prompt'] },
    ],
    promptInstructions: 'Board-game resource illustration. Keep the resource visually distinct and easy to identify at card size.',
    recipeKeywords: ['resource', 'token', 'material', 'ресурс'],
    defaultTemplateId: 'standard-item',
  },
  {
    key: 'custom', name: 'Custom', version: 1,
    description: 'Произвольный тип карточек.',
    fields: [
      { key: 'name', label: 'Name', type: 'text', aliases: ['card name', 'название', 'имя'] },
      { key: 'description', label: 'Description', type: 'textarea', aliases: ['desc', 'описание'] },
      { key: 'prompt', label: 'Prompt', type: 'textarea', required: true, aliases: ['промт', 'image prompt'] },
    ],
    promptInstructions: 'Board-game card illustration with a clear central subject and readable silhouette.',
    recipeKeywords: [],
    defaultTemplateId: 'custom',
  },
]

export const REJECT_REASONS_BY_TYPE: Record<CardTypeKey, string[]> = {
  role: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_detail', 'technical', 'other'],
  item: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_color', 'wrong_detail', 'technical', 'other'],
  world: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_color', 'wrong_detail', 'technical', 'other'],
  event: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_color', 'wrong_detail', 'technical', 'other'],
  faction: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_detail', 'technical', 'other'],
  resource: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_color', 'wrong_detail', 'technical', 'other'],
  custom: ['wrong_composition', 'wrong_style', 'wrong_subject', 'wrong_color', 'wrong_detail', 'technical', 'other'],
}

export function cardTypeDefinition(key?: CardTypeKey): CardTypeDefinition {
  return CARD_TYPES.find(type => type.key === key) || CARD_TYPES.find(type => type.key === 'custom')!
}

export function templateForType(key: CardTypeKey, templateId?: string): CardTemplateDefinition {
  if (templateId) {
    const exact = CARD_TEMPLATES.find(template => template.id === templateId)
    if (exact) return exact
  }
  return CARD_TEMPLATES.find(template => template.id === cardTypeDefinition(key).defaultTemplateId) || CARD_TEMPLATES[CARD_TEMPLATES.length - 1]
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
  const result: Record<string, string> = {}
  for (const field of cardTypeDefinition(type).fields) {
    if (field.key === 'prompt') continue
    const column = mapping?.[field.key] || suggestColumnMapping(Object.keys(row), type)[field.key]
    const value = column ? String(row[column] ?? '').trim() : ''
    if (value) result[field.key] = value
  }
  return result
}

export interface CardValidation {
  field: string
  message: string
}

export function validateCardData(cardNumber: string, prompt: string, fields: Record<string, string> | undefined, type: CardTypeKey): CardValidation[] {
  const definition = cardTypeDefinition(type)
  const values = fields || {}
  const errors: CardValidation[] = []
  if (!cardNumber.trim()) errors.push({ field: 'cardNumber', message: 'Card number is required.' })
  for (const field of definition.fields) {
    const value = field.key === 'prompt' ? prompt : field.key === 'name' ? values.name || '' : values[field.key] || ''
    if (field.required && !String(value).trim()) errors.push({ field: field.key, message: field.label + ' is required.' })
    if (field.type === 'select' && value && field.options && !field.options.includes(String(value).trim().toLowerCase())) {
      errors.push({ field: field.key, message: field.label + ' must be one of: ' + field.options.join(', ') })
    }
    if (field.type === 'number' && value) {
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

export function assembleCardPrompt(prompt: string, type: CardTypeKey, fields: Record<string, string> | undefined): string {
  const definition = cardTypeDefinition(type)
  const context = definition.fields
    .filter(field => field.key !== 'prompt' && fields?.[field.key])
    .map(field => field.label + ': ' + fields![field.key])
  const sections = [prompt.trim()]
  if (definition.promptInstructions) sections.push('Type direction: ' + definition.promptInstructions)
  if (context.length) sections.push('Card data:\n' + context.join('\n'))
  return sections.filter(Boolean).join('\n\n')
}

export function cardTypeSchemaVersion(type: CardTypeKey): string {
  return cardTypeDefinition(type).key + '@' + cardTypeDefinition(type).version
}
