<template>
  <div class="env-settings">
    <div class="section-header">
      <div class="section-header__titlewrap">
        <h2>{{ t('envVarSettings.title') }}</h2>
        <t-popup placement="bottom-start" trigger="hover" :overlay-inner-style="{ maxWidth: '380px' }">
          <button type="button" class="hint-trigger" :aria-label="t('envVarSettings.helpAria')">
            <t-icon name="help-circle" size="16px" />
          </button>
          <template #content>
            <div class="hint-popover">
              <div class="hint-popover__block">
                <p class="hint-popover__title">{{ t('envVarSettings.introGlobalTitle') }}</p>
                <p class="hint-popover__text">{{ t('envVarSettings.introGlobalBody') }}</p>
              </div>
              <div class="hint-popover__block">
                <p class="hint-popover__title">{{ t('envVarSettings.introRuntimeTitle') }}</p>
                <p class="hint-popover__text">{{ t('envVarSettings.introRuntimeBody') }}</p>
              </div>
            </div>
          </template>
        </t-popup>
      </div>
      <p class="section-description">{{ t('envVarSettings.description') }}</p>
    </div>

    <p v-if="loading" class="env-state">{{ t('envVarSettings.loading') }}</p>

    <div v-else-if="loadError" class="env-state env-state--error">
      <p class="env-state__text">{{ loadError }}</p>
      <t-button variant="outline" size="small" @click="load()">
        {{ t('envVarSettings.retry') }}
      </t-button>
    </div>

    <template v-else>
      <section class="env-section">
        <header class="env-section__head env-section__head--row">
          <div>
            <h3>{{ t('envVarSettings.secretListTitle') }}</h3>
            <p>{{ t('envVarSettings.globalHint') }}</p>
          </div>
          <t-button
            v-if="!adding"
            variant="text"
            theme="primary"
            size="small"
            :title="t('envVarSettings.addSecret')"
            :aria-label="t('envVarSettings.addSecret')"
            @click="startAdd"
          >
            <template #icon><t-icon name="add" /></template>
            {{ t('envVarSettings.addSecret') }}
          </t-button>
        </header>

        <div v-if="adding" class="env-composer">
          <div class="env-new-row">
            <t-input
              v-model="newName"
              autocomplete="off"
              :placeholder="t('envVarSettings.namePlaceholder')"
              :status="newName && !isValidEnvName(newName) ? 'error' : undefined"
              class="env-new-row__name"
            />
            <t-input
              :value="newValue"
              type="password"
              autocomplete="new-password"
              :placeholder="t('envVarSettings.valuePlaceholder')"
              class="env-new-row__value"
              spellcheck="false"
              data-lpignore="true"
              data-1p-ignore="true"
              data-bwignore="true"
              @update:value="(v: string) => { newValue = v }"
            />
            <t-button
              variant="text"
              theme="primary"
              size="small"
              shape="square"
              :title="t('envVarSettings.save')"
              :aria-label="t('envVarSettings.save')"
              :loading="busyKey === 'new'"
              @click="saveNew"
            >
              <template #icon><t-icon name="check" /></template>
            </t-button>
            <t-button
              variant="text"
              shape="square"
              size="small"
              :title="t('common.cancel')"
              :aria-label="t('common.cancel')"
              @click="cancelAdd"
            >
              <template #icon><t-icon name="close" /></template>
            </t-button>
          </div>
          <p class="env-sandbox-card__empty">{{ t('envVarSettings.nameRule') }}</p>
        </div>

        <div v-if="secrets.length === 0 && !adding" class="env-empty">
          <p class="env-empty__title">{{ t('envVarSettings.globalEmptyTitle') }}</p>
          <p class="env-empty__desc">{{ t('envVarSettings.globalEmptyDesc') }}</p>
        </div>

        <div v-else class="env-secret-list">
          <div
            v-for="entry in secrets"
            :key="entry.name"
            class="env-kv"
            :class="{ 'is-editing': isEditing(entry.name) }"
          >
            <div class="env-kv__head">
              <div class="env-kv__key">
                <div class="env-secret__meta">
                  <code>{{ entry.name }}</code>
                  <span
                    class="env-tag"
                    :class="entry.has_value ? 'env-tag--set' : 'env-tag--unset'"
                  >
                    {{
                      entry.has_value
                        ? t('envVarSettings.hasValue')
                        : t('envVarSettings.noValue')
                    }}
                  </span>
                </div>
                <p v-if="entry.updated_at" class="env-secret__when">
                  {{ t('envVarSettings.updatedAt', { time: formatTime(entry.updated_at) }) }}
                </p>
              </div>
              <div v-if="!isEditing(entry.name)" class="env-kv__actions">
                <t-button
                  variant="text"
                  theme="primary"
                  size="small"
                  shape="square"
                  :title="
                    entry.has_value
                      ? t('envVarSettings.replaceValue')
                      : t('envVarSettings.setValue')
                  "
                  :aria-label="
                    entry.has_value
                      ? t('envVarSettings.replaceValue')
                      : t('envVarSettings.setValue')
                  "
                  @click="startEdit(entry.name)"
                >
                  <template #icon><t-icon name="key" /></template>
                </t-button>
                <t-popconfirm
                  v-if="entry.has_value"
                  theme="warning"
                  :content="t('envVarSettings.deleteConfirm', { name: entry.name })"
                  :confirm-btn="{ content: t('envVarSettings.delete'), theme: 'danger' }"
                  :cancel-btn="{ content: t('common.cancel') }"
                  @confirm="removeSecret(entry.name)"
                >
                  <t-button
                    theme="danger"
                    variant="text"
                    size="small"
                    shape="square"
                    :title="t('envVarSettings.delete')"
                    :aria-label="t('envVarSettings.delete')"
                    :loading="busyKey === entry.name"
                  >
                    <template #icon><t-icon name="delete" /></template>
                  </t-button>
                </t-popconfirm>
              </div>
            </div>
            <div v-if="isEditing(entry.name)" class="env-kv__editor">
              <t-input
                ref="editorInput"
                :value="drafts[entry.name] ?? ''"
                type="password"
                autocomplete="new-password"
                :name="`wk-global-${entry.name.length}`"
                spellcheck="false"
                data-lpignore="true"
                data-1p-ignore="true"
                data-bwignore="true"
                :aria-label="entry.name"
                :placeholder="
                  entry.has_value
                    ? t('envVarSettings.storedPlaceholder')
                    : t('envVarSettings.valuePlaceholder')
                "
                @update:value="(v: string) => setDraft(entry.name, v)"
                @enter="saveSecret(entry.name)"
              />
              <t-button
                variant="text"
                theme="primary"
                size="small"
                shape="square"
                :title="t('envVarSettings.save')"
                :aria-label="t('envVarSettings.save')"
                :loading="busyKey === entry.name"
                @click="saveSecret(entry.name)"
              >
                <template #icon><t-icon name="check" /></template>
              </t-button>
              <t-button
                variant="text"
                size="small"
                shape="square"
                :title="t('common.cancel')"
                :aria-label="t('common.cancel')"
                @click="cancelEdit(entry.name)"
              >
                <template #icon><t-icon name="close" /></template>
              </t-button>
            </div>
          </div>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { nextTick, onMounted, reactive, ref } from 'vue'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import {
  deleteGlobalSandboxEnv,
  listGlobalSandboxEnv,
  setGlobalSandboxEnv,
  type GlobalSandboxEnvView,
} from '@/api/sandbox-env'
import {
  MAX_ENV_VALUE_BYTES,
  MAX_USER_ENV_VARS_PER_SCOPE,
  isValidEnvName,
  isValidEnvValueLength,
} from './envVarState'

const { t, locale } = useI18n()

const loading = ref(true)
const loadError = ref('')
const secrets = ref<GlobalSandboxEnvView[]>([])

// Drafts are keyed by name so a reload cannot silently discard half-typed input.
const drafts = reactive<Record<string, string>>({})
const editingKey = ref('')
const editorInput = ref<{ focus?: () => void } | null>(null)
const busyKey = ref('')

const adding = ref(false)
const newName = ref('')
const newValue = ref('')

function isEditing(name: string) {
  return editingKey.value === name
}

function setDraft(name: string, value: string) {
  drafts[name] = value ?? ''
}

async function startEdit(name: string) {
  editingKey.value = name
  await nextTick()
  editorInput.value?.focus?.()
}

function cancelEdit(name: string) {
  if (editingKey.value === name) editingKey.value = ''
  delete drafts[name]
}

function formatTime(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return date.toLocaleString(locale.value)
}

async function load() {
  loading.value = true
  loadError.value = ''
  try {
    const res = await listGlobalSandboxEnv()
    secrets.value = res?.data || []
  } catch (e: any) {
    secrets.value = []
    loadError.value = e?.message || t('envVarSettings.loadFailed')
  } finally {
    loading.value = false
  }
}

/** A global secret value is required and bounded; the same guard for add and edit. */
function rejectBadValue(value: string): boolean {
  if (!value) {
    MessagePlugin.warning(t('envVarSettings.valueRequired'))
    return true
  }
  if (!isValidEnvValueLength(value)) {
    MessagePlugin.error(t('envVarSettings.valueTooLong', { max: MAX_ENV_VALUE_BYTES }))
    return true
  }
  return false
}

async function submit(key: string, action: () => Promise<unknown>, successMessage: string) {
  busyKey.value = key
  try {
    await action()
    // Write-only by design: the value is encrypted and never echoed, so keeping
    // it in the input would leave a secret in the DOM for no benefit.
    delete drafts[key]
    if (editingKey.value === key) editingKey.value = ''
    MessagePlugin.success(successMessage)
    await load()
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('envVarSettings.saveFailed'))
  } finally {
    busyKey.value = ''
  }
}

async function saveSecret(name: string) {
  const value = drafts[name] || ''
  if (rejectBadValue(value)) return
  await submit(name, () => setGlobalSandboxEnv(name, value), t('envVarSettings.saveSuccess'))
}

async function removeSecret(name: string) {
  await submit(
    name,
    () => deleteGlobalSandboxEnv(name),
    t('envVarSettings.deleteSuccess'),
  )
}

function startAdd() {
  adding.value = true
  newName.value = ''
  newValue.value = ''
}

function cancelAdd() {
  adding.value = false
  newName.value = ''
  newValue.value = ''
}

async function saveNew() {
  const name = newName.value.trim()
  if (!isValidEnvName(name)) {
    MessagePlugin.error(t('envVarSettings.nameInvalid'))
    return
  }
  if (secrets.value.some((entry) => entry.name === name)) {
    MessagePlugin.error(t('envVarSettings.nameDuplicate'))
    return
  }
  if (rejectBadValue(newValue.value)) return
  if (secrets.value.length >= MAX_USER_ENV_VARS_PER_SCOPE) {
    MessagePlugin.error(t('envVarSettings.tooManyValues', { max: MAX_USER_ENV_VARS_PER_SCOPE }))
    return
  }
  busyKey.value = 'new'
  try {
    await setGlobalSandboxEnv(name, newValue.value)
    cancelAdd()
    MessagePlugin.success(t('envVarSettings.saveSuccess'))
    await load()
  } catch (e: any) {
    MessagePlugin.error(e?.message || t('envVarSettings.saveFailed'))
  } finally {
    busyKey.value = ''
  }
}

onMounted(() => {
  void load()
})
</script>

<style lang="less" scoped>
@import (reference) '@/components/css/settings-section.less';

.env-settings {
  width: 100%;
}

.section-header {
  .settings-section-header();
}

.section-header__titlewrap {
  display: flex;
  align-items: center;
  gap: 6px;
}

.hint-trigger {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 2px;
  border: none;
  border-radius: var(--app-radius-xs);
  background: transparent;
  color: var(--td-text-color-placeholder);
  cursor: help;
  line-height: 1;

  &:hover,
  &:focus-visible {
    color: var(--td-brand-color);
  }
}

.hint-popover {
  display: flex;
  flex-direction: column;
  gap: 12px;
  max-width: 340px;
}

.hint-popover__title {
  margin: 0;
  color: var(--td-text-color-primary);
  font-size: var(--app-text-md);
  font-weight: 600;
}

.hint-popover__text {
  margin: 4px 0 0;
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-sm);
  line-height: 1.55;
}

.env-state {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
  margin: 0;
}

.env-state--error .env-state__text {
  margin: 0;
  color: var(--td-error-color);
}

.env-empty {
  background: var(--td-bg-color-secondarycontainer);
  border-radius: var(--app-radius-lg);
  padding: 24px;
  text-align: center;
}

.env-empty__title {
  font-size: var(--app-text-base);
  font-weight: 500;
  color: var(--td-text-color-secondary);
  margin: 0 0 4px 0;
}

.env-empty__desc {
  font-size: var(--app-text-md);
  color: var(--td-text-color-placeholder);
  margin: 0;
}

.env-section__head {
  margin-bottom: 12px;

  h3 {
    margin: 0;
    font-size: var(--app-text-lg);
    font-weight: 600;
    color: var(--td-text-color-primary);
  }

  p {
    margin: 4px 0 0;
    font-size: var(--app-text-md);
    line-height: 1.55;
    color: var(--td-text-color-secondary);
  }
}

.env-section__head--row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.env-composer {
  margin-bottom: 12px;
  padding: 12px 14px;
  border: 1px dashed var(--td-component-stroke);
  border-radius: var(--app-radius-lg);
  background: var(--td-bg-color-secondarycontainer);
}

.env-secret-list {
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-lg);
  padding: 4px 16px;
  background: var(--td-bg-color-container);
}

.env-kv {
  padding: 12px 0;
}

.env-kv + .env-kv {
  border-top: 1px solid var(--td-component-stroke);
}

.env-kv__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.env-kv__key {
  min-width: 0;
  flex: 1;
}

.env-kv__actions {
  display: flex;
  align-items: center;
  flex-shrink: 0;
  margin: -4px -4px 0 0;
}

.env-kv__editor {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 8px;
}

.env-kv__editor :deep(.t-input__wrap) {
  flex: 1;
  min-width: 0;
}

.env-secret__meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;

  code {
    font-family: var(--td-font-family-mono);
    font-size: var(--app-text-md);
    color: var(--td-text-color-primary);
    overflow-wrap: anywhere;
  }
}

.env-tag {
  font-size: var(--app-text-sm);
  line-height: 18px;
  padding: 0 8px;
  border-radius: var(--app-radius-lg);
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
}

.env-tag--set {
  background: var(--td-success-color-light);
  color: var(--td-success-color);
}

.env-tag--unset {
  background: var(--td-warning-color-light);
  color: var(--td-warning-color);
}

.env-secret__when {
  margin: 4px 0 0;
  font-size: var(--app-text-sm);
  line-height: 1.5;
  color: var(--td-text-color-placeholder);
}

.env-new-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.env-new-row__value {
  flex: 1;
  min-width: 0;
}

.env-new-row__name {
  flex: 0 0 34%;
}

.env-sandbox-card__empty {
  margin: 8px 0 0;
  font-size: var(--app-text-sm);
  line-height: 1.5;
  color: var(--td-text-color-placeholder);
}

@media (max-width: 720px) {
  .env-kv__head {
    flex-wrap: wrap;
  }

  .env-new-row,
  .env-new-row__name {
    flex-wrap: wrap;
    flex-basis: 100%;
  }
}
</style>
