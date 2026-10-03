import { describe, expect, it } from 'vitest'
import {
  CARD_TYPES,
  cardTypeDefinition,
  ensureCardTypeDefinition,
  assembleCardPrompt,
  productionDefaults,
  fieldsFromRow,
  resolveDefaultRecipeId,
  suggestColumnMapping,
  validateCardData,
} from './cardBatchTypes'

describe('card batch type schemas', () => {
  it('maps multilingual spreadsheet headers using aliases', () => {
    const mapping = suggestColumnMapping(['Номер', 'Промт', 'Редкость', 'Описание'], 'item')
    expect(mapping).toMatchObject({
      rarity: 'Редкость',
      description: 'Описание',
      prompt: 'Промт',
    })
  })

  it('extracts typed fields without losing the stable prompt column', () => {
    const mapping = { name: 'Название', rarity: 'Редкость', description: 'Описание', prompt: 'Промт' }
    const fields = fieldsFromRow({
      Название: 'Knife',
      Редкость: 'rare',
      Описание: 'A field knife',
      Промт: 'A survival knife',
    }, 'item', mapping)
    expect(fields).toEqual({ name: 'Knife', rarity: 'rare', description: 'A field knife' })
  })

  it('rejects missing required typed fields and invalid enum values', () => {
    const errors = validateCardData('001', 'prompt', { name: 'Knife', rarity: 'mythic' }, 'item')
    expect(errors.map(item => item.field)).toContain('description')
    expect(errors.map(item => item.field)).toContain('rarity')
  })

  it('does not silently choose an unrelated recipe', () => {
    const recipes = [
      { id: '1', name: 'Character Paint' },
      { id: '2', name: 'Item Clean' },
    ]
    expect(resolveDefaultRecipeId(recipes, 'item')).toBe('2')
    expect(resolveDefaultRecipeId(recipes, 'world')).toBeUndefined()
  })

  it('keeps the original prompt while adding type context', () => {
    const result = assembleCardPrompt('A knife on a table', 'item', { name: 'Knife', rarity: 'rare' })
    expect(result).toContain('A knife on a table')
    expect(result).toContain('Type direction:')
    expect(result).toContain('Rarity: rare')
  })

  it('requires only the stable prompt for backward-compatible imports', () => {
    const errors = validateCardData('001', 'prompt', {}, 'item')
    expect(errors).toHaveLength(0)
  })



  it('supports a completely custom card type without a predefined field type', () => {
    const definition = ensureCardTypeDefinition('Spell', ['Номер', 'Название', 'Школа', 'Стоимость', 'Промт'])
    expect(definition.key).toBe('Spell')
    expect(definition.fields.map(field => field.key)).toEqual(['Название', 'Школа', 'Стоимость'])
    expect(definition.fields.every(field => field.type === undefined)).toBe(true)

    const fields = fieldsFromRow({
      Номер: '001',
      Название: 'Fireball',
      Школа: 'Fire',
      Стоимость: '3',
      Промт: 'A fire spell',
    }, 'Spell')
    expect(fields).toEqual({
      Название: 'Fireball',
      Школа: 'Fire',
      Стоимость: '3',
    })
  })

  it('does not crash for an unknown type id', () => {
    const definition = cardTypeDefinition('NPC')
    expect(definition.key).toBe('NPC')
    expect(definition.fields).toEqual([])
    expect(validateCardData('001', 'prompt', { faction: 'neutral' }, 'NPC')).toEqual([])
  })

  it('accepts arbitrary field kinds as UI metadata without blocking card validation', () => {
    const definition = cardTypeDefinition('item')
    definition.fields.push({ key: 'custom_data', label: 'Custom data', type: 'some_future_widget' })
    expect(validateCardData('001', 'prompt', { custom_data: 'anything' }, 'item')).toEqual([])
    definition.fields.pop()
  })

  it('resolves type production dimensions without forcing them onto custom cards', () => {
    expect(productionDefaults('role')).toMatchObject({ width: 750, height: 1050, aspectRatio: '5:7' })
    expect(productionDefaults('world')).toMatchObject({ width: 1200, height: 800, aspectRatio: '3:2' })
    expect(productionDefaults('custom').width).toBeUndefined()
  })

  it('defines a versioned schema for every production card type', () => {
    for (const type of CARD_TYPES) {
      expect(type.version).toBeGreaterThan(0)
      expect(type.fields.some(field => field.key === 'prompt' && field.required)).toBe(true)
    }
  })
})
