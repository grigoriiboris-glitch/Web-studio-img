<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  generationCount: number
  selectedVariantCount: number
  hasReferences: boolean
  hasFinalCandidate: boolean
  finalApproved: boolean
}>()

const steps = computed(() => [
  { id: 'brief', label: 'Brief', done: true, hint: 'Задача и процесс' },
  { id: 'references', label: 'References', done: props.hasReferences, hint: 'Скетч и референсы' },
  { id: 'generate', label: 'Generate', done: props.generationCount > 0, hint: props.generationCount ? `${props.generationCount} results` : 'Создай варианты' },
  { id: 'variants', label: 'Variants', done: props.generationCount > 0, hint: 'Сравнение результатов' },
  { id: 'select', label: 'Select', done: props.selectedVariantCount > 0, hint: props.selectedVariantCount ? `${props.selectedVariantCount} selected` : 'Выбери результат' },
  { id: 'finalize', label: 'Finalize', done: props.finalApproved, hint: props.finalApproved ? 'Финал утверждён' : props.hasFinalCandidate ? 'Кандидат готов к утверждению' : 'Нужен выбранный кандидат' },
])

const currentIndex = computed(() => {
  const firstIncomplete = steps.value.findIndex(step => !step.done)
  return firstIncomplete === -1 ? steps.value.length - 1 : firstIncomplete
})

function scrollTo(id: string) {
  document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
</script>

<template>
  <nav
    class="workflow-rail"
    aria-label="Creative workflow"
  >
    <div class="workflow-heading">
      <div>
        <span class="eyebrow">Creative workflow</span>
        <strong>От идеи до финального результата</strong>
      </div>
      <span class="progress">{{ Math.round((steps.filter(step => step.done).length / steps.length) * 100) }}%</span>
    </div>
    <div class="workflow-steps">
      <button
        v-for="(step, index) in steps"
        :key="step.id"
        type="button"
        class="workflow-step"
        :class="{ done: step.done, current: index === currentIndex }"
        :aria-current="index === currentIndex ? 'step' : undefined"
        @click="scrollTo(step.id)"
      >
        <span class="dot">{{ step.done ? '✓' : index + 1 }}</span>
        <span class="copy">
          <strong>{{ step.label }}</strong>
          <small>{{ step.hint }}</small>
        </span>
      </button>
    </div>
  </nav>
</template>

<style scoped>
.workflow-rail {
  position: sticky;
  top: 8px;
  z-index: 5;
  display: grid;
  gap: 12px;
  margin-bottom: 16px;
  padding: 14px;
  border: 1px solid var(--el-border-color);
  border-radius: 14px;
  background: color-mix(in srgb, var(--el-bg-color) 94%, var(--el-color-primary) 6%);
  box-shadow: var(--el-box-shadow-light);
}
.workflow-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.workflow-heading > div { display: grid; gap: 3px; }
.eyebrow {
  color: var(--el-color-primary);
  font-size: 11px;
  font-weight: 700;
  letter-spacing: .08em;
  text-transform: uppercase;
}
.progress { color: var(--el-text-color-secondary); font-size: 12px; font-weight: 700; }
.workflow-steps {
  display: grid;
  grid-template-columns: repeat(6, minmax(0, 1fr));
  gap: 6px;
}
.workflow-step {
  display: grid;
  grid-template-columns: 28px minmax(0, 1fr);
  gap: 8px;
  align-items: center;
  min-width: 0;
  padding: 8px;
  border: 1px solid transparent;
  border-radius: 10px;
  background: transparent;
  color: inherit;
  text-align: left;
  cursor: pointer;
}
.workflow-step:hover,
.workflow-step.current { border-color: var(--el-color-primary-light-5); background: var(--el-fill-color-light); }
.workflow-step.done .dot { background: var(--el-color-success); color: var(--el-color-white); }
.dot {
  display: grid;
  width: 28px;
  height: 28px;
  place-items: center;
  border-radius: 50%;
  background: var(--el-fill-color);
  color: var(--el-text-color-secondary);
  font-size: 12px;
  font-weight: 700;
}
.copy { display: grid; min-width: 0; gap: 2px; }
.copy strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.copy small { overflow: hidden; color: var(--el-text-color-secondary); text-overflow: ellipsis; white-space: nowrap; }
@media (max-width: 900px) {
  .workflow-steps { grid-template-columns: repeat(3, minmax(0, 1fr)); }
}
@media (max-width: 600px) {
  .workflow-rail { position: relative; top: 0; }
  .workflow-steps { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
