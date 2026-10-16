<template>
  <div
    class="aside_box"
    :class="{
      'aside_box--collapsed': uiStore.sidebarCollapsed,
      'aside_box--resizing': uiStore.sidebarResizing,
    }"
  >
    <!-- 展开时：Logo + 搜索/折叠按钮同行 -->
    <div class="logo_row" v-if="!uiStore.sidebarCollapsed">
      <div
        class="logo_box"
        @click="router.push('/platform/creatChat')"
        style="cursor: pointer"
      >
        <img class="logo" :src="logoSrc" alt="" />
        <sup v-if="isLiteEdition" class="lite-badge">Lite</sup>
      </div>
      <div class="logo_actions">
        <ThemeToggle />
        <t-tooltip placement="bottom">
          <template #content>
            <span class="cmdk-tip">
              <span class="cmdk-tip-label">{{ t('menu.search') }}</span>
              <span class="cmdk-tip-keys">{{ cmdModKeyLabel }}K</span>
            </span>
          </template>
          <div
            class="header-icon-btn"
            @click="commandPaletteStore.openPalette('')"
            :aria-label="t('menu.search')"
          >
            <img
              class="header-icon-img"
              :src="getImgSrc('search.svg')"
              alt=""
            />
          </div>
        </t-tooltip>
        <div
          class="sidebar-toggle"
          @click="uiStore.toggleSidebar"
          :title="t('menu.collapseSidebar')"
        >
          <svg
            viewBox="0 0 20 20"
            width="16"
            height="16"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
          >
            <rect
              x="1.5"
              y="1.5"
              width="17"
              height="17"
              rx="3"
              stroke="currentColor"
              stroke-width="1.5"
            />
            <line
              x1="7.5"
              y1="1.5"
              x2="7.5"
              y2="18.5"
              stroke="currentColor"
              stroke-width="1.5"
            />
            <line
              x1="4"
              y1="7.5"
              x2="4"
              y2="12.5"
              stroke="currentColor"
              stroke-width="1.5"
              stroke-linecap="round"
            />
          </svg>
        </div>
      </div>
    </div>
    <!-- 折叠时：展开按钮 -->
    <t-tooltip
      v-if="uiStore.sidebarCollapsed"
      :content="t('menu.expandSidebar')"
      placement="right"
    >
      <div class="menu_item sidebar-toggle-item" @click="uiStore.toggleSidebar">
        <div class="menu_item-box">
          <div class="menu_icon">
            <svg
              class="icon icon--toggle"
              viewBox="0 0 20 20"
              width="16"
              height="16"
              fill="none"
              xmlns="http://www.w3.org/2000/svg"
            >
              <rect
                x="1.5"
                y="1.5"
                width="17"
                height="17"
                rx="3"
                stroke="currentColor"
                stroke-width="1.5"
              />
              <line
                x1="7.5"
                y1="1.5"
                x2="7.5"
                y2="18.5"
                stroke="currentColor"
                stroke-width="1.5"
              />
              <line
                x1="5"
                y1="10"
                x2="3"
                y2="8"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
              />
              <line
                x1="5"
                y1="10"
                x2="3"
                y2="12"
                stroke="currentColor"
                stroke-width="1.5"
                stroke-linecap="round"
              />
            </svg>
          </div>
        </div>
      </div>
    </t-tooltip>

    <!-- 空间选择器：仅在用户可切换空间时显示 -->
    <TenantSelector v-if="canAccessAllTenants && !uiStore.sidebarCollapsed" />

    <!-- 侧栏边缘拖拽调宽，拖窄时自动收缩 -->
    <PanelResizeHandle
      edge="right"
      :label="t('knowledgeStages.resizeDrawer')"
      :value="uiStore.sidebarDisplayWidth"
      :min="SIDEBAR_COLLAPSED_WIDTH"
      :max="SIDEBAR_MAX_WIDTH"
      @start="startSidebarResize"
      @resize="resizeSidebar"
      @end="uiStore.sidebarResizing = false"
    />

    <!-- 上半部分：新对话吸顶 + 知识库/智能体/共享空间/历史会话随滚动一起滚走 -->
    <div class="menu_top" ref="scrollContainer" @scroll="handleScroll">
      <!-- 全局搜索入口：点击打开命令面板（⌘K）。展开态移至顶部 logo_row 的图标按钮；
                 折叠态在此处保留为图标项 + 深色 tooltip。 -->
      <div class="menu_box menu_box--cmdk" v-if="uiStore.sidebarCollapsed">
        <t-tooltip placement="right">
          <template #content>
            <span class="cmdk-tip">
              <span class="cmdk-tip-label">{{ t('menu.search') }}</span>
              <span class="cmdk-tip-keys">{{ cmdModKeyLabel }}K</span>
            </span>
          </template>
          <div
            class="menu_item menu_item--cmdk"
            @click="commandPaletteStore.openPalette('')"
          >
            <div class="menu_item-box">
              <div class="menu_icon">
                <img class="icon" :src="getImgSrc('search.svg')" alt="" />
              </div>
            </div>
          </div>
        </t-tooltip>
        <t-tooltip :content="t('theme.theme')" placement="right">
          <div class="menu_item menu_item--cmdk">
            <div class="menu_item-box">
              <div class="menu_icon menu_icon--theme-toggle">
                <ThemeToggle />
              </div>
            </div>
          </div>
        </t-tooltip>
      </div>
      <div
        class="menu_box"
        :class="{
          'menu_box--sticky': item.children && !uiStore.sidebarCollapsed,
        }"
        v-for="(item, index) in topMenuItems"
        :key="index"
      >
        <t-tooltip
          :content="item.title"
          placement="right"
          :disabled="!uiStore.sidebarCollapsed"
        >
          <div
            @click="handleMenuClick(item.path)"
            @mouseenter="mouseenteMenu(item.path)"
            @mouseleave="mouseleaveMenu(item.path)"
            :data-guide="`nav-${item.path}`"
            :class="[
              'menu_item',
              item.childrenPath && item.childrenPath == currentpath
                ? 'menu_item_c_active'
                : isMenuItemActive(item.path)
                  ? 'menu_item_active'
                  : '',
            ]"
          >
            <div class="menu_item-box">
              <div class="menu_icon" :class="`menu_icon--${item.icon}`">
                <img
                  class="icon"
                  :src="
                    getImgSrc(
                      item.icon == 'zhishiku'
                        ? knowledgeIcon
                        : item.icon == 'agent'
                          ? agentIcon
                          : item.icon == 'artifact'
                            ? artifactIcon
                            : item.icon == 'programming'
                              ? programmingIcon
                              : item.icon == 'toolbox'
                                ? toolboxIcon
                                : item.icon == 'organization'
                                  ? organizationIcon
                                  : item.icon == 'logout'
                                    ? logoutIcon
                                    : item.icon == 'setting'
                                      ? settingIcon
                                      : prefixIcon,
                    )
                  "
                  alt=""
                />
              </div>
              <template v-if="!uiStore.sidebarCollapsed">
                <span class="menu_title" :title="item.title">{{
                  item.title
                }}</span>
                <span
                  v-if="
                    item.path === 'organizations' &&
                    orgStore.totalPendingJoinRequestCount > 0
                  "
                  class="menu-pending-badge"
                  :title="t('organization.settings.pendingJoinRequestsBadge')"
                  >{{ orgStore.totalPendingJoinRequestCount }}</span
                >
                <span
                  v-if="item.path === 'toolbox' && toolboxPreview.length"
                  class="menu-toolbox-stack"
                  :title="
                    toolboxPreview
                      .map((tool) =>
                        tool.key === 'browserconnection' && browserStackStatus
                          ? `${t(tool.title)} (${t(`localBrowser.${browserStackStatus}`)})`
                          : t(tool.title),
                      )
                      .join(' · ')
                  "
                >
                  <span
                    v-for="tool in toolboxPreview"
                    :key="tool.key"
                    class="menu-toolbox-stack__item"
                  >
                    <template v-if="tool.key === 'browserconnection'">
                      <BrowserIcon width="12" height="12" />
                      <i
                        v-if="browserStackStatus"
                        class="menu-toolbox-stack__status"
                        :class="`is-${browserStackStatus}`"
                        aria-hidden="true"
                      />
                    </template>
                    <t-icon v-else :name="tool.icon" size="12px" />
                  </span>
                </span>
              </template>
            </div>
          </div>
        </t-tooltip>
      </div>
      <!-- 历史会话：按来源筛选后统一按日期分组展示 -->
      <div class="submenu" v-if="!uiStore.sidebarCollapsed">
        <!-- Source selection, folder controls, and the conversation list share
             one compact hierarchy, matching the folder structure below. -->
        <div
          v-if="
            !batchMode && (showSessionSourceFilter || sessionFoldersEnabled)
          "
          class="session-folders-toolbar"
        >
          <button
            v-if="sessionFoldersEnabled"
            type="button"
            class="session-folders-heading"
            :aria-expanded="!projectsCollapsed"
            @click="toggleProjectsSection"
          >
            <span>{{ t('menu.folderSectionTitle') }}</span>
            <t-icon
              :name="projectsCollapsed ? 'chevron-right' : 'chevron-down'"
            />
          </button>
          <div
            v-else
            class="session-folders-heading session-folders-source-heading"
          >
            <SessionSourceFilter
              inline
              :emphasized="sessionScopeFilterPinned"
              :sources="sessionSourceOptions"
              :current="activeSessionBucketKey"
              @select="switchSessionBucket"
            />
          </div>
          <div
            v-if="sessionFoldersEnabled"
            class="session-folders-header-actions"
          >
            <SessionSourceFilter
              v-if="showSessionSourceFilter"
              inline
              :emphasized="sessionScopeFilterPinned"
              :sources="sessionSourceOptions"
              :current="activeSessionBucketKey"
              @select="switchSessionBucket"
            />
            <t-popup
              v-model:visible="folderGroupMenuOpen"
              trigger="click"
              placement="bottom-right"
              destroy-on-close
              overlay-class-name="card-more session-action-menu-popup"
            >
              <button
                type="button"
                class="session-folder-create session-folder-toolbar-icon-button"
                :aria-label="t('chatHeader.moreActions')"
                :title="t('chatHeader.moreActions')"
                aria-haspopup="menu"
                :aria-expanded="folderGroupMenuOpen"
              >
                <MoreIcon
                  class="session-folder-more-icon"
                  size="16px"
                  :stroke-width="2.2"
                />
              </button>
              <template #content>
                <div class="card-menu" @click.stop>
                  <t-popup
                    trigger="click"
                    placement="right-top"
                    destroy-on-close
                    overlay-class-name="card-more card-submenu-popup"
                  >
                    <button
                      type="button"
                      class="card-menu-item"
                      aria-haspopup="menu"
                    >
                      <t-icon name="arrow-up-down-2" class="icon" />
                      <span>{{ t('menu.folderSortMode') }}</span>
                      <t-icon
                        name="chevron-right"
                        class="card-menu-item__chevron"
                      />
                    </button>
                    <template #content>
                      <div class="card-menu" @click.stop>
                        <button
                          type="button"
                          class="card-menu-item"
                          @click="setFolderSortMode('recent')"
                        >
                          <span
                            class="card-menu-item__check"
                            :class="{
                              'is-active': folderSortMode === 'recent',
                            }"
                          >
                            <t-icon name="check" class="icon" />
                          </span>
                          <span>{{ t('menu.folderSortRecent') }}</span>
                        </button>
                        <button
                          type="button"
                          class="card-menu-item"
                          @click="setFolderSortMode('manual')"
                        >
                          <span
                            class="card-menu-item__check"
                            :class="{
                              'is-active': folderSortMode === 'manual',
                            }"
                          >
                            <t-icon name="check" class="icon" />
                          </span>
                          <span>{{ t('menu.folderSortManual') }}</span>
                        </button>
                      </div>
                    </template>
                  </t-popup>
                </div>
              </template>
            </t-popup>
            <button
              type="button"
              class="session-folder-create session-folder-toolbar-icon-button"
              :aria-label="t('menu.createFolder')"
              :title="t('menu.createFolder')"
              @click="openFolderCreateDialog"
            >
              <AddIcon size="16px" :stroke-width="2.2" />
            </button>
          </div>
        </div>
        <template v-if="sessionListBooting && !hasAnySession">
          <div
            v-for="n in 4"
            :key="'skel-' + n"
            class="submenu_item_p session-chat-row"
          >
            <div class="session-list-row session-list-row--flat">
              <t-skeleton
                animation="gradient"
                class="session-list-row__body"
                :row-col="[{ width: '100%', height: '14px' }]"
              />
            </div>
          </div>
        </template>

        <div v-else class="session-filtered-list">
          <template
            v-if="
              activeBucket?.loading &&
              !activeBucket.loaded &&
              filteredGroupedSessions.length === 0
            "
          >
            <div
              v-for="n in 4"
              :key="'bucket-skel-' + n"
              class="submenu_item_p session-chat-row"
            >
              <div class="session-list-row session-list-row--flat">
                <t-skeleton
                  animation="gradient"
                  class="session-list-row__body"
                  :row-col="[{ width: '100%', height: '14px' }]"
                />
              </div>
            </div>
          </template>
          <template
            v-else-if="
              activeBucket?.loaded &&
              filteredGroupedSessions.length === 0 &&
              (!sessionFoldersEnabled || conversationFolders.length === 0)
            "
          >
            <div class="submenu_empty">{{ t('menu.noSessions') }}</div>
          </template>
          <template v-else>
            <section
              v-if="sessionFoldersEnabled && !batchMode && !projectsCollapsed"
              v-for="folder in folderSections"
              :key="folder.id"
              class="session-folder-section"
            >
              <div
                class="session-folder-header"
                :class="{
                  'session-folder-header--expanded': !folder.collapsed,
                }"
              >
                <button
                  type="button"
                  class="session-folder-toggle"
                  @click="toggleFolder(folder.id)"
                >
                  <span class="session-folder-toggle-icon" aria-hidden="true">
                    <t-icon
                      name="folder"
                      size="18px"
                      class="session-folder-default-icon"
                    />
                    <t-icon
                      :name="
                        folder.collapsed ? 'chevron-right' : 'chevron-down'
                      "
                      size="18px"
                      class="session-folder-hover-chevron"
                    />
                  </span>
                  <span class="session-folder-name">{{ folder.name }}</span>
                </button>
                <div class="session-folder-actions">
                  <t-popup
                    :visible="folderMenuOpenId === folder.id"
                    trigger="click"
                    placement="bottom-right"
                    destroy-on-close
                    overlay-class-name="card-more session-action-menu-popup"
                    @visible-change="setFolderMenuVisibility(folder.id, $event)"
                  >
                    <button
                      type="button"
                      :aria-label="t('chatHeader.moreActions')"
                      :title="t('chatHeader.moreActions')"
                      aria-haspopup="menu"
                      @click.stop
                    >
                      <MoreIcon
                        class="session-folder-more-icon"
                        size="18px"
                        :stroke-width="2.2"
                      />
                    </button>
                    <template #content>
                      <div class="card-menu" @click.stop>
                        <button
                          v-if="folderSortMode === 'manual'"
                          type="button"
                          class="card-menu-item"
                          :disabled="!folderCanMove(folder.id, -1)"
                          @click="moveFolderBy(folder.id, -1)"
                        >
                          <t-icon name="arrow-up" class="icon" />
                          <span>{{ t('menu.moveFolderUp') }}</span>
                        </button>
                        <button
                          v-if="folderSortMode === 'manual'"
                          type="button"
                          class="card-menu-item"
                          :disabled="!folderCanMove(folder.id, 1)"
                          @click="moveFolderBy(folder.id, 1)"
                        >
                          <t-icon name="arrow-down" class="icon" />
                          <span>{{ t('menu.moveFolderDown') }}</span>
                        </button>
                        <button
                          type="button"
                          class="card-menu-item"
                          @click="startFolderBatchManage(folder.id)"
                        >
                          <t-icon name="queue" class="icon" />
                          <span>{{ t('menu.batchManage') }}</span>
                        </button>
                        <button
                          type="button"
                          class="card-menu-item"
                          @click="renameFolderFromMenu(folder)"
                        >
                          <t-icon name="edit-1" class="icon" />
                          <span>{{ t('menu.renameFolder') }}</span>
                        </button>
                        <button
                          type="button"
                          class="card-menu-item session-folder-menu-delete"
                          @click="confirmDeleteFolder(folder.id)"
                        >
                          <t-icon name="delete" class="icon" />
                          <span>{{ t('menu.deleteFolder') }}</span>
                        </button>
                      </div>
                    </template>
                  </t-popup>
                  <button
                    type="button"
                    class="session-folder-new-chat"
                    :aria-label="t('menu.newChatInFolder')"
                    :title="t('menu.newChatInFolder')"
                    @click="createChatInFolder(folder.id)"
                  >
                    <AddIcon size="18px" :stroke-width="2.2" />
                  </button>
                </div>
              </div>
              <Transition name="session-folder-content">
                <div v-if="!folder.collapsed" class="session-folder-content">
                  <div class="session-folder-content__inner">
                    <div
                      v-for="subitem in folder.items"
                      :key="subitem.id"
                      class="submenu_item_p session-chat-row session-folder-chat-row"
                      :data-session-id="subitem.id"
                      :class="{
                        'session-chat-row--active':
                          !batchMode && subitem.path === currentSecondpath,
                        'session-chat-row--selected':
                          batchMode && batchSelectedIds.includes(subitem.id),
                        'session-chat-row--revealed':
                          revealedSessionId === subitem.id,
                      }"
                    >
                      <div class="session-list-row session-list-row--flat">
                        <div class="session-list-row__body">
                          <SessionSidebarRow
                            :item="subitem"
                            :nested="true"
                            :batch-mode="batchMode"
                            :running="
                              Boolean(sessionActivityEntries[subitem.id])
                            "
                            :active-path="currentSecondpath"
                            :selected-ids="batchSelectedIds"
                            :menu-options="buildSessionMenuOptions(subitem)"
                            @navigate="gotopage(subitem.path)"
                            @toggle-select="toggleBatchSelect(subitem.id)"
                            @menu-click="
                              handleSessionMenuClick($event, subitem)
                            "
                            @rename-submit="
                              renameSessionTitle(subitem, $event.title)
                            "
                            @hover-in="mouseenteBotDownr(subitem.id)"
                            @hover-out="mouseleaveBotDown"
                          />
                        </div>
                      </div>
                    </div>
                    <div
                      v-if="folder.items.length === 0"
                      class="session-folder-empty"
                    >
                      {{ t('menu.emptyFolder') }}
                    </div>
                  </div>
                </div>
              </Transition>
            </section>
            <template v-for="group in filteredGroupedSessions" :key="group.key">
              <div
                v-if="group.label"
                class="timeline_header session-list-row session-list-row--flat"
              >
                <span class="session-list-row__body">
                  <span class="timeline_header-label">{{ group.label }}</span>
                </span>
              </div>
              <div
                v-for="subitem in group.items"
                :key="subitem.id"
                class="submenu_item_p session-chat-row"
                :data-session-id="subitem.id"
                :class="{
                  'session-chat-row--active':
                    !batchMode && subitem.path === currentSecondpath,
                  'session-chat-row--selected':
                    batchMode && batchSelectedIds.includes(subitem.id),
                  'session-chat-row--revealed':
                    revealedSessionId === subitem.id,
                }"
              >
                <div class="session-list-row session-list-row--flat">
                  <div class="session-list-row__body">
                    <SessionSidebarRow
                      :item="subitem"
                      :batch-mode="batchMode"
                      :running="Boolean(sessionActivityEntries[subitem.id])"
                      :unread="Boolean(sessionUnread[subitem.id])"
                      :active-path="currentSecondpath"
                      :selected-ids="batchSelectedIds"
                      :menu-options="buildSessionMenuOptions(subitem)"
                      @navigate="gotopage(subitem.path)"
                      @toggle-select="toggleBatchSelect(subitem.id)"
                      @menu-click="handleSessionMenuClick($event, subitem)"
                      @rename-submit="renameSessionTitle(subitem, $event.title)"
                      @hover-in="mouseenteBotDownr(subitem.id)"
                      @hover-out="mouseleaveBotDown"
                    />
                  </div>
                </div>
              </div>
            </template>
            <div
              v-if="activeBucket?.loading && filteredGroupedSessions.length > 0"
              class="session-list-loading session-list-row session-list-row--flat"
            >
              <span class="session-list-row__body">
                <t-loading size="small" />
              </span>
            </div>
          </template>
        </div>
      </div>
    </div>

    <!-- 批量管理底部操作条：固定在侧栏底部、用户头像上方 -->
    <div
      v-if="batchMode && !uiStore.sidebarCollapsed"
      class="batch-inline-footer"
    >
      <div class="batch-footer-left">
        <t-checkbox
          :checked="isAllBatchSelected"
          :indeterminate="isBatchIndeterminate"
          @change="toggleBatchSelectAll"
        >
          {{ t('batchManage.selectAll') }}
        </t-checkbox>
      </div>
      <div class="batch-footer-right">
        <t-button size="small" variant="text" @click="exitBatchMode">
          {{ t('batchManage.cancel') }}
        </t-button>
        <t-button
          size="small"
          theme="danger"
          variant="base"
          :disabled="batchSelectedIds.length === 0"
          :loading="batchDeleting"
          @click="handleInlineBatchDelete"
        >
          {{ t('batchManage.delete')
          }}{{ batchSelectedIds.length > 0 ? `(${batchDisplayCount})` : '' }}
        </t-button>
      </div>
    </div>

    <!-- 下半部分：用户菜单 -->
    <div class="menu_bottom">
      <UserMenu />
    </div>

    <t-dialog
      v-model:visible="folderCreateOpen"
      :header="folderDialogTitle"
      :confirm-btn="{
        content: folderDialogTitle,
        theme: 'primary',
      }"
      :cancel-btn="{ content: t('common.cancel') }"
      width="420px"
      @confirm="saveFolderName"
      @cancel="cancelFolderEdit"
      @update:visible="handleFolderCreateVisibility"
    >
      <div class="session-folder-create-dialog-body">
        <t-input
          v-model="folderDraft"
          :placeholder="t('menu.folderNamePlaceholder')"
          :maxlength="48"
          autofocus
          clearable
          @enter="saveFolderName"
          @focus="selectFolderNameOnOpen"
        />
      </div>
    </t-dialog>
  </div>
</template>

<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { onMounted, onUnmounted, watch, computed, ref, h, nextTick } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  getSessionsList,
  batchDelSessions,
  deleteAllSessions,
  getSession,
} from '@/api/chat/index'
import { useChatResourcesStore } from '@/stores/chatResources'
import { listAllIMChannels } from '@/api/agent/index'
import SessionSidebarRow from './SessionSidebarRow.vue'
import {
  emptyFolderLayout,
  normalizeFolderLayout,
  resolveFolderLayout,
  toServerLayout,
  type ConversationFolder,
  type FolderLayout,
} from './sessionFolderState'
import PanelResizeHandle from './PanelResizeHandle.vue'
import ThemeToggle from './ThemeToggle.vue'
import allthinkerLogo from '@/assets/img/allthinker.png'
import allthinkerLogoDark from '@/assets/img/allthinker-dark.png'
import { useTheme } from '@/composables/useTheme'
import {
  SIDEBAR_COLLAPSED_WIDTH,
  SIDEBAR_MIN_WIDTH,
  SIDEBAR_MAX_WIDTH,
} from '@/utils/sidebarWidth'
import {
  clearSession,
  removeSession,
  renameSession,
  SESSION_MUTATION_EVENT,
  setSessionPinned,
  type SessionMutationDetail,
} from './sessionMutations'
import SessionSourceFilter from './SessionSourceFilter.vue'
import {
  SIDEBAR_BUCKET_PAGE_SIZE,
  applyBucketCountProbe,
  buildBucketDefinitions,
  bucketHasMore,
  bucketVisible,
  createEmptyBucket,
  flattenBucketItems,
  isChannelBucket,
  isChannelBucketKey,
  mergeBucketPage,
  prependSessionToWebBucket,
  removeSessionFromBuckets,
  type SidebarSessionBucket,
} from './sessionSidebarBuckets'
import type { SessionForGrouping } from './sessionGrouping'
import { listAllEmbedChannels } from '@/api/embed/index'
import {
  classifyDateBucket,
  configuredPlatforms,
  groupSessionsByDate,
  originGroupKey,
  resolveSessionOrigin,
  type DateBucketKey,
} from './sessionGrouping'
import {
  DEFAULT_SESSION_BUCKET_KEY,
  buildSessionSourceOptions,
  findSessionBucketKey,
  shouldShowSessionSourceFilter,
} from './sessionSidebarSourceFilter'
import { logout as logoutApi, updateMyPreferences } from '@/api/auth'
import { useMenuStore } from '@/stores/menu'
import { useSessionActivityStore } from '@/stores/sessionActivity'
import { useAuthStore } from '@/stores/auth'
import { useDeploymentCapabilitiesStore } from '@/stores/deploymentCapabilities'
import { TOOLBOX_ITEMS, canAccessToolboxSection } from '@/config/toolbox'
import BrowserIcon from '@/components/icons/BrowserIcon.vue'
import { useBrowserConnectionStore } from '@/stores/browserConnection'
import { useOrganizationStore } from '@/stores/organization'
import { useUIStore } from '@/stores/ui'
import { useCommandPaletteStore } from '@/stores/commandPalette'
import { MessagePlugin, DialogPlugin, Icon as TIcon } from 'tdesign-vue-next'
import { AddIcon, MoreIcon } from 'tdesign-icons-vue-next'
import UserMenu from '@/components/UserMenu.vue'
import TenantSelector from '@/components/TenantSelector.vue'
import { useI18n } from 'vue-i18n'
import { useEditorResourcesStore } from '@/stores/editorResources'

const chatResources = useChatResourcesStore()
const editorResources = useEditorResourcesStore()
// Platform logos reused from IMChannelsOverviewPanel — keeps the session list
// visually consistent with the channels admin view.
import wecomLogo from '@/assets/img/im/wecom.svg'
import feishuLogo from '@/assets/img/im/feishu.svg'
import larkLogo from '@/assets/img/im/lark.svg'
import slackLogo from '@/assets/img/im/slack.svg'
import telegramLogo from '@/assets/img/im/telegram.svg'
import dingtalkLogo from '@/assets/img/im/dingtalk.svg'
import mattermostLogo from '@/assets/img/im/mattermost.svg'
import wechatLogo from '@/assets/img/im/wechat.svg'
import qqbotLogo from '@/assets/img/im/qqbot.png'

const PLATFORM_LOGO: Record<string, string> = {
  wecom: wecomLogo,
  feishu: feishuLogo,
  lark: larkLogo,
  slack: slackLogo,
  telegram: telegramLogo,
  dingtalk: dingtalkLogo,
  mattermost: mattermostLogo,
  wechat: wechatLogo,
  qqbot: qqbotLogo,
}

const platformLogo = (p: string): string => (p ? PLATFORM_LOGO[p] || '' : '')

const { t } = useI18n()
const { currentTheme } = useTheme()
// 侧边栏 logo：浅色用深色文字原图，深色用白字版（竖线保持 #507aff），
// 替代旧的 invert 滤镜方案——文字更纯白、竖线不偏色。
const logoSrc = computed(() => {
  if (currentTheme.value === 'light') return allthinkerLogo
  if (currentTheme.value === 'dark') return allthinkerLogoDark
  return document.documentElement.getAttribute('theme-mode') === 'dark'
    ? allthinkerLogoDark
    : allthinkerLogo
})
const usemenuStore = useMenuStore()
const sessionActivity = useSessionActivityStore()
const { entries: sessionActivityEntries, unread: sessionUnread } =
  storeToRefs(sessionActivity)
let sessionActivityTimer: ReturnType<typeof setInterval> | undefined
const authStore = useAuthStore()
const conversationFolders = ref<ConversationFolder[]>([])
const sessionFolderAssignments = ref<Record<string, string>>({})
const projectsCollapsed = ref(false)
/**
 * 项目排序方式：
 * - recent：按文件夹内最新会话的更新时间倒序
 * - manual：按 conversationFolders 的存储顺序，可用文件夹菜单的上移/下移调整
 * 默认 manual，与引入排序方式之前的表现一致。
 */
const folderSortMode = ref<'recent' | 'manual'>('manual')
const folderEditor = ref('')
const folderDraft = ref('')
const folderCreateOpen = ref(false)
/** 通过会话菜单“新项目”打开新建弹窗时，记住待移入的会话。 */
const pendingMoveSessionId = ref('')
const folderGroupMenuOpen = ref(false)
const folderMenuOpenId = ref('')
const folderStorageKey = computed(
  () =>
    `weknora_session_folders:${authStore.currentUserId || 'anonymous'}:${authStore.effectiveTenantId || 'default'}`,
)
/** 当前空间在 preferences.session_folders 里的键（服务端那份跨设备副本）。 */
const folderPrefTenantKey = computed(() =>
  String(authStore.effectiveTenantId || 'default'),
)
const currentFolderState = (): FolderLayout => ({
  folders: conversationFolders.value,
  assignments: sessionFolderAssignments.value,
  projectsCollapsed: projectsCollapsed.value,
  sortMode: folderSortMode.value,
})
const applyFolderState = (state: FolderLayout) => {
  conversationFolders.value = state.folders
  sessionFolderAssignments.value = state.assignments
  projectsCollapsed.value = state.projectsCollapsed
  folderSortMode.value = state.sortMode
}
const readLocalFolderState = (key: string): FolderLayout | null => {
  try {
    const raw = localStorage.getItem(key)
    return raw ? normalizeFolderLayout(JSON.parse(raw)) : null
  } catch {
    return null
  }
}
/** 写穿缓存：本地始终留一份，读取即时，接口不可用时也不至于丢掉组织。 */
const writeLocalFolderState = () => {
  try {
    localStorage.setItem(
      folderStorageKey.value,
      JSON.stringify(currentFolderState()),
    )
  } catch {
    // Folder organization remains available for this view if storage is unavailable.
  }
}
const serverFolderState = (): FolderLayout | null => {
  const entry =
    authStore.user?.preferences?.session_folders?.[folderPrefTenantKey.value]
  return entry ? normalizeFolderLayout(entry) : null
}
/**
 * 上报当前空间的文件夹组织。每次只发这一个空间，后端按空间键合并，
 * 所以不会覆盖同一账号在其它空间里的组织。
 */
const pushFolderStateToServer = async () => {
  const user = authStore.user
  if (!user) return
  // 还没和后端对齐过就先别写：此时本地可能是「空间键还没就绪」导致的空状态，
  // 推上去会把服务端已有的组织抹掉。
  if (!reconciledFolderKeys.has(folderStorageKey.value)) return
  const res = await updateMyPreferences({
    session_folders: {
      [folderPrefTenantKey.value]: toServerLayout(currentFolderState()),
    },
  })
  if (res.success && res.data) {
    // 回写 store，避免下次切空间回来时读到过期的服务端值又把新数据盖掉。
    authStore.setUser({ ...user, preferences: res.data })
  }
}
let folderSyncTimer: ReturnType<typeof setTimeout> | undefined
const persistConversationFolders = () => {
  writeLocalFolderState()
  if (folderSyncTimer) clearTimeout(folderSyncTimer)
  folderSyncTimer = setTimeout(() => {
    folderSyncTimer = undefined
    void pushFolderStateToServer()
  }, 1000)
}
/**
 * 登录 / 切换空间时对齐一次。取舍规则在 sessionFolderState.ts 里并配有测试：
 * 空状态永远不许覆盖非空状态——早先按「服务端有记录就算数」判断，空记录
 * 会被当成真相应用并写回另一侧，两边一起销毁。
 */
const reconciledFolderKeys = new Set<string>()
const reconcileFolderState = () => {
  const key = folderStorageKey.value
  if (reconciledFolderKeys.has(key)) return
  if (!authStore.user) return // 还没登录，等 user 到位再对齐
  reconciledFolderKeys.add(key)

  const { layout, upload } = resolveFolderLayout(
    serverFolderState(),
    readLocalFolderState(key),
  )
  applyFolderState(layout)
  writeLocalFolderState()
  // upload = 服务端没有可用内容而本地有：补写上去（升级迁移 / 空记录自愈）。
  if (upload) void pushFolderStateToServer()
}
// 先用本地缓存渲染，避免等接口时侧栏空白。
watch(
  folderStorageKey,
  (key) => {
    applyFolderState(readLocalFolderState(key) ?? emptyFolderLayout())
  },
  { immediate: true },
)
watch(
  [folderStorageKey, () => authStore.user?.id],
  () => reconcileFolderState(),
  { immediate: true },
)
const deploymentCapabilities = useDeploymentCapabilitiesStore()
const toolboxPreview = computed(() =>
  TOOLBOX_ITEMS.filter((item) =>
    canAccessToolboxSection(item.key, {
      currentTenantRole: authStore.currentTenantRole,
      canAccessAllTenants: authStore.canAccessAllTenants,
      isOperationsAdmin: authStore.isOperationsAdmin,
      hasRole: (role) => authStore.hasRole(role),
      isSupported: (capability) =>
        deploymentCapabilities.isSupported(capability),
    }),
  ),
)
const orgStore = useOrganizationStore()
const uiStore = useUIStore()
const browserConnection = useBrowserConnectionStore()
const browserStackStatus = computed(() => {
  if (!uiStore.sidebarBrowserStatus) return ''
  if (
    !browserConnection.loaded ||
    !browserConnection.enabled ||
    !browserConnection.device
  )
    return ''
  return browserConnection.connected ? 'connected' : 'offline'
})
watch(
  () =>
    uiStore.sidebarBrowserStatus &&
    toolboxPreview.value.some((tool) => tool.key === 'browserconnection'),
  (visible) => {
    if (visible && !browserConnection.loaded)
      browserConnection.refresh().catch(() => {})
  },
  { immediate: true },
)
const commandPaletteStore = useCommandPaletteStore()

// Platform-aware label for the ⌘K hint. navigator.platform is deprecated but
// the alternatives (userAgentData.platform) aren't universally available yet;
// this check is good enough for Mac vs. non-Mac.
const isMacLike =
  typeof navigator !== 'undefined' &&
  /Mac|iPod|iPhone|iPad/.test(navigator.platform || '')
const cmdModKeyLabel = isMacLike ? '⌘' : 'Ctrl'
const route = useRoute()
const router = useRouter()
const currentpath = ref('')
const total = ref(0)
const sessionBuckets = ref<Record<string, SidebarSessionBucket>>({})
const bucketOrder = ref<string[]>([])
let bucketRequestToken = 0
const sessionListBooting = ref(false)
const currentSecondpath = ref('')
const scrollContainer = ref<HTMLElement | null>(null)
const imPlatforms = ref<string[]>([])
const embedChannelNames = ref<Record<string, string>>({})
const activeSessionBucketKey = ref(DEFAULT_SESSION_BUCKET_KEY)
const sessionListCanScroll = ref(false)
const visibleChannelBuckets = computed(() =>
  bucketOrder.value
    .map((key) => sessionBuckets.value[key])
    .filter(
      (bucket): bucket is SidebarSessionBucket =>
        !!bucket && isChannelBucket(bucket) && bucketVisible(bucket),
    ),
)
const showSessionSourceFilter = computed(() =>
  shouldShowSessionSourceFilter(visibleChannelBuckets.value.length),
)
const sessionScopeFilterPinned = computed(
  () => activeSessionBucketKey.value !== DEFAULT_SESSION_BUCKET_KEY,
)
const sessionSourceOptions = computed(() =>
  buildSessionSourceOptions(
    t('menu.myChats'),
    visibleChannelBuckets.value.map((bucket) => ({
      key: bucket.key,
      label: bucket.label,
      platform: bucket.platform,
    })),
    (platform) => platformLogo(platform),
  ),
)
const activeBucket = computed(
  () => sessionBuckets.value[activeSessionBucketKey.value],
)
const hasAnySession = computed(() =>
  Object.values(sessionBuckets.value).some((bucket) => bucket.items.length > 0),
)
type MenuItem = {
  title: string
  icon: string
  path: string
  childrenPath?: string
  children?: any[]
}
const { menuArr, visibleMenuArr } = storeToRefs(usemenuStore)
let activeSubmenu = ref<string>('')
const isLiteEdition = ref(false)

// 批量管理状态
const batchMode = ref(false)
const batchSelectedIds = ref<string[]>([])
const batchDeleting = ref(false)
/** 非空时批量管理限定在该文件夹内：侧栏只列它的会话，全选/计数/删除都只覆盖它。 */
const batchScopeFolderId = ref('')

const allSessionIds = computed(() => {
  const chatMenu = (menuArr.value as unknown as MenuItem[]).find(
    (item: MenuItem) => item.path === 'creatChat',
  )
  if (!chatMenu?.children) return []
  return (chatMenu.children as any[]).map((s: any) => s.id)
})

/** 批量管理实际覆盖的会话 id，限定文件夹时只取该文件夹内的会话。 */
const batchSessionIds = computed(() => {
  if (!batchScopeFolderId.value) return allSessionIds.value
  const items = activeBucket.value?.items ?? []
  return items
    .filter(
      (item) =>
        sessionFolderAssignments.value[item.id] === batchScopeFolderId.value,
    )
    .map((item) => item.id)
})

const isAllBatchSelected = computed(
  () =>
    batchSessionIds.value.length > 0 &&
    batchSelectedIds.value.length === batchSessionIds.value.length,
)

const isBatchIndeterminate = computed(
  () =>
    batchSelectedIds.value.length > 0 &&
    batchSelectedIds.value.length < batchSessionIds.value.length,
)

const batchDisplayCount = computed(() => {
  if (!isAllBatchSelected.value) return batchSelectedIds.value.length
  // 限定文件夹时 total 是全局会话数，与“只删这些”的实际动作不符。
  return batchScopeFolderId.value ? batchSessionIds.value.length : total.value
})

// 是否可以访问所有空间
const canAccessAllTenants = computed(() => authStore.canAccessAllTenants)

// 是否处于知识库详情页（不包括全局聊天）
const isInKnowledgeBase = computed<boolean>(() => {
  return (
    route.name === 'knowledgeBaseDetail' ||
    route.name === 'kbCreatChat' ||
    route.name === 'knowledgeBaseSettings'
  )
})

// 是否在知识库列表页面
const isInKnowledgeBaseList = computed<boolean>(() => {
  return route.name === 'knowledgeBaseList'
})

// 是否在创建聊天页面
const isInCreatChat = computed<boolean>(() => {
  return route.name === 'globalCreatChat' || route.name === 'kbCreatChat'
})

// 是否在对话详情页
const isInChatDetail = computed<boolean>(() => route.name === 'chat')
// Every conversation now shares the same chat route; the sidebar no longer has
// a separate coding mode.
const sessionPath = (id: string) => `chat/${id}`

// 是否在智能体列表页面
const isInAgentList = computed<boolean>(() => route.name === 'agentList')

// 是否在组织列表页面
const isInOrganizationList = computed<boolean>(
  () => route.name === 'organizationList',
)

// 统一的菜单项激活状态判断
const isMenuItemActive = (itemPath: string): boolean => {
  const currentRoute = route.name

  switch (itemPath) {
    case 'knowledge-bases':
      return (
        currentRoute === 'knowledgeBaseList' ||
        currentRoute === 'knowledgeBaseDetail' ||
        currentRoute === 'knowledgeBaseSettings'
      )
    case 'agents':
      return currentRoute === 'agentList'
    case 'toolbox':
      return currentRoute === 'toolbox'
    case 'artifacts':
      return currentRoute === 'artifactLibrary'
    case 'programming':
      return currentRoute === 'programmingSpace'
    case 'organizations':
      return currentRoute === 'organizationList'
    case 'creatChat':
      return (
        currentRoute === 'kbCreatChat' || currentRoute === 'globalCreatChat'
      )
    case 'settings':
      return currentRoute === 'settings'
    default:
      return itemPath === currentpath.value
  }
}

// 统一的图标激活状态判断
const getIconActiveState = (itemPath: string) => {
  const currentRoute = route.name

  return {
    isKbActive:
      itemPath === 'knowledge-bases' &&
      (currentRoute === 'knowledgeBaseList' ||
        currentRoute === 'knowledgeBaseDetail' ||
        currentRoute === 'knowledgeBaseSettings'),
    isCreatChatActive:
      itemPath === 'creatChat' &&
      (currentRoute === 'kbCreatChat' || currentRoute === 'globalCreatChat'),
    isSettingsActive: itemPath === 'settings' && currentRoute === 'settings',
    isChatActive: itemPath === 'chat' && currentRoute === 'chat',
  }
}

// 分离上下两部分菜单（使用 visibleMenuArr 以便 lite 模式过滤 logout）
const TOP_MENU_PATHS = new Set([
  'creatChat',
  'knowledge-bases',
  'programming',
  'artifacts',
  'agents',
  'toolbox',
  'organizations',
])

const topMenuItems = computed<MenuItem[]>(() => {
  return (visibleMenuArr.value as unknown as MenuItem[]).filter(
    (item: MenuItem) => TOP_MENU_PATHS.has(item.path),
  )
})

const bottomMenuItems = computed<MenuItem[]>(() => {
  return (visibleMenuArr.value as unknown as MenuItem[]).filter(
    (item: MenuItem) => !TOP_MENU_PATHS.has(item.path),
  )
})

// 当前知识库信息
const currentKbName = ref<string>('')
const currentKbInfo = ref<any>(null)

// 进行中的置顶/取消置顶请求，避免重复点击
const pinningIds = ref<Set<string>>(new Set())

// 「聊天」区内按日期分组（当前筛选来源）
const dateBucketLabels = computed<Record<DateBucketKey, string>>(() => ({
  pinned: t('time.pinned'),
  today: t('time.today'),
  yesterday: t('time.yesterday'),
  last7Days: t('time.last7Days'),
  last30Days: t('time.last30Days'),
  lastYear: t('time.lastYear'),
  earlier: t('time.earlier'),
}))

const sessionFoldersEnabled = computed(() => activeBucket.value?.kind === 'web')
const sessionHasFolder = (sessionId: string) => {
  const folderId = sessionFolderAssignments.value[sessionId]
  return Boolean(
    folderId &&
      conversationFolders.value.some((folder) => folder.id === folderId),
  )
}
/** 文件夹内最新会话的时间戳，用于「最近更新」排序。 */
const folderLatestActivity = (section: {
  items: Array<{ updated_at?: string; created_at?: string }>
}): number => {
  let latest = 0
  for (const item of section.items) {
    const ts = Date.parse(item.updated_at || item.created_at || '')
    if (!Number.isNaN(ts) && ts > latest) latest = ts
  }
  return latest
}
const folderSections = computed(() => {
  const items = activeBucket.value?.items ?? []
  const sections = conversationFolders.value.map((folder) => ({
    ...folder,
    items: items
      .filter((item) => sessionFolderAssignments.value[item.id] === folder.id)
      .map((item) => ({
        ...item,
        path: sessionPath(item.id),
        title: item.title || '',
      })),
  }))
  if (folderSortMode.value === 'manual') return sections
  // 「最近更新」：按文件夹内最新会话倒序，空文件夹没有时间戳因而排在最后。
  return [...sections].sort(
    (a, b) => folderLatestActivity(b) - folderLatestActivity(a),
  )
})

const filteredGroupedSessions = computed(() => {
  const bucket = activeBucket.value
  if (!bucket?.items.length) return []
  let visibleItems = bucket.items
  if (sessionFoldersEnabled.value) {
    if (batchMode.value) {
      // 限定文件夹的批量管理只列该文件夹内的会话，其余全部隐藏。
      visibleItems = batchScopeFolderId.value
        ? bucket.items.filter(
            (item) =>
              sessionFolderAssignments.value[item.id] ===
              batchScopeFolderId.value,
          )
        : bucket.items
    } else {
      visibleItems = bucket.items.filter((item) => !sessionHasFolder(item.id))
    }
  }
  return groupSessionsByDate(
    visibleItems.map((item) => ({
      ...item,
      path: sessionPath(item.id),
      title: item.title || '',
    })),
    dateBucketLabels.value,
    (session) => classifyDateBucket(session.updated_at || session.created_at),
  )
})

const persistFoldersAfterEdit = () => persistConversationFolders()
const toggleProjectsSection = () => {
  projectsCollapsed.value = !projectsCollapsed.value
  persistFoldersAfterEdit()
}
const prepareFolderCreate = () => {
  folderEditor.value = 'create'
  folderDraft.value = ''
}
const openFolderCreateDialog = () => {
  prepareFolderCreate()
  folderCreateOpen.value = true
}
const handleFolderCreateVisibility = (visible: boolean) => {
  folderCreateOpen.value = visible
  // 新建与重命名共用同一个弹窗，关闭时都要清掉编辑状态。
  if (!visible) cancelFolderEdit()
}
/** 弹窗打开时把旧名字全选，方便直接输入新名（沿用原先内联编辑的行为）。 */
const pendingFolderNameSelect = ref(false)
const selectFolderNameOnOpen = (
  _value: string,
  context: { e?: FocusEvent },
) => {
  if (!pendingFolderNameSelect.value) return
  pendingFolderNameSelect.value = false
  const el = context?.e?.target as HTMLInputElement | undefined
  el?.select()
}
/** 重命名复用新建文件夹的弹窗，只换标题与按钮文案。 */
const openFolderRenameDialog = (folder: ConversationFolder) => {
  folderEditor.value = folder.id
  folderDraft.value = folder.name
  pendingFolderNameSelect.value = true
  folderCreateOpen.value = true
}
const folderDialogTitle = computed(() =>
  folderEditor.value === 'create'
    ? t('menu.createFolder')
    : t('menu.renameFolder'),
)
const cancelFolderEdit = () => {
  folderEditor.value = ''
  folderDraft.value = ''
  folderCreateOpen.value = false
  pendingFolderNameSelect.value = false
  pendingMoveSessionId.value = ''
}
const saveFolderName = () => {
  const name = folderDraft.value.trim().slice(0, 48)
  if (!name) return
  const pendingSessionId = pendingMoveSessionId.value
  pendingMoveSessionId.value = ''
  if (folderEditor.value === 'create') {
    const id =
      typeof crypto !== 'undefined' && 'randomUUID' in crypto
        ? crypto.randomUUID()
        : `folder-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    conversationFolders.value.push({ id, name, collapsed: false })
    projectsCollapsed.value = false
    if (pendingSessionId) moveSessionToFolder(pendingSessionId, id)
  } else {
    const folder = conversationFolders.value.find(
      (item) => item.id === folderEditor.value,
    )
    if (folder) folder.name = name
  }
  cancelFolderEdit()
  persistFoldersAfterEdit()
}
const toggleFolder = (folderId: string) => {
  const folder = conversationFolders.value.find((item) => item.id === folderId)
  if (!folder) return
  folder.collapsed = !folder.collapsed
  persistFoldersAfterEdit()
}
const startFolderBatchManage = (folderId: string) => {
  folderMenuOpenId.value = ''
  enterBatchMode(folderId)
}
const renameFolderFromMenu = (folder: ConversationFolder) => {
  folderMenuOpenId.value = ''
  openFolderRenameDialog(folder)
}
const createChatInFolder = (folderId: string) => {
  const folder = conversationFolders.value.find((item) => item.id === folderId)
  if (!folder) return
  folder.collapsed = false
  projectsCollapsed.value = false
  folderMenuOpenId.value = ''
  persistFoldersAfterEdit()
  router.push({
    path: '/platform/creatChat',
    query: { ...route.query, folderId },
  })
}
const setFolderSortMode = (mode: 'recent' | 'manual') => {
  folderSortMode.value = mode
  folderGroupMenuOpen.value = false
  persistFoldersAfterEdit()
}
/** 手动排序下方向上/向下移动一位；已在边界时不动。 */
const folderCanMove = (folderId: string, delta: number): boolean => {
  const list = conversationFolders.value
  const from = list.findIndex((folder) => folder.id === folderId)
  const to = from + delta
  return from >= 0 && to >= 0 && to < list.length
}
const moveFolderBy = (folderId: string, delta: number) => {
  const list = conversationFolders.value
  const from = list.findIndex((folder) => folder.id === folderId)
  const to = from + delta
  if (from < 0 || to < 0 || to >= list.length) return
  const [moved] = list.splice(from, 1)
  list.splice(to, 0, moved)
  folderMenuOpenId.value = ''
  persistFoldersAfterEdit()
}
const deleteFolder = (folderId: string) => {
  if (folderMenuOpenId.value === folderId) folderMenuOpenId.value = ''
  conversationFolders.value = conversationFolders.value.filter(
    (folder) => folder.id !== folderId,
  )
  for (const [sessionId, assignedFolderId] of Object.entries(
    sessionFolderAssignments.value,
  )) {
    if (assignedFolderId === folderId)
      delete sessionFolderAssignments.value[sessionId]
  }
  persistFoldersAfterEdit()
}
const confirmDeleteFolder = (folderId: string) => {
  const folder = conversationFolders.value.find((item) => item.id === folderId)
  if (!folder) return
  folderMenuOpenId.value = ''
  const confirmDialog = DialogPlugin.confirm({
    header: t('menu.deleteFolderConfirmTitle'),
    body: t('menu.deleteFolderConfirmBody', { folder: folder.name }),
    confirmBtn: {
      content: t('menu.deleteFolder'),
      theme: 'danger' as const,
    },
    cancelBtn: t('common.cancel'),
    theme: 'warning',
    onConfirm: () => {
      deleteFolder(folderId)
      confirmDialog.destroy()
    },
  })
}
const setFolderMenuVisibility = (folderId: string, visible: boolean) => {
  folderMenuOpenId.value = visible ? folderId : ''
}
const moveSessionToFolder = (sessionId: string, folderId: string) => {
  if (
    folderId &&
    conversationFolders.value.some((folder) => folder.id === folderId)
  ) {
    sessionFolderAssignments.value[sessionId] = folderId
  } else {
    delete sessionFolderAssignments.value[sessionId]
  }
  persistFoldersAfterEdit()
}

// Only a locally created fork requests attention; loading history and switching
// between existing sessions must not replay the entrance animation.
const pendingForkRevealId = ref('')
const revealedSessionId = ref('')
let forkRevealTimer: ReturnType<typeof setTimeout> | undefined
usemenuStore.$onAction(({ name, args, after }) => {
  if (name !== 'updataMenuChildren' || !args[0]) return
  const session = args[0]
  const sessionId = String(session.id)
  const targetFolderId =
    typeof route.query.folderId === 'string' ? route.query.folderId : ''
  if (
    targetFolderId &&
    conversationFolders.value.some((folder) => folder.id === targetFolderId)
  ) {
    after(() => moveSessionToFolder(sessionId, targetFolderId))
  }
  if (!session.parent_session_id) return
  after(() => {
    pendingForkRevealId.value = sessionId
  })
})

watch(
  () => {
    const id = pendingForkRevealId.value
    return id &&
      !uiStore.sidebarCollapsed &&
      currentSecondpath.value === `chat/${id}` &&
      filteredGroupedSessions.value.some((group) =>
        group.items.some((item) => item.id === id),
      )
      ? id
      : ''
  },
  (id) => {
    if (!id) return
    const container = scrollContainer.value
    const row = Array.from(
      container?.querySelectorAll<HTMLElement>('[data-session-id]') ?? [],
    ).find((element) => element.dataset.sessionId === id)
    if (!container || !row) return

    // Reveal within the sidebar only, without moving the conversation pane.
    const bounds = container.getBoundingClientRect()
    const rowBounds = row.getBoundingClientRect()
    if (rowBounds.top < bounds.top)
      container.scrollTop += rowBounds.top - bounds.top
    else if (rowBounds.bottom > bounds.bottom)
      container.scrollTop += rowBounds.bottom - bounds.bottom

    clearTimeout(forkRevealTimer)
    revealedSessionId.value = id
    pendingForkRevealId.value = ''
    forkRevealTimer = setTimeout(() => {
      revealedSessionId.value = ''
    }, 350)
  },
  { flush: 'post' },
)

const refreshSessionListScrollability = async () => {
  await nextTick()
  const container = scrollContainer.value
  sessionListCanScroll.value =
    !!container && container.scrollHeight > container.clientHeight + 1
}

/** 列表未撑满滚动区时自动续页（按当前可见 DOM 测量，避免折叠导致误判） */
const ensureBucketFillsViewport = async (key: string) => {
  const MAX_ITERATIONS = 20
  for (let i = 0; i < MAX_ITERATIONS; i++) {
    await nextTick()
    await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()))
    const container = scrollContainer.value
    const bucket = sessionBuckets.value[key]
    if (!container || !bucket || !bucketHasMore(bucket) || bucket.loading) break

    const hasOverflow = container.scrollHeight > container.clientHeight + 1
    if (hasOverflow) break

    const prevCount = bucket.items.length
    await loadBucketPage(key)
    if ((sessionBuckets.value[key]?.items.length ?? 0) <= prevCount) break
  }
}

const mouseenteBotDownr = (val: string) => {
  activeSubmenu.value = val
}
const mouseleaveBotDown = () => {
  activeSubmenu.value = ''
}

const enterBatchMode = (folderId = '') => {
  batchMode.value = true
  batchScopeFolderId.value = folderId
  batchSelectedIds.value = []
}

const exitBatchMode = () => {
  batchMode.value = false
  batchScopeFolderId.value = ''
  batchSelectedIds.value = []
}

const toggleBatchSelect = (id: string) => {
  const idx = batchSelectedIds.value.indexOf(id)
  if (idx > -1) {
    batchSelectedIds.value.splice(idx, 1)
  } else {
    batchSelectedIds.value.push(id)
  }
}

const toggleBatchSelectAll = (checked: boolean) => {
  batchSelectedIds.value = checked ? [...batchSessionIds.value] : []
}

const handleInlineBatchDelete = () => {
  if (batchSelectedIds.value.length === 0) return
  // 限定文件夹时绝不可走 deleteAllSessions（那是清空整个会话列表），
  // 全选也只应删除该文件夹内的会话。
  const isDeleteAll = isAllBatchSelected.value && !batchScopeFolderId.value
  const displayCount = batchDisplayCount.value
  const confirmDialog = DialogPlugin.confirm({
    header: t('batchManage.deleteConfirmTitle'),
    body: isDeleteAll
      ? t('batchManage.deleteAllConfirmBody') ||
        t('batchManage.deleteConfirmBody', { count: displayCount })
      : t('batchManage.deleteConfirmBody', { count: displayCount }),
    confirmBtn: { content: t('batchManage.delete'), theme: 'danger' as const },
    cancelBtn: t('batchManage.cancel'),
    theme: 'warning',
    onConfirm: async () => {
      batchDeleting.value = true
      try {
        let res: any
        if (isDeleteAll) {
          res = await deleteAllSessions()
        } else {
          res = await batchDelSessions([...batchSelectedIds.value])
        }
        if (res && res.success === true) {
          if (isDeleteAll) {
            usemenuStore.clearMenuArr()
            total.value = 0
            await getMessageList()
          } else {
            let next = sessionBuckets.value
            for (const id of batchSelectedIds.value) {
              next = removeSessionFromBuckets(next, id)
            }
            sessionBuckets.value = next
            syncMenuStoreFromBuckets()
          }
          const currentChatId = route.params.chatid as string
          if (
            currentChatId &&
            (isDeleteAll || batchSelectedIds.value.includes(currentChatId))
          ) {
            router.push('/platform/creatChat')
          }
          batchSelectedIds.value = []
          MessagePlugin.success(t('batchManage.deleteSuccess'))
          exitBatchMode()
        } else {
          MessagePlugin.error(t('batchManage.deleteFailed'))
        }
      } catch {
        MessagePlugin.error(t('batchManage.deleteFailed'))
      }
      batchDeleting.value = false
      confirmDialog.destroy()
    },
  })
}

const handleSessionMenuClick = (data: { value: string }, item: any) => {
  if (data?.value === 'removeFromFolder') {
    moveSessionToFolder(item.id, '')
  } else if (data?.value?.startsWith('moveToFolder:')) {
    moveSessionToFolder(item.id, data.value.slice('moveToFolder:'.length))
  } else if (data?.value === 'newFolderAndMove') {
    pendingMoveSessionId.value = item.id
    openFolderCreateDialog()
  } else if (data?.value === 'delete') {
    delCard(item)
  } else if (data?.value === 'clearMessages') {
    clearMessages(item)
  } else if (data?.value === 'batchManage') {
    enterBatchMode()
  } else if (data?.value === 'pin' || data?.value === 'unpin') {
    togglePin(item, data.value === 'pin')
  }
}

// 基于会话来源推导展示用的短标签已经被 platformLogo(<img>) 取代，Web 会话没有图标。

const buildSessionMenuOptions = (item: any) => {
  const options: any[] = []
  if (item.is_pinned) {
    options.push({
      content: t('menu.unpin'),
      value: 'unpin',
      prefixIcon: () => h(TIcon, { name: 'pin-filled' }),
    })
  } else {
    options.push({
      content: t('menu.pin'),
      value: 'pin',
      prefixIcon: () => h(TIcon, { name: 'pin' }),
    })
  }
  options.push(
    {
      content: t('menu.renameSession'),
      value: 'rename',
      prefixIcon: () => h(TIcon, { name: 'edit-1' }),
    },
    {
      content: t('menu.clearMessages'),
      value: 'clearMessages',
      prefixIcon: () => h(TIcon, { name: 'clear' }),
    },
    // 文件夹内的会话改由文件夹「更多」菜单统一批量管理，作用域限定该文件夹。
    ...(sessionHasFolder(item.id)
      ? []
      : [
          {
            content: t('menu.batchManage'),
            value: 'batchManage',
            prefixIcon: () => h(TIcon, { name: 'queue' }),
          },
        ]),
    {
      content: t('upload.deleteRecord'),
      value: 'delete',
      theme: 'error',
      prefixIcon: () => h(TIcon, { name: 'delete' }),
    },
  )
  if (sessionFoldersEnabled.value && conversationFolders.value.length > 0) {
    options.splice(
      options.length - 1,
      0,
      ...[
        ...(sessionHasFolder(item.id)
          ? [
              {
                content: t('menu.removeFromFolder'),
                value: 'removeFromFolder',
                prefixIcon: () => h(TIcon, { name: 'folder-open' }),
              },
            ]
          : []),
        {
          content: t('menu.moveToProject'),
          value: 'moveToProject',
          prefixIcon: () => h(TIcon, { name: 'folder-move' }),
          children: [
            {
              content: t('menu.newProject'),
              value: 'newFolderAndMove',
              prefixIcon: () => h(TIcon, { name: 'folder-add' }),
            },
            ...conversationFolders.value
              .filter(
                (folder) =>
                  sessionFolderAssignments.value[item.id] !== folder.id,
              )
              .map((folder, index) => ({
                content: folder.name,
                value: `moveToFolder:${folder.id}`,
                prefixIcon: () => h(TIcon, { name: 'folder' }),
                ...(index === 0 ? { dividerBefore: true } : {}),
              })),
          ],
        },
      ],
    )
  }
  return options
}

const updateSessionInBuckets = (
  sessionId: string,
  patch: Partial<{
    is_pinned: boolean
    pinned_at: string | null
    title: string
    isNoTitle?: boolean
  }>,
) => {
  const next: Record<string, SidebarSessionBucket> = {}
  for (const [key, bucket] of Object.entries(sessionBuckets.value)) {
    next[key] = {
      ...bucket,
      items: bucket.items.map((row) =>
        row.id === sessionId ? { ...row, ...patch } : row,
      ),
    }
  }
  sessionBuckets.value = next
  syncMenuStoreFromBuckets()
}

const renameSessionTitle = async (item: any, title: string) => {
  try {
    await renameSession(item.id, title, item.description || '')
    MessagePlugin.success(t('menu.renameSessionSuccess'))
  } catch {
    MessagePlugin.error(t('menu.renameSessionFailed'))
  }
}

const togglePin = (item: any, pin: boolean) => {
  if (pinningIds.value.has(item.id)) return
  pinningIds.value.add(item.id)

  setSessionPinned(item.id, pin)
    .catch(() => {
      MessagePlugin.error(pin ? t('menu.pinFailed') : t('menu.unpinFailed'))
    })
    .finally(() => {
      pinningIds.value.delete(item.id)
    })
}

const clearMessages = (item: any) => {
  clearSession(item.id)
    .then(() => {
      MessagePlugin.success(t('menu.clearMessagesSuccess'))
    })
    .catch(() => {
      MessagePlugin.error(t('menu.clearMessagesFailed'))
    })
}

const delCard = (item: any) => {
  removeSession(item.id).catch(() =>
    MessagePlugin.error(t('chat.deleteSessionFailed')),
  )
}

const debounce = (fn: (...args: any[]) => void, delay: number) => {
  let timer: ReturnType<typeof setTimeout>
  return (...args: any[]) => {
    clearTimeout(timer)
    timer = setTimeout(() => fn(...args), delay)
  }
}
const mapSessionRow = (item: any) => ({
  title: item.title ? item.title : t('menu.newSession'),
  path: `chat/${item.id}`,
  id: item.id,
  isMore: false,
  isNoTitle: item.title ? false : true,
  created_at: item.created_at,
  updated_at: item.updated_at,
  is_pinned: !!item.is_pinned,
  pinned_at: item.pinned_at || null,
  im_platform: item.im_platform || '',
  description: item.description || '',
  user_id: item.user_id || '',
  parent_session_id: item.parent_session_id || '',
  project_id: item.project_id || '',
  project_folder_id: item.project_folder_id || '',
})

const syncMenuStoreFromBuckets = () => {
  usemenuStore.clearMenuArr()
  const flat = flattenBucketItems(sessionBuckets.value, bucketOrder.value)
  flat.forEach((item) => usemenuStore.updatemenuArr(item))
  total.value = flat.length
}

const menuChildToSessionRow = (
  item: Record<string, unknown>,
): SessionForGrouping & { path: string } => {
  const id = String(item.id)
  return {
    id,
    path: typeof item.path === 'string' ? item.path : sessionPath(id),
    title: typeof item.title === 'string' ? item.title : undefined,
    is_pinned: !!item.is_pinned,
    created_at:
      typeof item.created_at === 'string' ? item.created_at : undefined,
    updated_at:
      typeof item.updated_at === 'string' ? item.updated_at : undefined,
    im_platform: typeof item.im_platform === 'string' ? item.im_platform : '',
    description: typeof item.description === 'string' ? item.description : '',
    user_id: typeof item.user_id === 'string' ? item.user_id : '',
    parent_session_id:
      typeof item.parent_session_id === 'string' ? item.parent_session_id : '',
    project_id: typeof item.project_id === 'string' ? item.project_id : '',
    project_folder_id:
      typeof item.project_folder_id === 'string' ? item.project_folder_id : '',
  }
}

const sessionExistsInBuckets = (sessionId: string) =>
  Object.values(sessionBuckets.value).some((bucket) =>
    bucket.items.some((row) => row.id === sessionId),
  )

/** 创建会话后 menuStore 已乐观写入，但列表实际渲染自 sessionBuckets，需补齐。 */
const ensureSessionInSidebar = (sessionId: string) => {
  if (!sessionId || sessionExistsInBuckets(sessionId)) return

  const web = sessionBuckets.value.web
  if (!web) return

  const chatMenu = (menuArr.value as unknown as MenuItem[]).find(
    (item) => item.path === 'creatChat',
  )
  const fromStore = (
    chatMenu?.children as Record<string, unknown>[] | undefined
  )?.find((item) => item.id === sessionId)
  if (!fromStore) return

  sessionBuckets.value = {
    ...sessionBuckets.value,
    web: prependSessionToWebBucket(web, menuChildToSessionRow(fromStore)),
  }
  total.value = flattenBucketItems(
    sessionBuckets.value,
    bucketOrder.value,
  ).length
}

const rebuildBucketDefinitions = () =>
  buildBucketDefinitions(
    imPlatforms.value,
    embedChannelNames.value,
    {
      web: t('menu.myChats'),
      imPlatform: (platform) => t(`agentEditor.im.${platform}`),
      embedChannel: (name) => name,
      api: t('menu.apiChats'),
    },
    { includeAdminChannelBuckets: authStore.hasRole('admin') },
  )

/** 首屏轻量探测各渠道是否有会话（page_size=1 只取 total），避免展示空文件夹 */
const probeChannelBucketCounts = async (keys: string[], token: number) => {
  const targets = keys.filter((key) => isChannelBucketKey(key))
  await Promise.all(
    targets.map(async (key) => {
      const bucket = sessionBuckets.value[key]
      if (!bucket) return
      try {
        const res: any = await getSessionsList(1, 1, bucket.apiSource, 'chat')
        if (token !== bucketRequestToken) return
        sessionBuckets.value = {
          ...sessionBuckets.value,
          [key]: applyBucketCountProbe(bucket, res?.total ?? 0),
        }
      } catch {
        if (token !== bucketRequestToken) return
        sessionBuckets.value = {
          ...sessionBuckets.value,
          [key]: applyBucketCountProbe(bucket, 0),
        }
      }
    }),
  )
}

const loadBucketPage = async (key: string, page?: number, token?: number) => {
  const activeToken = token ?? bucketRequestToken
  const bucket = sessionBuckets.value[key]
  if (!bucket || bucket.loading) return

  const nextPage = page ?? bucket.page + 1
  sessionBuckets.value = {
    ...sessionBuckets.value,
    [key]: { ...bucket, loading: true },
  }

  try {
    const res: any = await getSessionsList(
      nextPage,
      SIDEBAR_BUCKET_PAGE_SIZE,
      bucket.apiSource,
      'chat',
    )
    if (activeToken !== bucketRequestToken) return
    const rows = (res?.data || []).map((item: any) => mapSessionRow(item))
    const current = sessionBuckets.value[key]
    sessionBuckets.value = {
      ...sessionBuckets.value,
      [key]: mergeBucketPage(
        current,
        rows,
        res?.total ?? rows.length,
        nextPage,
      ),
    }
    syncMenuStoreFromBuckets()
    await refreshSessionListScrollability()
  } catch {
    if (activeToken !== bucketRequestToken) return
    const current = sessionBuckets.value[key]
    sessionBuckets.value = {
      ...sessionBuckets.value,
      [key]: { ...current, loading: false, loaded: true },
    }
  }
}

const switchSessionBucket = async (key: string) => {
  if (key === activeSessionBucketKey.value) return
  activeSessionBucketKey.value = key
  const bucket = sessionBuckets.value[key]
  if (bucket && !bucket.loaded && !bucket.loading) {
    await loadBucketPage(key, 1)
  }
  await ensureBucketFillsViewport(key)
  await refreshSessionListScrollability()
}

const syncActiveBucketFromChat = async (sessionId: string | undefined) => {
  if (!sessionId) return

  let bucketKey = findSessionBucketKey(sessionBuckets.value, sessionId)
  if (!bucketKey) {
    const chatMenu = (menuArr.value as unknown as MenuItem[]).find(
      (item) => item.path === 'creatChat',
    )
    const fromStore = (
      chatMenu?.children as Record<string, unknown>[] | undefined
    )?.find((item) => item.id === sessionId)
    if (fromStore) {
      bucketKey = originGroupKey(
        resolveSessionOrigin(menuChildToSessionRow(fromStore)),
      )
    }
  }
  // On a hard refresh only the web bucket is loaded, so a session opened from
  // any other folder (IM, embed, or the admin-only API folder) isn't in any
  // bucket or the menu store. Fetch its detail and classify its origin folder
  // so the sidebar stays in sync with the chat pane instead of snapping back
  // to "my chats". Only switch when that folder is actually present.
  if (!bucketKey) {
    try {
      const res: any = await getSession(sessionId)
      const candidate = originGroupKey(
        resolveSessionOrigin({
          id: sessionId,
          im_platform: res?.data?.im_platform || '',
          description: res?.data?.description || '',
          user_id: res?.data?.user_id || '',
        }),
      )
      if (sessionBuckets.value[candidate]) {
        bucketKey = candidate
      }
    } catch {
      // Fall through: leave the default bucket active on lookup failure.
    }
  }
  if (!bucketKey || bucketKey === activeSessionBucketKey.value) return

  activeSessionBucketKey.value = bucketKey
  const bucket = sessionBuckets.value[bucketKey]
  if (bucket && !bucket.loaded && !bucket.loading) {
    await loadBucketPage(bucketKey, 1)
  }
}

const initSessionBuckets = async () => {
  const token = ++bucketRequestToken
  sessionListBooting.value = true

  const defs = rebuildBucketDefinitions()
  bucketOrder.value = defs.map((def) => def.key)
  const buckets: Record<string, SidebarSessionBucket> = {}
  for (const def of defs) {
    buckets[def.key] = createEmptyBucket(def)
  }
  sessionBuckets.value = buckets

  // 首屏：拉 web 会话 + 轻量探测各渠道 count（不拉完整列表）；有会话的渠道才展示文件夹
  const channelKeys = defs
    .map((def) => def.key)
    .filter((key) => isChannelBucketKey(key))
  await Promise.all([
    loadBucketPage('web', 1, token),
    probeChannelBucketCounts(channelKeys, token),
  ])

  if (token === bucketRequestToken) {
    sessionListBooting.value = false
    syncMenuStoreFromBuckets()
    await ensureBucketFillsViewport('web')
    await refreshSessionListScrollability()
  }
}

const getMessageList = async () => {
  await initSessionBuckets()
}

// 滚动到底时为当前筛选来源加载下一页
const checkScrollBottom = async () => {
  const container = scrollContainer.value
  const key = activeSessionBucketKey.value
  const bucket = sessionBuckets.value[key]
  if (!container || !bucket || !bucketHasMore(bucket) || bucket.loading) return

  const { scrollTop, scrollHeight, clientHeight } = container
  const hasOverflow = scrollHeight > clientHeight + 1
  if (!hasOverflow) {
    await ensureBucketFillsViewport(key)
    return
  }

  const isNearBottom = scrollHeight - (scrollTop + clientHeight) < 100
  if (!isNearBottom) return

  await loadBucketPage(key)
}

const handleScroll = debounce(checkScrollBottom, 200)

async function loadCurrentKbInfo(kbId: string) {
  if (!kbId || !isInKnowledgeBase.value) {
    currentKbName.value = ''
    currentKbInfo.value = null
    return
  }
  const data = await chatResources.fetchKnowledgeBaseById(kbId)
  if (data) {
    currentKbName.value = data.name || ''
    currentKbInfo.value = data
  } else {
    currentKbInfo.value = null
  }
}

const loadSessionOriginMeta = async () => {
  try {
    const res: any = await listAllIMChannels()
    imPlatforms.value = configuredPlatforms(res?.data || [])
  } catch {
    imPlatforms.value = []
  }
  try {
    const res: any = await listAllEmbedChannels()
    const names: Record<string, string> = {}
    for (const ch of res?.data || []) {
      if (ch?.id && ch?.name) names[ch.id] = ch.name
    }
    embedChannelNames.value = names
  } catch {
    embedChannelNames.value = {}
  }
}

const handleSessionMutation = (event: Event) => {
  const detail = (event as CustomEvent<SessionMutationDetail>).detail
  if (!detail?.sessionId) return
  if (detail.removed || detail.messagesCleared)
    sessionActivity.dismiss(detail.sessionId)
  if (detail.patch) {
    updateSessionInBuckets(detail.sessionId, {
      ...detail.patch,
      ...(detail.patch.title ? { isNoTitle: false } : {}),
    })
  }
  if (detail.removed) {
    sessionBuckets.value = removeSessionFromBuckets(
      sessionBuckets.value,
      detail.sessionId,
    )
    syncMenuStoreFromBuckets()
    if (detail.sessionId === route.params.chatid) {
      router.push('/platform/creatChat')
    }
  }
}

onMounted(async () => {
  sessionActivity.restore()
  void sessionActivity.refresh()
  sessionActivityTimer = setInterval(() => {
    void sessionActivity.refresh()
  }, 5000)
  const routeName =
    typeof route.name === 'string'
      ? route.name
      : route.name
        ? String(route.name)
        : ''
  currentpath.value = routeName
  if (route.params.chatid) {
    currentSecondpath.value = `chat/${route.params.chatid}`
  }

  window.addEventListener(SESSION_MUTATION_EVENT, handleSessionMutation)

  isLiteEdition.value = authStore.isLiteMode
  editorResources
    .ensureSystemInfo()
    .then(() => {
      if (editorResources.systemInfo?.edition === 'lite') {
        isLiteEdition.value = true
        authStore.setLiteMode(true)
      }
    })
    .catch(() => {})

  await loadCurrentKbInfo((route.params as any)?.kbId as string)

  await loadSessionOriginMeta()
  await getMessageList()
  const initialChatId = route.params.chatid as string | undefined
  if (initialChatId) {
    ensureSessionInSidebar(initialChatId)
    await syncActiveBucketFromChat(initialChatId)
  }
  // 若组织列表未加载则拉取一次，用于侧栏「待审批」角标
  if (
    deploymentCapabilities.isSupported('organizations') &&
    orgStore.organizations.length === 0
  ) {
    orgStore.fetchOrganizations()
  }
})

onUnmounted(() => {
  clearInterval(sessionActivityTimer)
  clearTimeout(forkRevealTimer)
  sessionActivity.clear()
  window.removeEventListener(SESSION_MUTATION_EVENT, handleSessionMutation)
})

watch([() => route.name, () => route.params], (newvalue, oldvalue) => {
  const nameStr =
    typeof newvalue[0] === 'string'
      ? (newvalue[0] as string)
      : newvalue[0]
        ? String(newvalue[0])
        : ''
  currentpath.value = nameStr
  if (newvalue[1].chatid) {
    currentSecondpath.value = `chat/${newvalue[1].chatid}`
  } else {
    currentSecondpath.value = ''
  }

  // 创建新会话时 creatChat 会先 updataMenuChildren，再跳转 chat/:id。
  // 侧栏实际渲染 sessionBuckets，需按 buckets 判断是否缺失，不能把 menuStore 当真相来源。
  const newChatId = (newvalue[1] as any)?.chatid as string | undefined
  if (nameStr === 'chat' && newChatId) {
    ensureSessionInSidebar(newChatId)
    void syncActiveBucketFromChat(newChatId)
  }

  // 路由变化时更新图标状态和知识库信息（不涉及对话列表）
  getIcon(nameStr)

  // 如果切换了知识库，更新知识库名称但不重新加载对话列表
  if (newvalue[1].kbId !== oldvalue?.[1]?.kbId) {
    loadCurrentKbInfo((newvalue[1] as any)?.kbId as string)
  }
})
let knowledgeIcon = ref('zhishiku.svg')
let prefixIcon = ref('prefixIcon.svg')
let logoutIcon = ref('logout.svg')
let settingIcon = ref('setting.svg')
let agentIcon = ref('agent.svg')
let artifactIcon = ref('artifact.svg')
let programmingIcon = ref('prefixIcon.svg')
let toolboxIcon = ref('toolbox.svg')
let organizationIcon = ref('organization.svg')
let pathPrefix = ref(route.name)
const getIcon = (path: string) => {
  // 根据当前路由状态更新所有图标
  const kbActiveState = getIconActiveState('knowledge-bases')
  const creatChatActiveState = getIconActiveState('creatChat')
  const settingsActiveState = getIconActiveState('settings')
  const agentsActiveState = route.name === 'agentList'
  const artifactsActiveState = route.name === 'artifactLibrary'
  const organizationsActiveState = route.name === 'organizationList'

  // All icons use default variants (no green)
  knowledgeIcon.value = 'zhishiku.svg'
  agentIcon.value = 'agent.svg'
  artifactIcon.value = 'artifact.svg'
  programmingIcon.value = 'prefixIcon.svg'
  toolboxIcon.value = 'toolbox.svg'
  organizationIcon.value = 'organization.svg'
  prefixIcon.value = 'prefixIcon.svg'
  settingIcon.value = 'setting.svg'

  // 退出图标：始终显示默认
  logoutIcon.value = 'logout.svg'
}
getIcon(
  typeof route.name === 'string'
    ? (route.name as string)
    : route.name
      ? String(route.name)
      : '',
)
const handleMenuClick = async (path: string) => {
  if (path === 'knowledge-bases') {
    // 知识库菜单项：如果在知识库内部，跳转到当前知识库文件页；否则跳转到知识库列表
    const kbId = await getCurrentKbId()
    if (kbId) {
      router.push(`/platform/knowledge-bases/${kbId}`)
    } else {
      router.push('/platform/knowledge-bases')
    }
  } else if (path === 'agents') {
    router.push('/platform/agents')
  } else if (path === 'organizations') {
    // 组织菜单项：跳转到组织列表
    router.push('/platform/organizations')
  } else if (path === 'settings') {
    // 设置菜单项：打开设置弹窗并跳转路由
    uiStore.openSettings()
    router.push('/platform/settings')
  } else {
    gotopage(path)
  }
}

// 处理退出登录确认
const handleLogout = () => {
  gotopage('logout')
}

const getCurrentKbId = async (): Promise<string | null> => {
  const kbId = (route.params as any)?.kbId as string
  if (isInKnowledgeBase.value && kbId) {
    return kbId
  }
  return null
}

const gotopage = async (path: string) => {
  pathPrefix.value = path
  // 处理退出登录
  if (path === 'logout') {
    try {
      // 调用后端API注销
      await logoutApi()
    } catch (error) {
      // 即使API调用失败，也继续执行本地清理
      console.error('注销API调用失败:', error)
    }
    // 清理所有状态和本地存储
    authStore.logout()
    MessagePlugin.success(t('menu.logoutSuccess'))
    router.push('/login')
    return
  } else {
    if (path === 'creatChat') {
      // 如果在知识库详情页，跳转到全局对话创建页
      if (isInKnowledgeBase.value) {
        router.push('/platform/creatChat')
      } else {
        // 如果不在知识库内，进入对话创建页
        router.push(`/platform/creatChat`)
      }
    } else {
      router.push(`/platform/${path}`)
    }
  }
  getIcon(path)
}

const getImgSrc = (url: string) => {
  return new URL(`/src/assets/img/${url}`, import.meta.url).href
}

const mouseenteMenu = (path: string) => {}
const mouseleaveMenu = (path: string) => {}

let sidebarResizeStartWidth = 0
const startSidebarResize = () => {
  sidebarResizeStartWidth = uiStore.sidebarDisplayWidth
  uiStore.sidebarResizing = true
}
const resizeSidebar = (delta: number, keyboard: boolean) => {
  if (keyboard && uiStore.sidebarCollapsed && delta > 0) {
    uiStore.expandSidebar()
  } else if (
    keyboard &&
    uiStore.sidebarWidth === SIDEBAR_MIN_WIDTH &&
    delta < 0
  ) {
    uiStore.collapseSidebar()
  } else {
    uiStore.resizeSidebar(sidebarResizeStartWidth + delta)
  }
}
</script>
<style lang="less" scoped>
.aside_box {
  // 侧栏水平栅格：图标列与文案列统一对齐（Logo / 菜单 / 会话分组 / 会话行）
  --sidebar-inset-x: 10px;
  --sidebar-icon-size: 18px;
  --sidebar-channel-icon: 14px;
  --sidebar-icon-gap: 8px;
  --sidebar-text-inset: calc(
    var(--sidebar-inset-x) + var(--sidebar-icon-size) + var(--sidebar-icon-gap)
  ); // 36px

  min-width: 0;
  width: var(--sidebar-width, 260px);
  flex-shrink: 0;
  padding: 8px 4px 6px;
  background: var(--td-bg-color-sidebar);
  box-sizing: border-box;
  /* Avoid 100vh because <html> carries a `zoom` multiplier for font-size
       control; 100vh is evaluated against the unscaled viewport and then
       scaled, so at "large" the sidebar would extend past the window. The
       ancestor chain (html/body/#app/.main) is already height: 100%. */
  height: 100%;
  overflow: visible;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--td-component-stroke);
  box-shadow: 1px 0 0
    color-mix(in srgb, var(--td-text-color-primary) 2%, transparent);
  transition:
    width var(--app-motion-base) ease,
    min-width 0.25s ease;
  position: relative;

  // macOS Wails 桌面：红绿灯位于 HiddenInset 标题栏区域，需让出顶部空间
  html.wails-desktop & {
    padding-top: 30px;
  }

  &--resizing {
    transition: none;
  }

  &.aside_box--collapsed {
    min-width: 48px;
    width: 48px;
    padding: 8px 0;
    overflow: visible;

    .menu_box {
      align-items: center;
    }

    .menu_item {
      justify-content: center;
      width: 32px;
      height: 32px;
      padding: 0;
      margin: 0 auto 6px;
      border-radius: var(--app-radius-sm);

      .menu_item-box {
        justify-content: center;
        width: auto;
      }

      .menu_icon {
        margin-right: 0;
        justify-content: center;
      }
    }

    .menu_item_active {
      border-radius: var(--app-radius-sm);
    }

    .menu_bottom {
      align-items: center;
    }

    .menu_top {
      margin-right: 0;
      padding-right: 0;
    }
  }

  .logo_row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    // 保留侧栏顶部留白，让 Logo 和操作按钮与窗口顶边保持舒适间距
    height: 39px;
    margin: 0 0 4px;
    flex-shrink: 0;
    padding: 0 2px 0 var(--sidebar-inset-x);
  }

  .sidebar-toggle {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 28px;
    height: 28px;
    flex-shrink: 0;
    cursor: pointer;
    color: var(--td-text-color-secondary);
    border-radius: var(--app-radius-xs);
    transition: background-color var(--app-motion-base) ease;
    box-sizing: border-box;

    &:hover {
      background: var(--td-bg-color-container-hover);
      color: var(--td-text-color-primary);
    }
  }

  .logo_box {
    .logo { width: 128px; height: auto; }
    display: flex;
    align-items: center;
    gap: 8px;
    flex: 1;
    min-width: 0;
    // 不能裁切：品牌标记带 -12° 旋转，包围盒比布局盒宽/高约 2px，
    // overflow:hidden 会把它的左缘和上下缘削平。文案改由省略号截断。
    overflow: visible;

    // 品牌标记沿用「新对话」首页的三道圆角短线，用 currentColor 跟随主题，
    // 侧栏因此不再依赖位图 logo，也不必再做深色模式的反转滤镜。
    .brand-mark {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 2.5px;
      height: 18px;
      flex-shrink: 0;
      // 右移一点：抵消旋转带来的左侧外扩，与下方菜单图标列对齐，
      // 同时避免标记贴住侧栏左缘显得被“切掉”。
      margin-left: 2px;
      color: var(--td-text-color-primary);
      transform: rotate(-12deg);

      > span {
        display: block;
        width: 3.5px;
        background: currentColor;
        border-radius: var(--app-radius-pill);

        &:nth-child(1) {
          height: 12px;
        }

        &:nth-child(2) {
          height: 18px;
        }

        &:last-child {
          height: 9px;
          align-self: flex-end;
        }
      }
    }

    // 与标记并排的文字字标，模仿 Manus 侧栏的紧凑品牌锁定。
    .brand-wordmark {
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      font-size: var(--app-text-lg);
      font-weight: 600;
      line-height: 1;
      letter-spacing: -0.2px;
      color: var(--td-text-color-primary);
      white-space: nowrap;
      user-select: none;
    }

    .lite-badge {
      margin-left: 2px;
      align-self: flex-start;
      margin-top: 2px;
      font-size: var(--app-text-2xs);
      font-weight: 600;
      color: var(--td-text-color-placeholder);
      user-select: none;
      white-space: nowrap;
    }
  }

  .menu_top {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow-y: auto;
    overflow-x: hidden;
    min-height: 0;
    // 抵消 .aside_box 的右内边距，让滚动条贴近面板右缘；
    // 等量 padding 补回，保证列表文字位置不变。
    margin-right: -4px;
    padding-right: 4px;

    // Claude 风格细滚动条：默认透明，悬浮时显示一条圆角细灰条
    scrollbar-width: thin;
    scrollbar-color: transparent transparent;
    transition: scrollbar-color var(--app-motion-base) ease;

    &::-webkit-scrollbar {
      width: 6px;
    }

    &::-webkit-scrollbar-track {
      background: transparent;
    }

    &::-webkit-scrollbar-thumb {
      background-color: transparent;
      border-radius: var(--app-radius-sm);
      transition: background-color var(--app-motion-base) ease;
    }

    &:hover {
      scrollbar-color: var(--td-scrollbar-color) transparent;

      &::-webkit-scrollbar-thumb {
        background-color: var(--td-scrollbar-color);
      }
    }

    &::-webkit-scrollbar-thumb:hover {
      background-color: var(--td-scrollbar-hover-color);
    }
  }

  .menu_bottom {
    flex-shrink: 0;
    display: flex;
    flex-direction: column;
  }

  .menu_box {
    display: flex;
    flex-direction: column;

    // 「新对话」吸顶：作为滚动容器(.menu_top)的直接子级，滚动时钉在顶部，
    // 知识库/智能体/共享空间及历史列表一起从其下方滚走。背景遮挡滚动内容。
    &--sticky {
      position: sticky;
      top: 0;
      z-index: 2;
      background: var(--td-bg-color-sidebar);
    }
  }

  .active-upload {
    color: var(--td-text-color-primary);
  }

  .menu_item_active {
    border-radius: var(--app-radius-xs);
    background: var(--td-bg-color-secondarycontainer) !important;

    .menu_icon,
    .menu_title {
      color: var(--td-text-color-primary) !important;
    }
  }

  .menu_item_c_active {
    .menu_icon,
    .menu_title {
      color: var(--td-text-color-primary);
    }
  }

  .menu_item {
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 34px;
    padding: 6px 8px 6px var(--sidebar-inset-x);
    box-sizing: border-box;
    margin-bottom: 1px;
    border-radius: var(--app-radius-xs);
    transition: background-color var(--app-motion-base) ease;

    .menu_item-box {
      display: flex;
      align-items: center;
    }

    &:hover {
      background: var(--td-bg-color-container-hover);

      .menu_icon,
      .menu_title {
        color: var(--td-text-color-primary);
      }
    }
  }

  .menu_icon {
    display: flex;
    align-items: center;
    justify-content: center;
    flex: 0 0 var(--sidebar-icon-size);
    width: var(--sidebar-icon-size);
    height: var(--sidebar-icon-size);
    margin-right: var(--sidebar-icon-gap);
    color: var(--td-text-color-secondary);

    .icon {
      width: 18px;
      height: 18px;
      overflow: hidden;
    }

    // 图形几乎占满画布的图标缩小显示，否则会比其他图标大一圈
    &.menu_icon--zhishiku .icon,
    &.menu_icon--organization .icon,
    .icon.icon--toggle {
      width: 16px;
      height: 16px;
    }
  }

  // 折叠态侧栏里的主题切换：按钮比常规图标宽，居中摆放并去掉图标间距
  .menu_icon--theme-toggle {
    align-items: center;
    justify-content: center;
    margin-right: 0;
  }

  .menu_title {
    color: var(--td-text-color-primary);
    text-overflow: ellipsis;
    font-family: var(--app-font-family);
    font-size: var(--app-text-base);
    font-style: normal;
    font-weight: 500;
    line-height: 20px;
    overflow: hidden;
    white-space: nowrap;
    max-width: 120px;
    flex: 1;
  }

  .submenu {
    position: relative;
    font-family: var(--app-font-family);
    font-size: var(--app-text-base);
    font-style: normal;
    min-width: 0;
    padding-top: 3px;
  }

  :deep(.submenu_pin_icon) {
    color: inherit;
    font-size: var(--app-text-sm);
    margin-right: 4px;
    vertical-align: middle;
    flex-shrink: 0;
  }

  .submenu_source_icon {
    width: 14px;
    height: 14px;
    margin-right: 0px;
    vertical-align: middle;
    object-fit: contain;
    flex-shrink: 0;
    // 默认淡化处理，避免未选中状态下彩色图标与灰色标题不协调；
    // 悬浮或选中时恢复彩色，交互时才引人注意。
    filter: grayscale(1);
    opacity: 0.55;
    transition:
      filter var(--app-motion-fast) ease,
      opacity var(--app-motion-fast) ease;
  }

  :deep(.submenu_item:hover .submenu_source_icon),
  :deep(.submenu_item_active .submenu_source_icon) {
    filter: none;
    opacity: 1;
  }

  // 列表行统一栅格：左缘 inset-x + 图标槽 18px + 间距 8px → 文案列与主菜单文字对齐
  .session-list-row {
    display: flex;
    align-items: center;
    gap: var(--sidebar-icon-gap);
    padding: 0 8px 0 var(--sidebar-inset-x);
    min-width: 0;
    box-sizing: border-box;
  }

  .session-list-row__icon {
    flex: 0 0 var(--sidebar-icon-size);
    width: var(--sidebar-icon-size);
    height: var(--sidebar-icon-size);
    display: inline-flex;
    align-items: center;
    justify-content: center;
    flex-shrink: 0;
  }

  .session-list-row__body {
    flex: 1 1 auto;
    min-width: 0;
    overflow: hidden;
  }

  // 聊天区分组标题 / 会话行：与「聊天」节标题同列左对齐，不再预留图标槽
  .session-list-row--flat {
    padding-left: var(--sidebar-inset-x);
    gap: 0;
  }

  .session-list-loading {
    display: flex;
    align-items: center;
    min-height: 26px;
    color: var(--td-text-color-placeholder);
  }

  .timeline_header {
    font-family: var(--app-font-family);
    font-size: var(--app-text-xs);
    font-weight: 600;
    color: var(--td-text-color-disabled);
    padding-top: 4px;
    padding-bottom: 1px;
    margin-top: 0;
    line-height: 16px;
    user-select: none;
  }

  .timeline_header-label {
    white-space: nowrap;
  }

  .session-folders-toolbar {
    min-height: 36px;
    margin: 4px;
    padding: 0 6px 0 10px;
    display: flex;
    align-items: center;
    gap: 6px;
    box-sizing: border-box;
    border-radius: var(--app-radius-md);
    background: transparent;

    &:hover {
      background: var(--td-bg-color-container-hover);
    }
  }

  .session-folders-heading {
    flex: 1 1 auto;
    min-width: 0;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--td-text-color-primary);
    font-size: var(--app-text-base);
    font-weight: 700;
    letter-spacing: -0.01em;
  }

  button.session-folders-heading {
    justify-content: flex-start;
    padding: 0;
    border: 0;
    background: transparent;
    font-family: var(--app-font-family);
    text-align: left;
    cursor: pointer;
    color: #737373;
    font-size: 13px;

    &:focus-visible {
      outline: none;
    }
  }

  .session-folders-heading :deep(.session-source-filter--inline) {
    max-width: 100%;
  }

  .session-folders-header-actions {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    flex: 0 0 auto;
  }

  .session-folders-header-actions :deep(.session-source-filter__trigger) {
    min-height: 28px;
    padding: 0 4px;
    color: var(--td-text-color-secondary);
  }

  .session-folders-header-actions :deep(.session-source-filter__label) {
    max-width: 72px;
    font-size: var(--app-text-2xs);
  }

  .session-folders-header-actions :deep(.t-icon),
  .session-folder-actions :deep(.t-icon) {
    font-size: 17px;
    font-weight: 700;
    filter: drop-shadow(0 0 0.3px currentColor);
  }

  .session-folders-header-actions :deep(svg),
  .session-folder-actions :deep(svg) {
    stroke-width: 2.25px;
  }

  .session-folder-more-icon {
    transform: rotate(90deg);
  }

  .session-folder-create,
  .session-folder-actions,
  .session-folder-toggle {
    display: inline-flex;
    align-items: center;
  }

  .session-folder-create {
    width: 26px;
    height: 26px;
    justify-content: center;
    padding: 0;
    border: 0;
    background: transparent;
    color: var(--td-text-color-secondary);
    border-radius: var(--app-radius-xs);
    cursor: pointer;
    transition:
      background var(--app-motion-fast) ease,
      color var(--app-motion-fast) ease;

    &:hover {
      background: var(--td-bg-color-container-hover);
      color: var(--td-text-color-primary);
    }
    &:active {
      transform: scale(0.96);
    }
    &:focus-visible {
      outline: 2px solid var(--td-brand-color);
      outline-offset: 1px;
    }
    &:disabled {
      opacity: 0.45;
      cursor: default;
    }
  }

  .session-folder-toolbar-icon-button {
    opacity: 0;
    pointer-events: none;
    transition: opacity var(--app-motion-fast) ease;
  }

  .session-folders-toolbar:hover .session-folder-toolbar-icon-button,
  .session-folders-toolbar .session-folder-toolbar-icon-button:focus-visible {
    opacity: 1;
    pointer-events: auto;
  }

  @media (hover: none) {
    .session-folder-toolbar-icon-button {
      opacity: 1;
      pointer-events: auto;
    }
  }

  .session-folder-section {
    min-width: 0;
    margin-bottom: 2px;
  }

  .session-folder-content {
    display: grid;
    grid-template-rows: 1fr;
    overflow: hidden;
    opacity: 1;
    margin-top: 1px;
  }

  .session-folder-content__inner {
    min-height: 0;
    overflow: hidden;
    transform: translateY(0);
  }

  .session-folder-content-enter-active,
  .session-folder-content-leave-active {
    display: grid;
    overflow: hidden;
    transition:
      grid-template-rows 220ms cubic-bezier(0.2, 0, 0, 1),
      opacity 160ms ease;
  }

  .session-folder-content-enter-from,
  .session-folder-content-leave-to {
    grid-template-rows: 0fr;
    opacity: 0;
  }

  .session-folder-content-enter-to,
  .session-folder-content-leave-from {
    grid-template-rows: 1fr;
    opacity: 1;
  }

  .session-folder-content-enter-active .session-folder-content__inner,
  .session-folder-content-leave-active .session-folder-content__inner {
    transition: transform 180ms ease;
  }

  .session-folder-content-enter-from .session-folder-content__inner,
  .session-folder-content-leave-to .session-folder-content__inner {
    transform: translateY(-4px);
  }

  .session-folder-header {
    min-height: 34px;
    display: flex;
    align-items: center;
    margin: 0 4px;
    padding: 0 8px 0 10px;
    gap: 6px;
    color: var(--td-text-color-secondary);
    border-radius: var(--app-radius-sm);
    background: var(--td-bg-color-secondarycontainer);
    transition: background var(--app-motion-fast) ease;

    &:hover {
      background: var(--td-bg-color-secondarycontainer-hover);
    }
  }

  .session-folder-toggle {
    min-width: 0;
    flex: 1 1 auto;
    gap: 9px;
    padding: 5px 0;
    border: 0;
    background: transparent;
    color: inherit;
    text-align: left;
    cursor: pointer;
    font: inherit;
    font-size: var(--app-text-base);

    &:focus-visible {
      outline: 2px solid var(--td-brand-color);
      outline-offset: 2px;
    }
  }

  .session-folder-toggle-icon {
    width: 18px;
    height: 18px;
    flex: 0 0 18px;
    display: grid;
    place-items: center;
  }

  .session-folder-toggle-icon :deep(.session-folder-default-icon),
  .session-folder-toggle-icon :deep(.session-folder-hover-chevron) {
    grid-area: 1 / 1;
    transition: opacity var(--app-motion-fast) ease;
  }

  .session-folder-toggle-icon :deep(.session-folder-hover-chevron) {
    opacity: 0;
  }

  .session-folder-header:hover
    .session-folder-toggle-icon
    :deep(.session-folder-default-icon) {
    opacity: 0;
  }

  .session-folder-header:hover
    .session-folder-toggle-icon
    :deep(.session-folder-hover-chevron) {
    opacity: 1;
  }

  .session-folder-name {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    color: var(--td-text-color-primary);
    font-size: var(--app-text-base);
    font-weight: 500;
    letter-spacing: -0.01em;
  }

  .session-folder-actions {
    gap: 2px;
    opacity: 0;
    transition: opacity var(--app-motion-fast) ease;
    flex: 0 0 auto;
  }
  .session-folder-header:hover .session-folder-actions,
  .session-folder-header:has(:focus-visible) .session-folder-actions {
    opacity: 1;
  }

  .session-folder-actions button {
    width: 26px;
    height: 26px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    border: 0;
    border-radius: var(--app-radius-xs);
    background: transparent;
    color: var(--td-text-color-secondary);
    cursor: pointer;
  }

  .session-folder-actions button:hover {
    background: var(--td-bg-color-container-hover);
    color: var(--td-text-color-primary);
  }

  .session-folder-empty {
    padding: 12px calc(var(--sidebar-inset-x) + 25px);
    color: var(--td-text-color-placeholder);
    font-size: var(--app-text-sm);
  }

  .session-folder-create-dialog-body {
    padding-top: 6px;
  }

  .session-folder-create-dialog-body :deep(.t-input) {
    width: 100%;
  }

  .submenu_item_p {
    padding: 0;
    box-sizing: border-box;
    min-width: 0;
    overflow: hidden;

    &.session-chat-row {
      margin-bottom: 1px;
    }

    &.session-chat-row .session-list-row {
      height: 34px;
      padding-right: 4px;
      border-radius: var(--app-radius-sm);
      transition:
        background var(--app-motion-fast) ease,
        color var(--app-motion-fast) ease;
    }

    &.session-folder-chat-row .session-list-row {
      min-height: 34px;
      margin: 0 4px;
      padding-left: 0;
      /* 右内边距交给内层 .submenu_item 的 8px，使「…」与文件夹标题栏右边线对齐。 */
      padding-right: 0;
      border-radius: var(--app-radius-sm);
    }

    &.session-folder-chat-row :deep(.submenu_item) {
      box-sizing: border-box;
      width: 100%;
      padding: 7px 8px 7px 36px;
    }

    &.session-chat-row--revealed {
      animation: session-fork-enter 280ms ease-out both;
    }

    &.session-chat-row:hover .session-list-row {
      background: var(--td-bg-color-container-hover);

      :deep(.menu-more) {
        color: var(--td-text-color-primary);
      }
    }

    &.session-chat-row--active .session-list-row {
      background: transparent;

      :deep(.submenu_item) {
        color: var(--td-text-color-primary);
      }

      :deep(.menu-more) {
        color: var(--td-text-color-primary);
      }
    }

    &.session-chat-row--active:hover .session-list-row {
      background: var(--td-bg-color-container-hover);
    }

    &.session-chat-row--selected .session-list-row {
      background: var(--td-bg-color-container-hover);
    }
  }

  // SessionSidebarRow 为子组件，需 :deep 才能让标题省略号生效
  :deep(.submenu_item) {
    cursor: pointer;
    display: flex;
    align-items: center;
    color: var(--td-text-color-primary);
    font-weight: 400;
    font-size: var(--app-text-base);
    line-height: 20px;
    height: 100%;
    width: 100%;
    padding: 0;
    position: relative;
    min-width: 0;
    background: transparent;

    .submenu_title {
      display: flex;
      align-items: center;
      flex: 1 1 auto;
      min-width: 0;
      overflow: hidden;
    }

    .submenu_title-text {
      flex: 1 1 auto;
      min-width: 0;
      overflow: hidden;
      white-space: nowrap;
      text-overflow: ellipsis;
    }

    .menu-more-wrap {
      transition: opacity var(--app-motion-base) ease;
      flex-shrink: 0;
    }

    .menu-more {
      display: inline-block;
      font-weight: bold;
      color: var(--td-text-color-secondary);
    }

    .submenu_title--batch {
      margin-left: 4px;
    }

    &.submenu_item_batch {
      padding-left: 0;
    }
  }

  :deep(.submenu_item_batch) {
    cursor: pointer;
    user-select: none;
  }

  .batch-checkbox {
    flex-shrink: 0;
  }
}

.batch-inline-footer {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 12px;
  border-top: 1px solid var(--td-component-stroke);
  background: var(--td-bg-color-container);

  .batch-footer-left {
    display: flex;
    align-items: center;
    font-size: var(--app-text-md);
    color: var(--td-text-color-placeholder);
  }

  .batch-footer-right {
    display: flex;
    align-items: center;
    gap: 6px;
  }
}

.menu_item-box {
  display: flex;
  align-items: center;
  width: 100%;
  position: relative;
}

/* Empty state when there are no sessions. */
.submenu_empty {
  padding: 24px 14px;
  text-align: center;
  font-size: var(--app-text-sm);
  color: var(--td-text-color-placeholder);
  user-select: none;
}

// 顶部 logo_row 右侧的图标按钮组（搜索 + 折叠），与折叠按钮风格一致
.logo_actions {
  display: flex;
  align-items: center;
  gap: 2px;
  flex-shrink: 0;
}

.header-icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  flex-shrink: 0;
  cursor: pointer;
  border-radius: var(--app-radius-sm);
  color: var(--td-text-color-secondary);
  transition: background-color var(--app-motion-base) ease;
  box-sizing: border-box;

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  .header-icon-img {
    width: 18px;
    height: 18px;
    display: block;
  }
}

// 深色 tooltip 内容：标签 + 浅灰快捷键内联
.cmdk-tip {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  white-space: nowrap;

  .cmdk-tip-label {
    font-size: var(--app-text-md);
  }

  .cmdk-tip-keys {
    font-size: var(--app-text-md);
    opacity: 0.6;
    letter-spacing: 0.5px;
  }
}

.menu-toolbox-stack {
  display: inline-flex;
  align-items: center;
  flex-shrink: 0;
  margin-left: auto;
}

.menu-toolbox-stack__item {
  position: relative;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  box-sizing: border-box;
  border: 1px solid var(--td-component-stroke);
  border-radius: 50%;
  background: var(--td-bg-color-container);
  color: var(--td-text-color-secondary);
  rotate: var(--stack-rotate, 0deg);
  --stack-spring: cubic-bezier(0.34, 1.56, 0.64, 1);
  animation: menu-toolbox-stack-in 420ms var(--stack-spring) both;
  animation-delay: var(--stack-delay, 0ms);
  transition:
    margin var(--app-motion-slow) var(--stack-spring),
    rotate var(--app-motion-slow) var(--stack-spring),
    translate var(--app-motion-slow) var(--stack-spring),
    color var(--app-motion-base) ease,
    box-shadow var(--app-motion-base) ease;
  transition-delay: var(--stack-delay, var(--app-motion-base));

  & + & {
    margin-left: -6px;
  }

  &:nth-child(1) {
    z-index: 3;
    --stack-rotate: -10deg;
  }

  &:nth-child(2) {
    z-index: 2;
    --stack-delay: 50ms;
  }

  &:nth-child(3) {
    z-index: 1;
    --stack-rotate: 10deg;
    --stack-delay: 100ms;
  }
}

.menu-toolbox-stack__status {
  position: absolute;
  right: -1px;
  bottom: -1px;
  width: 6px;
  height: 6px;
  border-radius: var(--app-radius-pill);
  box-shadow: 0 0 0 1.5px var(--td-bg-color-container);

  &.is-connected {
    background: var(--td-success-color);
  }

  &.is-offline {
    background: var(--td-warning-color);
  }
}

@keyframes menu-toolbox-stack-in {
  from {
    opacity: 0;
    scale: 0.4;
  }
}

.menu_item:hover .menu-toolbox-stack__item {
  color: var(--td-text-color-primary);
  rotate: 0deg;
  translate: 0 -1px;
  box-shadow: 0 2px 6px
    color-mix(in srgb, var(--td-text-color-primary) 8%, transparent);
}

.menu_item:hover .menu-toolbox-stack__item + .menu-toolbox-stack__item {
  margin-left: 3px;
}

@media (prefers-reduced-motion: reduce) {
  .menu-toolbox-stack__item {
    animation: none;
    transition: color var(--app-motion-base) ease;
  }
}

.menu-pending-badge {
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  margin-left: 6px;
  border-radius: var(--app-radius-lg);
  background: color-mix(in srgb, var(--td-warning-color) 20%, transparent);
  color: var(--td-warning-color);
  font-size: var(--app-text-sm);
  font-weight: 600;
  line-height: 18px;
  text-align: center;
  flex-shrink: 0;
}

.menu_box {
  position: relative;
}

@keyframes session-fork-enter {
  from {
    opacity: 0;
    transform: translateX(-10px);
  }

  to {
    opacity: 1;
    transform: translateX(0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .aside_box .submenu_item_p.session-chat-row--revealed {
    animation: none;
  }

  .session-folder-content-enter-active,
  .session-folder-content-leave-active,
  .session-folder-content-enter-active .session-folder-content__inner,
  .session-folder-content-leave-active .session-folder-content__inner {
    transition: none;
  }
}
</style>
<style lang="less">
// Dark mode: 滚动条在深色背景下需要更亮的颜色才看得见
.card-more .card-menu-item:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.card-more .card-menu-item.session-folder-menu-delete {
  margin-top: 0;
  color: var(--td-error-color-6);
}

.card-more .card-menu-item.session-folder-menu-delete .icon {
  color: var(--td-error-color-6);
}

.card-more .card-menu-item.session-folder-menu-delete:hover {
  background: var(--td-error-color-1);
  color: var(--td-error-color-6);
}

.card-more .card-menu-item.session-folder-menu-delete:hover .icon {
  color: var(--td-error-color-6);
}

html[theme-mode='dark'] .aside_box .menu_top:hover {
  scrollbar-color: rgba(255, 255, 255, 0.22) transparent;
}

html[theme-mode='dark'] .aside_box .menu_top:hover::-webkit-scrollbar-thumb {
  background-color: rgba(255, 255, 255, 0.22);
}

html[theme-mode='dark'] .aside_box .menu_top::-webkit-scrollbar-thumb:hover {
  background-color: rgba(255, 255, 255, 0.38);
}

// Dark mode: invert the top search icon button image to match text color
html[theme-mode='dark'] .aside_box .header-icon-img {
  filter: invert(1);
  opacity: 0.55;
}

html[theme-mode='dark'] .aside_box .header-icon-btn:hover .header-icon-img {
  opacity: 0.9;
}

// 下拉菜单样式已统一至 @/assets/dropdown-menu.less

// 退出登录确认框样式
:deep(.t-popconfirm) {
  .t-popconfirm__content {
    background: var(--td-bg-color-container);
    border: 1px solid var(--td-component-stroke);
    border-radius: var(--app-radius-sm);
    box-shadow: var(--td-shadow-3);
    padding: 12px 16px;
    font-size: var(--app-text-base);
    color: var(--td-text-color-primary);
    max-width: 200px;
  }

  .t-popconfirm__arrow {
    border-bottom-color: var(--td-component-stroke);
  }

  .t-popconfirm__arrow::after {
    border-bottom-color: var(--td-bg-color-container);
  }

  .t-popconfirm__buttons {
    margin-top: 8px;
    display: flex;
    justify-content: flex-end;
    gap: 8px;
  }

  .t-button--variant-outline {
    border-color: var(--td-component-border);
    color: var(--td-text-color-secondary);
  }

  .t-button--theme-danger {
    background-color: var(--td-error-color);
    border-color: var(--td-error-color);
  }

  .t-button--theme-danger:hover {
    background-color: var(--td-error-color);
    border-color: var(--td-error-color);
  }
}
</style>
