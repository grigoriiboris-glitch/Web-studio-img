
<!-- eslint-disable vue/max-attributes-per-line, vue/singleline-html-element-content-newline -->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { RouterLink, useRoute } from 'vue-router'
import { visualDnaApi, type VisualDNASource, type VisualDNAProfile, type VisualDNAComparison } from '../../api/creative'

const route = useRoute()
const projectId = () => String(route.params.projectId)

const loading = ref(false)
const analyzing = ref(false)
const sources = ref<VisualDNASource[]>([])
const selectedAssetIds = ref<string[]>([])
const profiles = ref<VisualDNAProfile[]>([])
const selectedProfile = ref<VisualDNAProfile | null>(null)
const compareAssetId = ref('')
const comparison = ref<VisualDNAComparison | null>(null)

const selectedCountLabel = computed(() =>
  selectedAssetIds.value.length === 0 ? 'All allowed assets' : selectedAssetIds.value.length + ' selected',
)

async function load() {
  loading.value = true
  try {
    const [sourceResult, profileResult] = await Promise.all([
      visualDnaApi.sources(projectId()),
      visualDnaApi.list(projectId()),
    ])
    sources.value = sourceResult.sources
    profiles.value = profileResult.profiles
    if (profiles.value.length > 0) {
      selectedProfile.value = profiles.value[0]
      compareAssetId.value = sources.value[0]?.id ?? ''
    }
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not load Visual DNA')
  } finally {
    loading.value = false
  }
}

async function analyze() {
  analyzing.value = true
  try {
    const created = await visualDnaApi.analyze(
      projectId(),
      selectedAssetIds.value.length > 0 ? { asset_ids: selectedAssetIds.value } : {},
    )
    profiles.value = [created, ...profiles.value]
    selectedProfile.value = created
    ElMessage.success('Visual DNA profile created')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not analyze Visual DNA')
  } finally {
    analyzing.value = false
  }
}

async function recompute(profile: VisualDNAProfile) {
  analyzing.value = true
  try {
    const created = await visualDnaApi.recompute(projectId(), profile.id)
    profiles.value = [created, ...profiles.value]
    selectedProfile.value = created
    ElMessage.success('Recompute created a new immutable version')
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not recompute Visual DNA')
  } finally {
    analyzing.value = false
  }
}

async function showSuggestion(profile: VisualDNAProfile) {
  try {
    const result = await visualDnaApi.suggestion(projectId(), profile.id)
    await ElMessageBox.alert(
      JSON.stringify(result.suggestion, null, 2),
      'Use as suggestion — no prompt mutation',
      { type: 'info', confirmButtonText: 'OK' },
    )
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not build suggestion')
  }
}

async function compare(profile: VisualDNAProfile) {
  if (!compareAssetId.value) {
    ElMessage.warning('Select a target asset')
    return
  }
  try {
    comparison.value = await visualDnaApi.compare(projectId(), profile.id, compareAssetId.value)
  } catch (err) {
    ElMessage.error(err instanceof Error ? err.message : 'Could not compare Visual DNA')
  }
}

function selectProfile(profile: VisualDNAProfile) {
  selectedProfile.value = profile
  comparison.value = null
}

onMounted(load)
</script>

<template>
  <el-container v-loading="loading" class="visual-dna">
    <el-header class="header">
      <div>
        <strong>Visual DNA v2</strong>
        <span class="muted">Actual image features, separate from Personal Visual Language</span>
      </div>
      <el-space>
        <RouterLink :to="'/projects/' + projectId() + '/resources'">
          <el-button>Creative Resources</el-button>
        </RouterLink>
        <RouterLink :to="'/projects/' + projectId() + '/studio'">
          <el-button plain>Studio</el-button>
        </RouterLink>
      </el-space>
    </el-header>

    <el-main>
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="Visual DNA never changes a prompt automatically. “Use as suggestion” only returns parameters and requires user approval."
      />

      <el-card class="analysis-card">
        <template #header>Analyze actual work</template>
        <el-space wrap>
          <el-select
            v-model="selectedAssetIds"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            clearable
            placeholder="Leave empty to analyze all allowed assets"
            style="min-width: 560px"
          >
            <el-option
              v-for="asset in sources"
              :key="asset.id"
              :label="asset.id + ' · ' + asset.width + '×' + asset.height"
              :value="asset.id"
            />
          </el-select>
          <el-button type="primary" :loading="analyzing" @click="analyze">
            Analyze
          </el-button>
          <el-tag>{{ selectedCountLabel }}</el-tag>
        </el-space>
        <div class="muted help">
          Только принадлежащие проекту активные assets без ограничения rights. Профиль хранит список исходных assets, алгоритм и uncertainty.
        </div>
      </el-card>

      <el-card class="profiles-card">
        <template #header>
          <div class="card-header">
            <span>Versioned profiles</span>
            <el-tag>{{ profiles.length }} versions</el-tag>
          </div>
        </template>

        <el-table :data="profiles" stripe>
          <el-table-column prop="version" label="Version" width="90" />
          <el-table-column prop="algorithm_version" label="Algorithm" width="150" />
          <el-table-column label="Sources" width="100">
            <template #default="scope">{{ scope.row.source_assets.length }}</template>
          </el-table-column>
          <el-table-column label="Confidence" width="120">
            <template #default="scope">{{ Number(scope.row.summary.confidence).toFixed(2) }}</template>
          </el-table-column>
          <el-table-column prop="created_at" label="Created" />
          <el-table-column label="Actions" width="310">
            <template #default="scope">
              <el-button size="small" @click="selectProfile(scope.row)">Details</el-button>
              <el-button size="small" @click="showSuggestion(scope.row)">Use as suggestion</el-button>
              <el-button size="small" type="primary" @click="recompute(scope.row)">Recompute</el-button>
            </template>
          </el-table-column>
        </el-table>
      </el-card>

      <el-row v-if="selectedProfile" :gutter="16" class="content-row">
        <el-col :span="12">
          <el-card>
            <template #header>Что повторяется в ваших работах</template>
            <el-space wrap>
              <el-tag v-for="item in selectedProfile.summary.recurring" :key="item" type="success">{{ item }}</el-tag>
              <span v-if="selectedProfile.summary.recurring.length === 0" class="muted">Нет устойчивого повторения в этой выборке.</span>
            </el-space>
          </el-card>
          <el-card>
            <template #header>Что появилось недавно</template>
            <el-space wrap>
              <el-tag v-for="item in selectedProfile.summary.emerging" :key="item" type="warning">{{ item }}</el-tag>
              <span v-if="selectedProfile.summary.emerging.length === 0" class="muted">Недостаточно данных для устойчивого тренда.</span>
            </el-space>
          </el-card>
          <el-card>
            <template #header>Uncertainty</template>
            <pre class="json">{{ JSON.stringify(selectedProfile.uncertainty, null, 2) }}</pre>
          </el-card>
        </el-col>

        <el-col :span="12">
          <el-card>
            <template #header>Сигналы профиля</template>
            <pre class="json">{{ JSON.stringify(selectedProfile.signals, null, 2) }}</pre>
          </el-card>
          <el-card>
            <template #header>Источники</template>
            <div class="source-list">
              <div v-for="id in selectedProfile.source_assets" :key="id">{{ id }}</div>
            </div>
            <div class="muted help">Algorithm: {{ selectedProfile.algorithm_version }}</div>
          </el-card>
        </el-col>
      </el-row>

      <el-card v-if="selectedProfile" class="comparison-card">
        <template #header>Чем этот проект отличается</template>
        <el-space wrap>
          <el-select v-model="compareAssetId" filterable clearable placeholder="Target asset" style="min-width: 520px">
            <el-option
              v-for="asset in sources"
              :key="asset.id"
              :label="asset.id + ' · ' + asset.width + '×' + asset.height"
              :value="asset.id"
            />
          </el-select>
          <el-button type="primary" @click="compare(selectedProfile)">Compare</el-button>
        </el-space>

        <el-table v-if="comparison" :data="comparison.comparison" stripe style="margin-top: 16px">
          <el-table-column prop="signal" label="Signal" />
          <el-table-column prop="profile_mean" label="Profile" />
          <el-table-column prop="asset_value" label="Asset" />
          <el-table-column prop="delta" label="Delta" />
          <el-table-column prop="interpretation" label="Interpretation" />
        </el-table>
      </el-card>
    </el-main>
  </el-container>
</template>

<style scoped>
.visual-dna {
  min-height: 100vh;
}
.header,
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.muted {
  color: var(--el-text-color-secondary);
}
.header .muted {
  margin-left: 12px;
}
.analysis-card,
.profiles-card,
.content-row,
.comparison-card {
  margin-top: 16px;
}
.help {
  margin-top: 12px;
  display: block;
}
.json {
  max-height: 430px;
  overflow: auto;
  padding: 12px;
  background: var(--el-fill-color-lighter);
  border-radius: 6px;
}
.source-list {
  max-height: 220px;
  overflow: auto;
  font-family: monospace;
}
</style>
