<template>
  <section
    class="conversation-workspace"
    :aria-label="
      t(
        `workspace.${tab === 'source' ? 'source' : tab === 'preview' ? 'preview' : 'tests'}`,
      )
    "
  >
    <template v-if="tab !== 'tests'">
      <div class="workspace-body">
        <aside
          v-show="!showFileDiff"
          ref="treeRef"
          class="file-tree"
          :style="treeStyle"
          :aria-label="t('workspace.files')"
        >
          <div class="file-tree__scroll">
            <p
              v-if="busy && !treeRows.length"
              class="file-tree__hint"
              role="status"
            >
              {{ t("workspace.loading") }}
            </p>
            <p
              v-else-if="workspaceState === 'expired' && !treeRows.length"
              class="file-tree__hint"
              role="status"
            >
              {{ t("workspace.workspaceExpiredHint") }}
            </p>
            <p v-else-if="!treeRows.length" class="file-tree__hint">
              {{ t("workspace.noFiles") }}
            </p>
            <ul v-else class="file-tree__list">
              <li v-for="row in treeRows" :key="row.node.path">
                <button
                  type="button"
                  class="file-tree__row"
                  :class="{
                    'is-selected': file?.path === row.node.path,
                    'is-dir': row.node.kind === 'directory',
                  }"
                  :style="{ paddingLeft: `${10 + row.depth * 14}px` }"
                  :title="row.node.path"
                  :aria-expanded="
                    row.node.kind === 'directory' ? row.expanded : undefined
                  "
                  @click="activateNode(row.node)"
                >
                  <span class="file-tree__lead">
                    <t-icon
                      v-if="row.node.kind === 'directory'"
                      :name="row.expanded ? 'chevron-down' : 'chevron-right'"
                      size="14px"
                    />
                    <span
                      v-else-if="row.glyph.text"
                      class="file-tree__glyph"
                      aria-hidden="true"
                      >{{ row.glyph.text }}</span
                    >
                    <t-icon
                      v-else
                      :name="row.glyph.icon"
                      size="14px"
                      aria-hidden="true"
                    />
                  </span>
                  <span class="file-tree__name">{{ row.node.name }}</span>
                  <t-icon
                    v-if="row.loading"
                    name="loading"
                    size="13px"
                    class="spinning"
                  />
                </button>
              </li>
            </ul>
          </div>
          <!-- 拖拽分隔条：像调整整个面板一样，把宽度让给代码/差异视图。 -->
          <PanelResizeHandle
            edge="right"
            :label="t('workspace.resizeFiles')"
            :value="treeWidth || measuredTreeWidth || TREE_MIN_WIDTH"
            :min="TREE_MIN_WIDTH"
            :max="TREE_MAX_WIDTH"
            @start="startTreeResize"
            @resize="resizeTree"
            @end="endTreeResize"
          />
        </aside>
        <div class="workspace-main">
          <div v-if="file || hasChangesPanel" class="file-toolbar">
            <span class="file-toolbar__path" :title="file?.path">{{
              showFileDiff ? t("workspace.diff") : file?.path
            }}</span>
            <div
              v-if="showViewSwitch"
              class="file-view-switch"
              role="group"
              :aria-label="t('workspace.fileView')"
            >
              <button
                type="button"
                :aria-pressed="fileView === 'source'"
                @click="fileView = 'source'"
              >
                {{ t("workspace.source") }}
              </button>
              <button
                v-if="hasChangesPanel"
                type="button"
                :aria-pressed="fileView === 'diff'"
                @click="fileView = 'diff'"
              >
                {{ t("workspace.diff") }}
              </button>
              <button
                v-if="canPreviewFile"
                type="button"
                :aria-pressed="fileView === 'preview'"
                @click="fileView = 'preview'"
              >
                {{ t("workspace.preview") }}
              </button>
            </div>
            <button
              v-if="showFilePreview"
              type="button"
              :aria-pressed="mobilePreview"
              :title="
                t(mobilePreview ? 'workspace.desktop' : 'workspace.mobile')
              "
              :aria-label="
                t(mobilePreview ? 'workspace.desktop' : 'workspace.mobile')
              "
              @click="mobilePreview = !mobilePreview"
            >
              <t-icon :name="mobilePreview ? 'desktop' : 'mobile'" />
            </button>
            <button
              v-else-if="file && !imageSource && !showFileDiff"
              type="button"
              :title="t(copied ? 'workspace.copied' : 'workspace.copy')"
              :aria-label="t(copied ? 'workspace.copied' : 'workspace.copy')"
              @click="copySource"
            >
              <t-icon :name="copied ? 'check' : 'copy'" />
            </button>
            <button
              type="button"
              :title="t('workspace.refresh')"
              :aria-label="t('workspace.refresh')"
              @click="refresh"
            >
              <t-icon name="refresh" />
            </button>
          </div>
          <div
            v-if="showFileDiff && gitStatusLoading && gitRepos === null"
            class="remote-strip remote-strip--loading"
            role="status"
            :aria-label="t('workspace.gitMetadataLoading')"
          >
            <div class="remote-strip__row">
              <span class="remote-strip__placeholder" aria-hidden="true"><t-icon name="git-branch" size="14px" /></span>
              <span class="remote-strip__loading-text">{{ t('workspace.gitMetadataLoading') }}</span>
              <t-icon name="loading" size="13px" class="spinning" />
            </div>
          </div>
          <div v-if="error" class="workspace-notice" role="alert">
            <t-icon name="info-circle" /><span>{{ error }}</span
            ><button type="button" @click="refresh">
              {{ t("workspace.retry") }}
            </button>
          </div>
          <div
            v-if="showFileDiff"
            class="diff-file-list"
            :aria-label="t('workspace.diff')"
          >
            <div
              v-for="repo in gitRepos ?? []"
              :key="repo.root || '__root__'"
              class="repo-group"
            >
              <div v-if="multiRepo" class="repo-group__header">
                <t-icon name="folder" size="14px" />
                <span class="repo-group__name">{{ repo.name }}</span>
              </div>
              <div
                v-if="remoteVisible(repo.root) && remotes[repo.root]"
                class="remote-strip"
              >
                <div class="remote-strip__row">
                  <t-select
                    class="remote-strip__branch-select"
                    :value="`refs/heads/${remotes[repo.root]!.branch}`"
                    :disabled="
                      branchSwitching[repo.root] ||
                      commitPending[repo.root] ||
                      pushPending[repo.root] ||
                      pullPending[repo.root]
                    "
                    :loading="branchLoading[repo.root]"
                    :loading-text="t('workspace.branchesLoading')"
                    :placeholder="t('workspace.searchBranches')"
                    :empty="t('workspace.noMatchingBranches')"
                    :popup-props="{
                      overlayClassName: 'workspace-branch-popup',
                      overlayInnerStyle: {
                        minWidth: 'min(280px, calc(100vw - 32px))',
                        maxWidth: 'min(400px, calc(100vw - 32px))',
                      },
                    }"
                    :title="t('workspace.switchBranch')"
                    size="small"
                    filterable
                    @popup-visible-change="
                      (visible: boolean) => {
                        if (visible) void ensureBranches(repo.root);
                      }
                    "
                    @change="
                      (value: unknown) => switchBranch(repo.root, String(value))
                    "
                  >
                    <template #prefixIcon>
                      <t-icon name="git-branch" size="14px" />
                    </template>
                    <t-option-group :label="t('workspace.localBranches')">
                      <t-option
                        v-for="b in localBranchOptions(repo.root)"
                        :key="`local:${b}`"
                        :value="`refs/heads/${b}`"
                        :label="b"
                      >
                        <span class="branch-option">
                          <t-icon
                            name="git-branch"
                            size="14px"
                            class="branch-option__icon"
                          />
                          <span class="branch-option__name">{{ b }}</span>
                          <span
                            v-if="b === remotes[repo.root]!.branch"
                            class="branch-option__current"
                            >{{ t("workspace.currentBranch") }}</span
                          >
                        </span>
                      </t-option>
                    </t-option-group>
                    <t-option-group
                      v-if="remoteBranchOptions[repo.root]?.length"
                      :label="t('workspace.remoteBranches')"
                      divider
                    >
                      <t-option
                        v-for="b in remoteBranchOptions[repo.root] || []"
                        :key="`remote:${b}`"
                        :value="`refs/remotes/${b}`"
                        :label="b"
                      >
                        <span class="branch-option">
                          <t-icon
                            name="cloud"
                            size="14px"
                            class="branch-option__icon"
                          />
                          <span class="branch-option__name">{{ b }}</span>
                        </span>
                      </t-option>
                    </t-option-group>
                  </t-select>
                  <template v-if="remotes[repo.root]!.upstream">
                    <t-icon
                      name="arrow-right"
                      size="12px"
                      class="remote-strip__arrow"
                    />
                    <span
                      class="remote-strip__upstream"
                      :title="remotes[repo.root]!.upstream"
                      >{{ remotes[repo.root]!.upstream }}</span
                    >
                    <span
                      v-if="remotes[repo.root]!.ahead"
                      class="remote-strip__badge remote-strip__badge--ahead"
                      :title="
                        t('workspace.remoteAhead', {
                          count: remotes[repo.root]!.ahead,
                        })
                      "
                      >↑{{ remotes[repo.root]!.ahead }}</span
                    >
                    <span
                      v-if="remotes[repo.root]!.behind"
                      class="remote-strip__badge remote-strip__badge--behind"
                      :title="
                        t('workspace.remoteBehind', {
                          count: remotes[repo.root]!.behind,
                        })
                      "
                      >↓{{ remotes[repo.root]!.behind }}</span
                    >
                  </template>
                  <span v-else class="remote-strip__no-upstream">{{
                    t("workspace.noUpstream")
                  }}</span>
                  <button
                    type="button"
                    class="remote-strip__action"
                    :title="t('workspace.remoteRefresh')"
                    :aria-label="t('workspace.remoteRefresh')"
                    :disabled="remoteLoading[repo.root]"
                    @click="refreshGitRemote(true, repo.root)"
                  >
                    <t-icon
                      name="refresh"
                      size="13px"
                      :class="{ spinning: remoteLoading[repo.root] }"
                    />
                  </button>
                  <button
                    type="button"
                    class="remote-strip__action remote-strip__history"
                    :title="t('workspace.commitHistory')"
                    :aria-label="t('workspace.commitHistory')"
                    :aria-expanded="!!historyOpen[repo.root]"
                    @click="toggleHistory(repo.root)"
                  >
                    <t-icon name="history" size="13px" />
                  </button>
                  <button
                    v-if="pullVisible(repo.root) && !diverged(repo.root)"
                    type="button"
                    class="remote-strip__action remote-strip__pull"
                    :disabled="
                      pullPending[repo.root] ||
                      pushPending[repo.root] ||
                      commitPending[repo.root] ||
                      branchSwitching[repo.root] ||
                      !remotes[repo.root]!.behind
                    "
                    @click="submitPull(repo.root)"
                  >
                    <t-icon
                      :name="pullPending[repo.root] ? 'loading' : 'download'"
                      size="13px"
                    />
                    {{
                      pullPending[repo.root]
                        ? t("workspace.pulling")
                        : t("workspace.pull")
                    }}
                  </button>
                  <button
                    v-else-if="diverged(repo.root)"
                    type="button"
                    class="remote-strip__action remote-strip__pull"
                    :title="t('workspace.pullAlignHint')"
                    :disabled="
                      pullPending[repo.root] ||
                      pushPending[repo.root] ||
                      commitPending[repo.root] ||
                      branchSwitching[repo.root]
                    "
                    @click="submitPull(repo.root, true)"
                  >
                    <t-icon
                      :name="pullPending[repo.root] ? 'loading' : 'download'"
                      size="13px"
                    />
                    {{
                      pullPending[repo.root]
                        ? t("workspace.pulling")
                        : t("workspace.pullAlign")
                    }}
                  </button>
                  <button
                    v-if="pushVisible(repo.root)"
                    type="button"
                    class="remote-strip__action remote-strip__push"
                    :disabled="
                      pushPending[repo.root] ||
                      commitPending[repo.root] ||
                      pullPending[repo.root] ||
                      branchSwitching[repo.root]
                    "
                    @click="submitPush(repo.root)"
                  >
                    <t-icon
                      :name="pushPending[repo.root] ? 'loading' : 'upload'"
                      size="13px"
                    />
                    {{
                      pushPending[repo.root]
                        ? t("workspace.pushing")
                        : t("workspace.push")
                    }}
                  </button>
                  <button
                    v-if="repoChanges(repo.root).length"
                    type="button"
                    class="remote-strip__action remote-strip__discard"
                    :disabled="
                      commitPending[repo.root] ||
                      pushPending[repo.root] ||
                      pullPending[repo.root] ||
                      branchSwitching[repo.root]
                    "
                    :title="t('workspace.discard')"
                    @click="
                      discardForRepo = repo.root;
                      discardError = '';
                    "
                  >
                    <t-icon name="rollback" size="13px" />
                    {{ t("workspace.discard") }}
                  </button>
                </div>
                <p
                  v-if="branchErrors[repo.root]"
                  class="remote-strip__error"
                  role="alert"
                >
                  {{ branchErrors[repo.root] }}
                </p>
                <p
                  v-if="pullErrors[repo.root]"
                  class="remote-strip__error"
                  role="alert"
                >
                  {{ pullErrors[repo.root] }}
                </p>
                <p
                  v-if="remotes[repo.root]!.behind"
                  class="remote-strip__notice"
                  role="status"
                >
                  {{
                    t("workspace.remoteUpdatesAvailable", {
                      count: remotes[repo.root]!.behind,
                    })
                  }}
                </p>
                <ul
                  v-if="
                    remotes[repo.root]!.behind &&
                    remotes[repo.root]!.incoming.length
                  "
                  class="remote-strip__incoming"
                >
                  <li
                    v-for="entry in remotes[repo.root]!.incoming"
                    :key="entry.sha"
                    class="remote-strip__commit"
                  >
                    <code class="remote-strip__sha">{{ entry.sha }}</code>
                    <span class="remote-strip__subject">{{
                      entry.subject
                    }}</span>
                  </li>
                </ul>
              </div>
              <div v-if="!remoteVisible(repo.root) && remoteLoading[repo.root]" class="remote-strip remote-strip--loading" role="status">
                <div class="remote-strip__row">
                  <span class="remote-strip__placeholder" aria-hidden="true"><t-icon name="git-branch" size="14px" /></span>
                  <span class="remote-strip__loading-text">{{ t('workspace.gitMetadataLoading') }}</span>
                  <t-icon name="loading" size="13px" class="spinning" />
                </div>
              </div>
              <p
                v-if="remoteErrors[repo.root]"
                class="remote-strip__error"
                role="status"
              >
                {{ remoteErrors[repo.root] }}
              </p>
              <p
                v-if="pushErrors[repo.root]"
                class="remote-strip__error"
                role="alert"
              >
                {{ pushErrors[repo.root] }}
                <button
                  v-if="!pushVisible(repo.root)"
                  type="button"
                  class="remote-strip__action"
                  :disabled="pushPending[repo.root] || commitPending[repo.root]"
                  @click="submitPush(repo.root)"
                >
                  {{ t("workspace.push") }}
                </button>
              </p>
              <p
                v-if="pushPending[repo.root] && !remoteVisible(repo.root)"
                class="remote-strip__notice"
                role="status"
              >
                {{ t("workspace.pushing") }}
              </p>
              <WorkspaceGitHistory
                v-if="historyOpen[repo.root]"
                :session-id="sessionId"
                :repo="repo.root"
                :load-files="api.gitHistoryFiles ? (commit: string, path?: string) => api.gitHistoryFiles!(sessionId, commit, repo.root, path) : undefined"
                :report="gitHistories[repo.root]"
                :loading="historyLoading[repo.root]"
                :error="historyErrors[repo.root]"
                @refresh="refreshGitHistory(repo.root, false, true)"
                @load-more="refreshGitHistory(repo.root, true)"
                @revert="askRevert(repo.root, $event)"
              />
              <div
                v-if="commitVisible(repo.root) && !historyOpen[repo.root]"
                class="commit-box"
              >
                <textarea
                  v-model="commitMessages[repo.root]"
                  class="commit-box__input"
                  rows="3"
                  :placeholder="t('workspace.commitPlaceholder')"
                  :disabled="commitPending[repo.root]"
                  @keydown.meta.enter.prevent="submitCommit(repo.root, true)"
                  @keydown.ctrl.enter.prevent="submitCommit(repo.root, true)"
                />
                <div class="commit-box__footer">
                  <span
                    v-if="commitErrors[repo.root]"
                    class="commit-box__error"
                    role="alert"
                    >{{ commitErrors[repo.root] }}
                  </span>
                  <div class="commit-box__actions">
                    <button
                      type="button"
                      class="commit-box__submit commit-box__submit--local"
                      :disabled="commitDisabled(repo.root)"
                      @click="submitCommit(repo.root)"
                    >
                      <t-icon
                        v-if="commitLocalPending(repo.root)"
                        name="loading"
                        size="14px"
                      />
                      {{
                        commitLocalPending(repo.root)
                          ? t("workspace.committing")
                          : t("workspace.commitLocalOnly")
                      }}
                    </button>
                    <button
                      type="button"
                      class="commit-box__submit"
                      :title="t('workspace.commitSelectedPushHint')"
                      :disabled="commitDisabled(repo.root)"
                      @click="submitCommit(repo.root, true)"
                    >
                      <t-icon
                        :name="commitPushPending(repo.root) ? 'loading' : 'check'"
                        size="14px"
                      />
                      {{
                        commitPushPending(repo.root)
                          ? t(
                              pushPending[repo.root]
                                ? "workspace.pushing"
                                : "workspace.committingAndPushing",
                            )
                          : t("workspace.commitAndPushFiles", {
                              count: selectedCommitPaths(repo.root).length,
                            })
                      }}
                    </button>
                  </div>
                </div>
                <div class="commit-box__selection">
                  <t-checkbox
                    :checked="
                      selectedCommitPaths(repo.root).length ===
                      repoChanges(repo.root).length
                    "
                    :indeterminate="
                      selectedCommitPaths(repo.root).length > 0 &&
                      selectedCommitPaths(repo.root).length <
                        repoChanges(repo.root).length
                    "
                    :disabled="
                      commitPending[repo.root] || pushPending[repo.root]
                    "
                    @change="
                      (checked: boolean) =>
                        selectAllCommitFiles(repo.root, checked)
                    "
                    >{{ t("workspace.selectAllCommitFiles") }}</t-checkbox
                  >
                  <span>{{
                    t("workspace.commitSelectionCount", {
                      selected: selectedCommitPaths(repo.root).length,
                      total: repoChanges(repo.root).length,
                    })
                  }}</span>
                </div>
              </div>
              <div
                v-if="
                  !repoChanges(repo.root).length && !historyOpen[repo.root]
                "
                class="diff-empty diff-empty--history"
              >
                <span class="diff-empty__icon" aria-hidden="true">
                  <t-icon name="check-circle" size="30px" />
                </span>
                <p class="diff-empty__hint">
                  {{ t("workspace.noChangesHint") }}
                </p>
              </div>
              <template v-if="!historyOpen[repo.root]">
                <WorkspaceFileDiff
                  v-for="change in repoChanges(repo.root)"
                  :key="`${sessionId}:${repo.root}:${change.path}`"
                  :change="change"
                  selectable
                  :selected="selectedCommitPaths(repo.root).includes(change.path)"
                  :selection-disabled="
                    commitPending[repo.root] || pushPending[repo.root]
                  "
                  @selection-change="
                    selectCommitFile(repo.root, change.path, $event)
                  "
                  @open-source="
                    openFile(
                      relativePath(
                        repo.root ? `${repo.root}/${change.path}` : change.path,
                      ) ||
                        (repo.root ? `${repo.root}/${change.path}` : change.path),
                      'source',
                    )
                  "
                />
              </template>
            </div>
          </div>
          <div
            v-else-if="!file && workspaceState !== 'expired'"
            class="workspace-empty"
          >
            <div class="empty-art" aria-hidden="true">
              <span>&lt;/&gt;</span>
            </div>
            <h2>{{ t("workspace.emptyTitle") }}</h2>
            <p>{{ t("workspace.emptyDescription") }}</p>
            <div class="empty-steps">
              <span
                >01 <b>{{ t("workspace.code") }}</b></span
              ><i></i
              ><span
                >02 <b>{{ t("workspace.preview") }}</b></span
              ><i></i
              ><span
                >03 <b>{{ t("workspace.test") }}</b></span
              >
            </div>
          </div>
          <template v-else-if="imageSource && file">
            <p v-if="imageError" class="workspace-notice" role="alert">
              {{ t("workspace.imageError") }}
            </p>
            <div
              v-else
              class="image-preview"
              tabindex="0"
              :aria-label="file.path"
            >
              <img
                :key="imageSource"
                :src="imageSource"
                :alt="file.path"
                @error="imageError = true"
              />
            </div>
          </template>
          <template v-else-if="!showFilePreview && !showFileDiff && file">
            <p v-if="copyError" class="workspace-notice" role="alert">
              {{ copyError }}
            </p>
            <div class="source-scroll" tabindex="0" :aria-label="file.path">
              <div class="source-lines">
                <div
                  v-for="(line, index) in highlightedLines"
                  :key="index"
                  class="source-row"
                >
                  <span class="source-gutter" aria-hidden="true">{{
                    index + 1
                  }}</span>
                  <pre
                    class="source-code"
                  ><code v-html="line || '&#8203;'" /></pre>
                </div>
              </div>
            </div>
          </template>
          <template v-else-if="showFilePreview && file">
            <p v-if="previewError" class="workspace-notice" role="alert">
              {{ previewError }}
            </p>
            <p v-if="previewWarnings.length" class="workspace-notice">
              {{
                t("workspace.previewWarning", {
                  files: previewWarnings.join(", "),
                })
              }}
            </p>
            <div v-if="previewLoading" class="preview-loading" role="status">
              {{ t("workspace.loading") }}
            </div>
            <div
              v-else-if="previewHtml"
              class="preview-stage"
              :class="{ mobile: mobilePreview }"
            >
              <iframe
                :key="previewVersion"
                :srcdoc="previewHtml"
                sandbox="allow-scripts"
                referrerpolicy="no-referrer"
                :title="`${t('workspace.preview')} · ${file.path}`"
              />
            </div>
            <div v-else class="workspace-empty">
              <t-icon name="file-code" size="32px" />
              <p>{{ t("workspace.previewEmpty") }}</p>
            </div>
          </template>
          <footer v-if="file && !showFileDiff" class="file-status">
            <span>{{
              t(
                imageSource
                  ? "workspace.imagePreview"
                  : showFilePreview
                    ? "workspace.previewNote"
                    : "workspace.readOnly",
              )
            }}</span>
            <span v-if="!showFilePreview && !imageSource">{{
              t("workspace.lines", { count: lineCount })
            }}</span>
          </footer>
        </div>
      </div>
    </template>
    <WorkspaceTestResults
      v-else
      :key="sessionId"
      :session-id="sessionId"
      :active="active"
      :revision="revision"
      :agent-id="agentId"
      :agent-source-tenant-id="agentSourceTenantId"
      @ask="emit('ask', $event)"
      @report-created="emit('reportCreated', $event)"
    />
    <t-dialog
      :visible="pushAuthForRepo !== null"
      :header="t('workspace.pushAuthTitle')"
      :confirm-btn="{
        content: t('workspace.pushAuthConfirm'),
        loading: pushAuthPending,
        disabled: pushAuthPending || pushAuthInvalid,
        theme: 'primary',
      }"
      :cancel-btn="{ content: t('common.cancel'), disabled: pushAuthPending }"
      :on-confirm="confirmPushAuth"
      :on-close="closePushAuth"
      :close-btn="!pushAuthPending"
      :close-on-overlay-click="!pushAuthPending"
      :close-on-esc-keydown="!pushAuthPending"
      width="440px"
      @update:visible="
        (v: boolean) => {
          if (!v) closePushAuth();
        }
      "
    >
      <p class="identity-dialog__hint">
        {{
          t(
            pushAuthMode === "ssh"
              ? "workspace.pushAuthSSH"
              : "workspace.pushAuthHint",
          )
        }}
      </p>
      <p class="push-auth__remote">{{ pushAuthRemote }}</p>
      <template v-if="pushAuthMode === 'http'">
        <div class="identity-dialog__field">
          <label class="identity-dialog__label">{{
            t("workspace.pushAuthUsername")
          }}</label>
          <t-input
            v-model="pushAuthUsername"
            :disabled="pushAuthPending"
            :placeholder="t('workspace.pushAuthUsernamePlaceholder')"
            autocomplete="off"
            autofocus
          />
        </div>
        <div class="identity-dialog__field">
          <label class="identity-dialog__label">{{
            t("workspace.pushAuthPassword")
          }}</label>
          <t-input
            v-model="pushAuthPassword"
            type="password"
            :disabled="pushAuthPending"
            :placeholder="t('workspace.pushAuthPasswordPlaceholder')"
            autocomplete="new-password"
            @enter="confirmPushAuth"
          />
        </div>
      </template>
      <p v-if="pushAuthError" class="identity-dialog__error" role="alert">
        {{ pushAuthError }}
      </p>
    </t-dialog>
    <t-dialog
      :visible="discardForRepo !== null"
      :header="t('workspace.discardTitle')"
      :confirm-btn="{
        content: t('workspace.discardConfirm'),
        loading: discardPending,
        theme: 'danger',
      }"
      :on-confirm="confirmDiscard"
      :on-close="() => (discardForRepo = null)"
      :close-on-esc-keydown="!discardPending"
      width="420px"
      @update:visible="
        (v: boolean) => {
          if (!v) discardForRepo = null;
        }
      "
    >
      <p class="identity-dialog__hint">{{ t("workspace.discardHint") }}</p>
      <p v-if="discardError" class="identity-dialog__error" role="alert">
        {{ discardError }}
      </p>
    </t-dialog>
    <t-dialog
      :visible="revertTarget !== null"
      :header="t('workspace.historyRevertTitle')"
      :confirm-btn="{
        content: t('workspace.historyRevertConfirm'),
        loading: revertPending,
        theme: 'warning',
      }"
      :on-confirm="confirmRevert"
      :on-close="() => (revertTarget = null)"
      :close-on-esc-keydown="!revertPending"
      width="420px"
      @update:visible="
        (v: boolean) => {
          if (!v) revertTarget = null;
        }
      "
    >
      <p class="identity-dialog__hint">
        {{
          t("workspace.historyRevertHint", {
            sha: revertTarget ? revertTarget.shortSha : "",
          })
        }}
      </p>
      <p v-if="revertError" class="identity-dialog__error" role="alert">
        {{ revertError }}
      </p>
    </t-dialog>
  </section>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { MessagePlugin } from "tdesign-vue-next";
import PanelResizeHandle from "@/components/PanelResizeHandle.vue";
import WorkspaceFileDiff from "./WorkspaceFileDiff.vue";
import WorkspaceTestResults from "./WorkspaceTestResults.vue";
import WorkspaceGitHistory from "./WorkspaceGitHistory.vue";
import {
  formatWorkspaceCode,
  highlightWorkspaceLines,
} from "@/utils/workspaceCode";
import { marked } from "marked";
import { sanitizeMarkdownHTML } from "@/utils/security";
import {
  getProgrammingTree,
  getProgrammingFile,
  getProgrammingGitStatus,
  commitProgrammingChanges,
  getProgrammingGitRemote,
  pullProgrammingChanges,
  pushProgrammingChanges,
  getProgrammingGitBranches,
  getProgrammingGitHistory,
  getProgrammingGitHistoryFiles,
  switchProgrammingGitBranch,
  discardProgrammingChanges,
  resetProgrammingGitCommit,
  runProgrammingCommand,
  type ProgrammingNode,
  type ProgrammingFile,
  type ProgrammingCommandResult,
  type ProgrammingGitStatus,
  type ProgrammingGitStatusFile,
  type ProgrammingGitCommitResult,
  type ProgrammingGitRemote,
  type ProgrammingGitPullResult,
  type ProgrammingGitPushResult,
  type ProgrammingGitPushCredentials,
  type ProgrammingGitBranches,
  type ProgrammingGitActionResult,
  type ProgrammingGitHistory,
  type ProgrammingGitHistoryCommit,
  type ProgrammingGitHistoryFiles,
  type ProgrammingGitResetResult,
} from "@/api/programming";
import {
  buildWorkspacePreview,
  canPreviewSource,
} from "@/utils/workspacePreview";
import { useFileChanges, type FileChange } from "@/composables/useFileChanges";

import type {
  WorkspaceTab,
  WorkspaceCommandRun,
} from "@/utils/workspaceEvents";
import { mergeWorkspaceFileChangesWithGitStatus } from "@/utils/workspaceEvents";

export interface WorkspaceGateway {
  tree: (
    session: string,
    path: string,
  ) => Promise<{
    nodes: ProgrammingNode[];
    root: string;
    origin?: "host" | "sandbox";
    workspace_state?: "live" | "expired";
  }>;
  file: (session: string, path: string) => Promise<ProgrammingFile>;
  run: (session: string, command: string) => Promise<ProgrammingCommandResult>;
  gitStatus?: (session: string) => Promise<ProgrammingGitStatus>;
  gitCommit?: (
    session: string,
    message: string,
    repo?: string,
    paths?: string[],
  ) => Promise<ProgrammingGitCommitResult>;
  gitRemote?: (
    session: string,
    refresh?: boolean,
    repo?: string,
  ) => Promise<ProgrammingGitRemote>;
  gitPull?: (
    session: string,
    repo?: string,
    realign?: boolean,
  ) => Promise<ProgrammingGitPullResult>;
  gitPush?: (
    session: string,
    repo?: string,
    credentials?: ProgrammingGitPushCredentials,
  ) => Promise<ProgrammingGitPushResult>;
  gitIdentity?: (
    session: string,
    data: { name: string; email: string; repo?: string },
  ) => Promise<{ saved: boolean }>;
  gitBranches?: (
    session: string,
    repo?: string,
  ) => Promise<ProgrammingGitBranches>;
  gitHistory?: (session: string, repo?: string, page?: { skip?: number; head?: string; upstream?: string }) => Promise<ProgrammingGitHistory>;
  gitHistoryFiles?: (session: string, commit: string, repo?: string, path?: string) => Promise<ProgrammingGitHistoryFiles>;
  gitSwitchBranch?: (
    session: string,
    branch: string,
    repo?: string,
    remote?: boolean,
  ) => Promise<ProgrammingGitActionResult>;
  gitDiscard?: (
    session: string,
    repo?: string,
  ) => Promise<ProgrammingGitActionResult>;
  gitReset?: (
    session: string,
    commit: string,
    repo?: string,
  ) => Promise<ProgrammingGitResetResult>;
}
const props = withDefaults(
  defineProps<{
    sessionId: string;
    tab: WorkspaceTab;
    active: boolean;
    revision?: number;
    focusPath?: string;
    agentRuns?: WorkspaceCommandRun[];
    agentId?: string;
    agentSourceTenantId?: string | number | null;
    gateway?: WorkspaceGateway;
  }>(),
  { revision: 0, focusPath: "", agentRuns: () => [] },
);
const emit = defineEmits<{
  ask: [prompt: string];
  reportCreated: [messageId: string];
  viewChange: [view: "source" | "diff" | "preview"];
}>();
const { t } = useI18n();
const api: WorkspaceGateway = {
  tree: async (session, path) => (await getProgrammingTree(session, path)).data,
  file: async (session, path) => (await getProgrammingFile(session, path)).data,
  run: async (session, command) =>
    (await runProgrammingCommand(session, { command, timeout: 60 })).data,
  gitStatus: async (session) => (await getProgrammingGitStatus(session)).data,
  gitCommit: async (session, message, repo, paths) =>
    (await commitProgrammingChanges(session, message, repo, paths)).data,
  gitRemote: async (session, refresh, repo) =>
    (await getProgrammingGitRemote(session, refresh, repo)).data,
  gitPull: async (session, repo, realign) =>
    (await pullProgrammingChanges(session, repo, realign)).data,
  gitPush: async (session, repo, credentials) =>
    (await pushProgrammingChanges(session, repo, credentials)).data,
  gitBranches: async (session, repo) =>
    (await getProgrammingGitBranches(session, repo)).data,
  gitHistory: async (session, repo, page) =>
    (await getProgrammingGitHistory(session, repo, page)).data,
  gitHistoryFiles: async (session, commit, repo, path) =>
    (await getProgrammingGitHistoryFiles(session, commit, repo, path)).data,
  gitSwitchBranch: async (session, branch, repo, remote) =>
    (await switchProgrammingGitBranch(session, branch, repo, remote)).data,
  gitDiscard: async (session, repo) =>
    (await discardProgrammingChanges(session, repo)).data,
  gitReset: async (session, commit, repo) =>
    (await resetProgrammingGitCommit(session, commit, repo)).data,
  ...props.gateway,
};
// Remote sandboxes stage attachments in /workspace/input and artifacts in
// /workspace/output next to the project. The code tree follows Manus and only
// shows the project itself, so both are hidden there. A host workspace has no
// such staging tree, so folders with those names still belong to the project.
const HIDDEN_SANDBOX_ROOT_ENTRIES = new Set(["input", "output"]);
// Manus-style markers: TS/JS text for scripts, braces for data and config files,
// the git glyph for dotfiles that configure the repository, a page for the rest.
const TEXT_GLYPHS: Record<string, string> = {
  ts: "TS",
  tsx: "TS",
  mts: "TS",
  cts: "TS",
  js: "JS",
  jsx: "JS",
  mjs: "JS",
  cjs: "JS",
};
const BRACE_EXTENSIONS = new Set([
  "json",
  "jsonc",
  "json5",
  "yaml",
  "yml",
  "toml",
  "lock",
]);
const GIT_FILES = /^\.git(?:ignore|attributes|modules|config)$/;
type FileGlyph = { text?: string; icon?: string };
type TreeRow = {
  node: ProgrammingNode;
  depth: number;
  expanded: boolean;
  loading: boolean;
  glyph: FileGlyph;
};
function fileGlyph(name: string): FileGlyph {
  const lower = name.toLowerCase();
  if (GIT_FILES.test(lower)) return { icon: "git-branch" };
  const extension = /\.([a-z0-9]+)$/.exec(lower)?.[1] || "";
  if (TEXT_GLYPHS[extension]) return { text: TEXT_GLYPHS[extension] };
  if (BRACE_EXTENSIONS.has(extension)) return { text: "{}" };
  if (/^\.(?!gitkeep$)[a-z0-9]+$/.test(lower)) return { text: "{}" };
  return { icon: "file" };
}
const nodes = ref<ProgrammingNode[]>([]),
  file = ref<ProgrammingFile | null>(null);
const directory = ref(""),
  root = ref(""),
  error = ref(""),
  workspaceOrigin = ref<"host" | "sandbox">("sandbox");
// A reclaimed sandbox comes back as an empty workspace; the backend marks it
// so the panel explains the state instead of showing "no files yet".
const workspaceState = ref<"live" | "expired">("live");
const childrenByPath = ref<Record<string, ProgrammingNode[]>>({}),
  expandedPaths = ref<string[]>([]),
  pendingPaths = ref<string[]>([]);
const copied = ref(false),
  copyError = ref(""),
  mobilePreview = ref(false),
  previewHtml = ref(""),
  previewWarnings = ref<string[]>([]),
  previewError = ref(""),
  previewLoading = ref(false),
  previewVersion = ref(0);
type FileView = "source" | "diff" | "preview";
const fileView = ref<FileView>("source");
watch(fileView, (view) => emit("viewChange", view), { immediate: true });
const imageError = ref(false);
const imageSource = computed(() => {
  const current = file.value;
  if (
    current?.encoding !== "base64" ||
    !/^image\/(?:png|jpeg|gif|webp|bmp|x-icon|avif)$/.test(
      current.mime_type || "",
    )
  )
    return "";
  return `data:${current.mime_type};base64,${current.content}`;
});
watch(imageSource, () => {
  imageError.value = false;
});
const { changes: fileChanges, activeId: activeFileChange } = useFileChanges();
const gitLinked = ref(false);
const gitStatusLoading = ref(false);
// Per-repo git status snapshot, keyed by workspace-relative repo root (""
// for a repo at the workspace root). One entry per discovered repository; the
// changes panel renders one group per entry. null until the first load.
type RepoGitState = {
  root: string;
  name: string;
  files: ProgrammingGitStatusFile[];
};
const gitRepos = ref<RepoGitState[] | null>(null);
// Backward-compatible flat view: the changes the session log recorded merged
// with whatever the (single) git repo reports. Only used when there is exactly
// one repo — multi-repo rendering reads gitRepos directly per group.
const gitStatusFiles = computed<ProgrammingGitStatusFile[] | null>(() => {
  if (gitRepos.value === null) return null;
  if (gitRepos.value.length === 1) return gitRepos.value[0].files;
  // Multi-repo: flatten for the legacy single-list consumers.
  return gitRepos.value.flatMap((r) => r.files);
});
const visibleFileChanges = computed<FileChange[]>(() => {
  if (!gitLinked.value) return [];
  if (gitRepos.value === null) return fileChanges.value;
  // Consumers outside a repo group open files relative to the workspace.
  return gitRepos.value.flatMap((repo) =>
    repoChanges(repo.root).map((change) => ({
      ...change,
      path: repo.root ? `${repo.root}/${change.path}` : change.path,
    })),
  );
});
const canPreviewFile = computed(() =>
  Boolean(file.value && canPreviewSource(file.value.path)),
);
const formattedCode = ref<{ source: string } | null>(null);
watch(
  () =>
    [
      props.sessionId,
      file.value?.path,
      file.value?.encoding === "base64" ? undefined : file.value?.content,
    ] as const,
  async ([, path, source], _, onCleanup) => {
    let cancelled = false;
    onCleanup(() => {
      cancelled = true;
    });
    formattedCode.value = null;
    if (!path || source === undefined) return;
    const display = await formatWorkspaceCode(source, path);
    if (!cancelled) formattedCode.value = { source: display };
  },
  { immediate: true, flush: "sync" },
);
// Source always contains the complete file. Changes have their own file list,
// which stays visible on a linked git workspace even when clean so the reader
// can confirm "no changes" instead of hunting for a vanished tab.
const hasChangesPanel = computed(() => gitLinked.value);
const showFileDiff = computed(
  () =>
    props.tab === "source" &&
    fileView.value === "diff" &&
    hasChangesPanel.value,
);
const showViewSwitch = computed(
  () =>
    props.tab === "source" && (canPreviewFile.value || hasChangesPanel.value),
);
const showFilePreview = computed(
  () =>
    !imageSource.value &&
    (props.tab === "preview" ||
      (fileView.value === "preview" && canPreviewFile.value)),
);
let generation = 0,
  fileRequest = 0,
  previewRequest = 0,
  refreshRequest = 0;
let gitStatusRequest = 0;
const remoteRequests = new Map<string, number>();
const treeRequests = new Map<string, number>();
let refreshTimer: ReturnType<typeof setTimeout> | undefined;
const busy = computed(() => pendingPaths.value.length > 0);
const content = computed(() =>
  file.value?.encoding === "base64"
    ? ""
    : (formattedCode.value?.source ?? file.value?.content ?? ""),
);
const lineCount = computed(() => content.value.split("\n").length);
const highlightedLines = computed(() =>
  highlightWorkspaceLines(content.value, file.value?.path || ""),
);
const fileParts = computed(() => {
  const path =
    relativePath(file.value?.path || "") ||
    (file.value?.path || "").replaceAll("\\", "/");
  return path.split("/").filter(Boolean);
});

function visibleChildren(path: string): ProgrammingNode[] {
  const list = childrenByPath.value[path] || [];
  if (path || workspaceOrigin.value === "host") return list;
  return list.filter(
    (node) => !HIDDEN_SANDBOX_ROOT_ENTRIES.has(node.name.toLowerCase()),
  );
}
const treeRows = computed<TreeRow[]>(() => {
  const rows: TreeRow[] = [];
  const walk = (path: string, depth: number) => {
    for (const node of visibleChildren(path)) {
      const expanded =
        node.kind === "directory" && expandedPaths.value.includes(node.path);
      rows.push({
        node,
        depth,
        expanded,
        loading: expanded && pendingPaths.value.includes(node.path),
        glyph: node.kind === "file" ? fileGlyph(node.name) : {},
      });
      if (expanded) walk(node.path, depth + 1);
    }
  };
  walk("", 0);
  return rows;
});
function defaultFile(): string {
  const roots = visibleChildren("");
  const isIndex = (path: string) => /(?:^|\/)index\.html?$/i.test(path);
  return (
    roots.find((node) => node.kind === "file" && isIndex(node.path))?.path ||
    roots.find((node) => node.kind === "file")?.path ||
    treeRows.value.find((row) => row.node.kind === "file")?.node.path ||
    ""
  );
}

async function loadDirectory(path = "") {
  const request = (treeRequests.get(path) || 0) + 1,
    epoch = generation;
  treeRequests.set(path, request);
  pendingPaths.value = [
    ...pendingPaths.value.filter((item) => item !== path),
    path,
  ];
  error.value = "";
  try {
    const data = await api.tree(props.sessionId, path);
    if (epoch !== generation || treeRequests.get(path) !== request) return;
    childrenByPath.value = { ...childrenByPath.value, [path]: data.nodes };
    // gitLinked is no longer set from a top-level .git node here: a project
    // cloned into a subdirectory has no root .git, so the backend's git-status
    // probe (refreshGitStatus) is the authoritative link signal instead.
    if (data.origin) workspaceOrigin.value = data.origin;
    workspaceState.value =
      data.workspace_state === "expired" ? "expired" : "live";
    nodes.value = visibleChildren(path);
    root.value = data.root;
    directory.value = path;
    if (workspaceState.value === "expired")
      error.value = t("workspace.workspaceExpired");
  } catch {
    if (epoch === generation && treeRequests.get(path) === request)
      error.value = t("workspace.unavailable");
  } finally {
    if (epoch === generation && treeRequests.get(path) === request) {
      pendingPaths.value = pendingPaths.value.filter((item) => item !== path);
    }
  }
}
async function toggleDirectory(node: ProgrammingNode) {
  if (node.kind !== "directory") return;
  if (expandedPaths.value.includes(node.path)) {
    expandedPaths.value = expandedPaths.value.filter(
      (path) => path !== node.path && !path.startsWith(`${node.path}/`),
    );
    return;
  }
  expandedPaths.value = [...expandedPaths.value, node.path];
  if (!(node.path in childrenByPath.value)) await loadDirectory(node.path);
}
function activateNode(node: ProgrammingNode) {
  if (node.kind === "directory") void toggleDirectory(node);
  else void openFile(node.path, "source");
}
/** Expand every ancestor of a file so the tree reveals it after a tool event. */
async function revealPath(target: string) {
  const parts = target.split("/").filter(Boolean),
    epoch = generation;
  if (parts.length < 2) return;
  let current = "";
  for (const part of parts.slice(0, -1)) {
    current = current ? `${current}/${part}` : part;
    if (!(current in childrenByPath.value)) await loadDirectory(current);
    if (epoch !== generation) return;
    if (!expandedPaths.value.includes(current))
      expandedPaths.value = [...expandedPaths.value, current];
  }
}
function relativePath(path: string) {
  const normalized = path.replaceAll("\\", "/");
  const prefix = root.value.replace(/\/$/, "") + "/";
  return normalized.startsWith(prefix)
    ? normalized.slice(prefix.length)
    : normalized.startsWith("/")
      ? ""
      : normalized;
}
async function openFile(path: string, view?: FileView) {
  const request = ++fileRequest,
    epoch = generation;
  error.value = "";
  copied.value = false;
  copyError.value = "";
  previewHtml.value = "";
  previewRequest++;
  try {
    const data = await api.file(props.sessionId, path);
    if (epoch !== generation || request !== fileRequest) return;
    file.value = data;
    imageError.value = false;
    if (view) fileView.value = view;
    saveOpenFile(path);
    if (showFilePreview.value) void preparePreview();
  } catch (err) {
    if (epoch !== generation || request !== fileRequest) return;
    // A reclaimed workspace loses every path at once. Say that, instead of
    // blaming one file and sending the reader looking for a path bug.
    if (workspaceExpiredFailure(err)) {
      workspaceState.value = "expired";
      error.value = t("workspace.workspaceExpired");
      void reconcileMissingFile(path);
      return;
    }
    error.value = t("workspace.fileError");
    void reconcileMissingFile(path);
  }
}

function openFileStorageKey(sessionId: string) {
  return `weknora.workspace.openFile.${sessionId}`;
}

// --- 文件树 / 代码区之间的可伸缩分隔条 --------------------------------
// The sandbox panel is already resizable; the code side of it is too. Dragging
// the divider trades width between the project tree and the code/diff view so
// a wide diff is readable without widening the whole panel.
const TREE_MIN_WIDTH = 132;
const TREE_MAX_WIDTH = 420;
// Never let the tree eat the code pane: the viewer keeps at least this much.
const TREE_CODE_MIN_WIDTH = 200;
const TREE_WIDTH_STORAGE_KEY = "weknora.workspace.treeWidth";
const treeRef = ref<HTMLElement | null>(null);
const measuredTreeWidth = ref(0);
// 0 keeps the responsive CSS default until the reader drags the divider.
const treeWidth = ref(storedTreeWidth());
let treeDragStart = 0;

function storedTreeWidth(): number {
  try {
    const raw = Number(localStorage.getItem(TREE_WIDTH_STORAGE_KEY));
    if (!Number.isFinite(raw) || raw <= 0) return 0;
    return Math.min(TREE_MAX_WIDTH, Math.max(TREE_MIN_WIDTH, Math.round(raw)));
  } catch {
    return 0;
  }
}

const treeStyle = computed(() =>
  treeWidth.value ? { width: `${treeWidth.value}px` } : undefined,
);

function clampTreeWidth(width: number): number {
  const body = treeRef.value?.parentElement;
  const cap = body
    ? Math.max(TREE_MIN_WIDTH, body.clientWidth - TREE_CODE_MIN_WIDTH)
    : TREE_MAX_WIDTH;
  return Math.min(
    TREE_MAX_WIDTH,
    cap,
    Math.max(TREE_MIN_WIDTH, Math.round(width)),
  );
}

function currentTreeWidth(): number {
  return (
    treeWidth.value ||
    measuredTreeWidth.value ||
    treeRef.value?.getBoundingClientRect().width ||
    TREE_MIN_WIDTH
  );
}

function startTreeResize() {
  treeDragStart = currentTreeWidth();
}

function resizeTree(delta: number) {
  treeWidth.value = clampTreeWidth(treeDragStart + delta);
}

function endTreeResize() {
  if (!treeWidth.value) return;
  try {
    localStorage.setItem(TREE_WIDTH_STORAGE_KEY, String(treeWidth.value));
  } catch {
    // localStorage may be unavailable in private mode; the drag still works.
  }
}

onMounted(() => {
  measuredTreeWidth.value = treeRef.value?.getBoundingClientRect().width || 0;
});

// Remember the file the user last had open so a reload restores it instead of
// falling back to the agent's last signal or the first file in the tree.
function savedOpenFile(): string {
  try {
    return localStorage.getItem(openFileStorageKey(props.sessionId)) || "";
  } catch {
    return "";
  }
}

function saveOpenFile(path: string) {
  try {
    localStorage.setItem(openFileStorageKey(props.sessionId), path);
  } catch {
    // localStorage may be unavailable in private mode; the view still works.
  }
}

/** 404 bodies from the programming API mark a reaped sandbox with a reason. */
function workspaceExpiredFailure(err: unknown): boolean {
  const details = (err as { error?: { details?: { reason?: string } } } | null)
    ?.error?.details;
  return details?.reason === "workspace_expired";
}
// A reclaimed sandbox comes back with an empty /workspace, so the tree can
// still offer rows the backend no longer reads. Re-listing the parent replaces
// the stale level; when the row really is gone it is dropped instead of
// staying clickable forever, and the notice says why rather than blaming the
// read.
async function reconcileMissingFile(path: string) {
  const parent = path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : "";
  const epoch = generation,
    request = fileRequest,
    readError = error.value;
  await loadDirectory(parent);
  if (epoch !== generation || request !== fileRequest) return;
  if ((childrenByPath.value[parent] || []).some((node) => node.path === path)) {
    error.value = readError || t("workspace.fileError");
    return;
  }
  childrenByPath.value = Object.fromEntries(
    Object.entries(childrenByPath.value).filter(
      ([key]) => key !== path && !key.startsWith(`${path}/`),
    ),
  );
  expandedPaths.value = expandedPaths.value.filter(
    (item) => item !== path && !item.startsWith(`${path}/`),
  );
  // The whole workspace is gone, so the file was not removed from under the
  // reader: keep the last content they opened visible and let the notice
  // explain why it can no longer be refreshed.
  if (workspaceState.value === "expired") {
    error.value = t("workspace.workspaceExpired");
    return;
  }
  if (file.value?.path === path) file.value = null;
  error.value = t("workspace.fileGone");
}
// Set by a live edit so the very next refresh opens that file in the diff view.
// A panel that mounts *after* the change missed the activeFileChange watcher
// firing, so the refresh is what lands the reader on the diff.
let pendingDiffPath = "";

/** Snapshot uncommitted changes so the diff list follows git, not the session log. */
async function refreshGitStatus() {
  if (!api.gitStatus) return;
  const epoch = generation,
    request = ++gitStatusRequest;
  gitStatusLoading.value = true;
  try {
    const data = await api.gitStatus(props.sessionId);
    if (epoch !== generation || request !== gitStatusRequest) return;
    if (!data.available) {
      // Not a git workspace: the conversation log drives the panel.
      gitRepos.value = null;
      gitLinked.value = false;
      return;
    }
    // Authoritative link signal: the backend probed for real repositories, so
    // a project cloned into a subdirectory links the same as one at the root.
    gitLinked.value = true;
    if (data.repos) {
      gitRepos.value = data.repos.map((r) => ({
        root: r.root,
        name: r.name,
        files: r.files ?? [],
      }));
    } else {
      // Legacy single-repo shape: no root/name, files at the top level.
      gitRepos.value = [{ root: "", name: "", files: data.files ?? [] }];
    }
    const cached = new Map(data.repos?.map((repo) => [repo.root, repo.remote]));
    for (const repo of gitRepos.value) {
      const initialRemote = cached.get(repo.root);
      if (initialRemote?.available && !remotes.value[repo.root]?.available) {
        remotes.value = { ...remotes.value, [repo.root]: initialRemote };
      } else {
        // Read cached refs immediately, independently of directory loading.
        void refreshGitRemote(false, repo.root);
      }
      void refreshGitHistory(repo.root);
    }
  } catch {
    if (epoch === generation && request === gitStatusRequest)
      gitRepos.value = null;
  } finally {
    if (epoch === generation && request === gitStatusRequest) gitStatusLoading.value = false;
  }
}

// Per-repo action state, keyed by workspace-relative repo root ("" for the
// workspace-root repo). Each repo group in the changes panel has its own
// commit box and remote strip, so the state is per-repo rather than global.
// commitError/pullError stay inline next to the box/button that caused them
// instead of a toast that scrolls away.
const commitMessages = ref<Record<string, string>>({});
const commitPending = ref<Record<string, boolean>>({});
const commitErrors = ref<Record<string, string>>({});
const commitPushRequested = ref<Record<string, boolean>>({});
const commitSelections = ref<Record<string, string[]>>({});
const pushPending = ref<Record<string, boolean>>({});
const pushErrors = ref<Record<string, string>>({});
const remotes = ref<Record<string, ProgrammingGitRemote | null>>({});
const remoteLoading = ref<Record<string, boolean>>({});
const remoteErrors = ref<Record<string, string>>({});
const gitHistories = ref<Record<string, ProgrammingGitHistory>>({});
const historyLoading = ref<Record<string, boolean>>({});
const historyErrors = ref<Record<string, string>>({});
const historyRequests = new Map<string, number>();
// Opened from the remote strip's action row, so a long list never takes up
// height in the changes view unless the reader asks for it.
const historyOpen = ref<Record<string, boolean>>({});
function toggleHistory(root: string) {
  historyOpen.value = {
    ...historyOpen.value,
    [root]: !historyOpen.value[root],
  };
  if (historyOpen.value[root]) void refreshGitHistory(root, false, true);
}
const fetchingRemoteRoots = new Set<string>();
let remotePollTimer: ReturnType<typeof setTimeout> | undefined;
let remotePollGeneration = 0;
const pullPending = ref<Record<string, boolean>>({});
const pullErrors = ref<Record<string, string>>({});

const multiRepo = computed(() => (gitRepos.value?.length ?? 0) > 1);

/** Merge recorded diff bodies only with the repository that owns each path.
 * Group paths stay repo-relative; opening a file prepends the repo root. */
function repoChanges(root: string): FileChange[] {
  if (!gitLinked.value) return [];
  const repo = gitRepos.value?.find((r) => r.root === root);
  if (!repo) return [];
  const changes = fileChanges.value.flatMap((change) => {
    const path = relativePath(change.path);
    if (!path || (root && !path.startsWith(`${root}/`))) return [];
    // A workspace-root repo must not claim a nested checkout's recorded edit.
    if (
      !root &&
      gitRepos.value?.some((r) => r.root && path.startsWith(`${r.root}/`))
    )
      return [];
    return [{ ...change, path: root ? path.slice(root.length + 1) : path }];
  });
  return mergeWorkspaceFileChangesWithGitStatus(changes, repo.files).map(
    (change) => ({
      ...change,
      id: change.id || `git:${root}:${change.path}`,
      timestamp: change.timestamp ?? 0,
      live: change.live ?? false,
    }),
  );
}

function selectedCommitPaths(root: string): string[] {
  const dirty = new Set(gitRepos.value?.find((repo) => repo.root === root)?.files.map((file) => file.path) ?? []);
  return (commitSelections.value[root] ?? []).filter((path) => dirty.has(path));
}

function selectCommitFile(root: string, path: string, checked: boolean) {
  if (commitPending.value[root] || pushPending.value[root]) return;
  const selected = new Set(selectedCommitPaths(root));
  if (checked) selected.add(path);
  else selected.delete(path);
  commitSelections.value = { ...commitSelections.value, [root]: [...selected] };
}

function selectAllCommitFiles(root: string, checked: boolean) {
  if (commitPending.value[root] || pushPending.value[root]) return;
  commitSelections.value = { ...commitSelections.value, [root]: checked ? repoChanges(root).map((change) => change.path) : [] };
}

watch(gitRepos, () => {
  // Do not carry a selection onto a new file that later reuses a committed path.
  const next: Record<string, string[]> = {};
  for (const repo of gitRepos.value ?? []) next[repo.root] = selectedCommitPaths(repo.root);
  commitSelections.value = next;
});

function commitVisible(root: string) {
  // The commit box only appears when the repo actually has changes; a clean
  // repo shows just the "no changes" hint, not an empty commit form.
  return (
    gitLinked.value && gitRepos.value !== null && repoChanges(root).length > 0
  );
}
function commitDisabled(root: string) {
  return (
    !!commitPending.value[root] ||
    !!pushPending.value[root] ||
    !!pullPending.value[root] ||
    !!branchSwitching.value[root] ||
    !(commitMessages.value[root] || "").trim() ||
    !selectedCommitPaths(root).length
  );
}

// The two commit buttons are separate actions, so each one shows progress only
// for the action the user actually pressed; commitPushRequested records which.
function commitLocalPending(root: string) {
  return !!commitPending.value[root] && !commitPushRequested.value[root];
}
function commitPushPending(root: string) {
  return !!commitPending.value[root] && !!commitPushRequested.value[root];
}

async function submitCommit(root: string, pushAfter = false, retryPaths?: string[]) {
  const message = (commitMessages.value[root] || "").trim();
  const paths = [...(retryPaths ?? selectedCommitPaths(root))];
  if (
    !api.gitCommit ||
    !message ||
    !paths.length ||
    commitPending.value[root] ||
    pushPending.value[root] ||
    pullPending.value[root] ||
    branchSwitching.value[root]
  )
    return;
  const epoch = generation;
  commitPushRequested.value = {
    ...commitPushRequested.value,
    [root]: pushAfter,
  };
  commitPending.value = { ...commitPending.value, [root]: true };
  commitErrors.value = { ...commitErrors.value, [root]: "" };
  try {
    const result = await api.gitCommit(props.sessionId, message, root, paths);
    if (epoch !== generation) return;
    if (result.committed) {
      commitMessages.value = { ...commitMessages.value, [root]: "" };
      commitSelections.value = { ...commitSelections.value, [root]: [] };
      // Selected files leave the list; other working/staged changes remain.
      // The commit also moves HEAD, so refresh history and remote counts.
      void refreshGitStatus();
      void refreshGitHistory(root, false, true);
      if (pushAfter) await submitPush(root, true);
      else {
        MessagePlugin.success(t("workspace.commitSuccess"));
        void refreshGitRemote(false, root);
      }
    } else {
      void refreshGitStatus();
      commitErrors.value = {
        ...commitErrors.value,
        [root]: result.detail || t("workspace.commitFailed"),
      };

    }
  } catch (err: any) {
    if (epoch !== generation) return;
    commitErrors.value = {
      ...commitErrors.value,
      [root]: typeof err?.message === "string" ? err.message : t("workspace.commitFailed"),
    };
    void refreshGitStatus();
  } finally {
    if (epoch === generation)
      commitPending.value = { ...commitPending.value, [root]: false };
  }
}

// The browser retains credentials only while submitting. Verified credentials
// are remembered by Git in the current session sandbox for later pushes.
const pushAuthForRepo = ref<string | null>(null);
const pushAuthMode = ref<'http' | 'ssh'>('http');
const pushAuthRemote = ref('');
const pushAuthUsername = ref('');
const pushAuthPassword = ref('');
const pushAuthError = ref('');
const pushAuthFromCommit = ref(false);
const pushAuthPending = computed(() => pushAuthForRepo.value !== null && !!pushPending.value[pushAuthForRepo.value]);
const pushAuthInvalid = computed(() => pushAuthMode.value === 'http' && (
  !pushAuthUsername.value.trim() || !pushAuthPassword.value ||
  /[:\r\n\x00]/.test(pushAuthUsername.value) || /[\r\n\x00]/.test(pushAuthPassword.value)
));
function resetPushAuth() {
  pushAuthForRepo.value = null;
  pushAuthUsername.value = '';
  pushAuthPassword.value = '';
  pushAuthRemote.value = '';
  pushAuthError.value = '';
  pushAuthFromCommit.value = false;
}
function closePushAuth() {
  if (!pushAuthPending.value) resetPushAuth();
}
async function confirmPushAuth() {
  const root = pushAuthForRepo.value;
  if (root === null || pushAuthPending.value || pushAuthInvalid.value) return;
  const credentials: ProgrammingGitPushCredentials = { remote_url: pushAuthRemote.value };
  if (pushAuthMode.value === 'ssh') credentials.use_ssh = true;
  else {
    credentials.username = pushAuthUsername.value.trim();
    credentials.password = pushAuthPassword.value;
  }
  pushAuthError.value = '';
  pushAuthPassword.value = '';
  await submitPush(root, pushAuthFromCommit.value, credentials);
}

// Branch switcher + discard, per repo. Reopening the dropdown reloads local
// and fetched remote branches. Switching is refused server-side while the
// tree is dirty, so the UI surfaces that as an inline error rather than
// guessing. Discard drops all uncommitted changes after a typed confirm.
const branchOptions = ref<Record<string, string[]>>({});
const remoteBranchOptions = ref<Record<string, string[]>>({});
const branchLoading = ref<Record<string, boolean>>({});
const branchRequests = new Map<string, number>();
const branchSwitching = ref<Record<string, boolean>>({});
const branchErrors = ref<Record<string, string>>({});
const discardForRepo = ref<string | null>(null);
const discardPending = ref(false);
const discardError = ref("");

function localBranchOptions(root: string): string[] {
  const branches = branchOptions.value[root] ?? [];
  const current = remotes.value[root]?.branch;
  // Keep unborn branches and detached HEADs visible even without a local ref.
  return current && !branches.includes(current)
    ? [current, ...branches]
    : branches;
}

async function ensureBranches(root: string, force = false) {
  if (!api.gitBranches || (branchLoading.value[root] && !force)) return;
  const epoch = generation,
    request = (branchRequests.get(root) || 0) + 1;
  branchRequests.set(root, request);
  branchLoading.value = { ...branchLoading.value, [root]: true };
  branchErrors.value = { ...branchErrors.value, [root]: "" };
  try {
    const data = await api.gitBranches(props.sessionId, root);
    if (epoch !== generation || branchRequests.get(root) !== request) return;
    branchOptions.value = {
      ...branchOptions.value,
      [root]: data.branches ?? [],
    };
    remoteBranchOptions.value = {
      ...remoteBranchOptions.value,
      [root]: data.remote_branches ?? [],
    };
    // Keep the strip's branch label in sync with what the server sees.
    if (remotes.value[root] && data.current) {
      remotes.value = {
        ...remotes.value,
        [root]: { ...remotes.value[root]!, branch: data.current },
      };
    }
  } catch {
    if (epoch === generation && branchRequests.get(root) === request)
      branchErrors.value = {
        ...branchErrors.value,
        [root]: t("workspace.branchesLoadFailed"),
      };
  } finally {
    if (epoch === generation && branchRequests.get(root) === request)
      branchLoading.value = { ...branchLoading.value, [root]: false };
  }
}

async function switchBranch(root: string, branch: string) {
  const remote = branch.startsWith("refs/remotes/");
  branch = branch.replace(remote ? /^refs\/remotes\// : /^refs\/heads\//, "");
  if (
    !api.gitSwitchBranch ||
    !branch ||
    branchSwitching.value[root] ||
    commitPending.value[root] ||
    pushPending.value[root] ||
    pullPending.value[root]
  )
    return;
  if (!remote && remotes.value[root]?.branch === branch) return;
  const epoch = generation;
  branchSwitching.value = { ...branchSwitching.value, [root]: true };
  branchErrors.value = { ...branchErrors.value, [root]: "" };
  try {
    const result = await api.gitSwitchBranch(
      props.sessionId,
      branch,
      root,
      remote,
    );
    if (epoch !== generation) return;
    if (result.switched) {
      const current = result.branch || branch;
      if (remotes.value[root])
        remotes.value = {
          ...remotes.value,
          [root]: { ...remotes.value[root]!, branch: current },
        };
      MessagePlugin.success(t("workspace.branchSwitched", { branch: current }));
      void ensureBranches(root, true);
      void refreshGitHistory(root, false, true);
      void refresh();
    } else {
      branchErrors.value = {
        ...branchErrors.value,
        [root]: result.detail || t("workspace.branchSwitchFailed"),
      };
    }
  } catch {
    if (epoch === generation)
      branchErrors.value = {
        ...branchErrors.value,
        [root]: t("workspace.branchSwitchFailed"),
      };
  } finally {
    if (epoch === generation)
      branchSwitching.value = { ...branchSwitching.value, [root]: false };
  }
}

async function confirmDiscard() {
  const root = discardForRepo.value;
  if (
    root === null ||
    !api.gitDiscard ||
    discardPending.value ||
    commitPending.value[root] ||
    pushPending.value[root] ||
    pullPending.value[root] ||
    branchSwitching.value[root]
  )
    return;
  const epoch = generation;
  discardPending.value = true;
  discardError.value = "";
  try {
    const result = await api.gitDiscard(props.sessionId, root);
    if (epoch !== generation) return;
    if (result.discarded) {
      discardForRepo.value = null;
      MessagePlugin.success(t("workspace.discardSuccess"));
      void refresh();
    } else {
      discardError.value = result.detail || t("workspace.discardFailed");
    }
  } catch {
    if (epoch === generation) discardError.value = t("workspace.discardFailed");
  } finally {
    if (epoch === generation) discardPending.value = false;
  }
}

// Undoing a local commit: the branch moves back to the target, while the
// dropped commits' changes stay in the working tree as uncommitted edits. The
// server refuses targets that were already pushed, so the dialog only warns
// about what will be restored, not about rewritten history.
const revertTarget = ref<{
  root: string;
  sha: string;
  shortSha: string;
} | null>(null);
const revertPending = ref(false);
const revertError = ref("");

function askRevert(root: string, commit: ProgrammingGitHistoryCommit) {
  if (!commit?.sha || commit.can_reset !== true) return;
  revertTarget.value = {
    root,
    sha: commit.sha,
    shortSha: commit.short_sha,
  };
  revertError.value = "";
}

async function confirmRevert() {
  const target = revertTarget.value;
  if (!target || !api.gitReset || revertPending.value) return;
  const epoch = generation;
  revertPending.value = true;
  revertError.value = "";
  try {
    const result = await api.gitReset(props.sessionId, target.sha, target.root);
    if (epoch !== generation) return;
    if (result.reset) {
      revertTarget.value = null;
      MessagePlugin.success(t("workspace.historyRevertSuccess"));
      void refresh();
      void refreshGitHistory(target.root, false, true);
      return;
    }
    const reasons = {
      not_on_branch: 'workspace.historyRevertNotOnBranch',
      already_at_commit: 'workspace.historyRevertAlreadyAtCommit',
      published_history: 'workspace.historyRevertPublished',
      dirty_tree: 'workspace.historyRevertDirty',
      git_error: 'workspace.historyRevertFailed',
    } as const;
    revertError.value = result.reason && result.reason !== 'git_error'
      ? t(reasons[result.reason])
      : result.detail || t("workspace.historyRevertFailed");
    void refreshGitHistory(target.root, false, true);
  } catch {
    if (epoch === generation)
      revertError.value = t("workspace.historyRevertFailed");
  } finally {
    if (epoch === generation) revertPending.value = false;
  }
}

// Remote strip above a repo's commit box: how that branch relates to its
// upstream. Opening an active workspace and periodic polls fetch fresh refs;
// local actions can still read cached refs while updating their own counts.

function remoteVisible(root: string) {
  const r = remotes.value[root];
  return gitLinked.value && !!r && r.available && r.branch !== "";
}
function pullVisible(root: string) {
  const r = remotes.value[root];
  return remoteVisible(root) && !!r && r.upstream !== "";
}

// A diverged branch can align with the remote after saving its local edits.
// Reapplying them can merge automatically or leave explicit conflicts to resolve.
function diverged(root: string) {
  const r = remotes.value[root];
  return pullVisible(root) && !!r && r.ahead > 0 && r.behind > 0;
}

function pushVisible(root: string) {
  const r = remotes.value[root];
  return (
    remoteVisible(root) &&
    !!r &&
    (r.ahead > 0 ||
      !r.upstream ||
      !!pushErrors.value[root] ||
      !!pushPending.value[root])
  );
}

async function submitPush(root: string, fromCommit = false, credentials?: ProgrammingGitPushCredentials) {
  if (
    pushPending.value[root] ||
    pullPending.value[root] ||
    branchSwitching.value[root] ||
    (!fromCommit && commitPending.value[root])
  )
    return;
  const epoch = generation;
  pushPending.value = { ...pushPending.value, [root]: true };
  pushErrors.value = { ...pushErrors.value, [root]: "" };
  try {
    if (!api.gitPush) throw new Error("push is unavailable");
    const result = await api.gitPush(props.sessionId, root, credentials);
    if (epoch !== generation) return;
    if (result.auth_required) {
      pushAuthForRepo.value = root;
      pushAuthMode.value = result.auth_mode === 'ssh' ? 'ssh' : 'http';
      pushAuthRemote.value = result.remote_url || '';
      pushAuthUsername.value = result.username || '';
      pushAuthPassword.value = '';
      pushAuthFromCommit.value = fromCommit;
      pushAuthError.value = result.auth_failed ? t('workspace.pushAuthExpired') : credentials ? t('workspace.pushAuthRemoteChanged') : '';
      if (fromCommit && !credentials) MessagePlugin.success(t('workspace.commitSuccess'));
      return;
    }
    if (result.pushed) {
      if (pushAuthForRepo.value === root) resetPushAuth();
      if (result.detail) MessagePlugin.warning(result.detail);
      MessagePlugin.success(
        t(
          fromCommit
            ? "workspace.commitAndPushSuccess"
            : "workspace.pushSuccess",
        ),
      );
      void refreshGitRemote(true, root);
      void refreshGitHistory(root, false, true);
    } else {
      pushErrors.value = {
        ...pushErrors.value,
        [root]: result.auth_failed
          ? t("workspace.pushAuthFailed") + (result.detail ? "\n" + result.detail : "")
          : result.detail || t("workspace.pushFailed"),
      };
      if (pushAuthForRepo.value === root) pushAuthError.value = pushErrors.value[root] || t("workspace.pushFailed");
      if (fromCommit)
        MessagePlugin.warning(t("workspace.commitLocalPushFailed"));
    }
  } catch (err: any) {
    if (epoch !== generation) return;
    pushErrors.value = {
      ...pushErrors.value,
      [root]: typeof err?.message === "string" ? err.message : t("workspace.pushFailed"),
    };
    if (pushAuthForRepo.value === root) pushAuthError.value = pushErrors.value[root] || t("workspace.pushFailed");
    if (fromCommit) MessagePlugin.warning(t("workspace.commitLocalPushFailed"));
  } finally {
    if (epoch === generation) {
      pushPending.value = { ...pushPending.value, [root]: false };
      // A rejected push still leaves a local commit to retry.
      void refreshGitRemote(false, root);
    }
  }
}

async function refreshGitRemote(refresh = false, root = "") {
  if (!api.gitRemote) return;
  // A cached read must not replace a fetch in flight with older ref counts.
  if (remoteLoading.value[root] && (!refresh || fetchingRemoteRoots.has(root)))
    return;
  const epoch = generation,
    request = (remoteRequests.get(root) || 0) + 1;
  remoteRequests.set(root, request);
  if (refresh) fetchingRemoteRoots.add(root);
  remoteLoading.value = { ...remoteLoading.value, [root]: true };
  try {
    const data = await api.gitRemote(props.sessionId, refresh, root);
    if (epoch !== generation || remoteRequests.get(root) !== request) return;
    remotes.value = { ...remotes.value, [root]: data };
    if (data.available) void refreshGitHistory(root);
    if (refresh) {
      remoteErrors.value = {
        ...remoteErrors.value,
        [root]:
          data.available && !data.fetched
            ? t("workspace.remoteFetchFailed")
            : "",
      };
      if (data.fetched) void ensureBranches(root, true);
    }
  } catch {
    if (epoch === generation && remoteRequests.get(root) === request)
      remoteErrors.value = {
        ...remoteErrors.value,
        [root]: t("workspace.remoteFetchFailed"),
      };
  } finally {
    if (epoch === generation && remoteRequests.get(root) === request) {
      fetchingRemoteRoots.delete(root);
      remoteLoading.value = { ...remoteLoading.value, [root]: false };
    }
  }
}

async function refreshGitHistory(root: string, more = false, reset = false) {
  if (!api.gitHistory || !historyOpen.value[root] || (historyLoading.value[root] && !reset)) return;
  const previous = gitHistories.value[root];
  if (more && !previous?.has_more) return;
  const epoch = generation, request = (historyRequests.get(root) || 0) + 1;
  historyRequests.set(root, request);
  historyLoading.value = { ...historyLoading.value, [root]: true };
  historyErrors.value = { ...historyErrors.value, [root]: "" };
  try {
    const page = more && previous
      ? { skip: previous.commits.length, head: previous.head, upstream: previous.upstream }
      : undefined;
    const data = await api.gitHistory(props.sessionId, root, page);
    if (epoch !== generation || historyRequests.get(root) !== request) return;
    let report = data;
    if (more && previous) {
      const seen = new Set(previous.commits.map((commit) => commit.sha));
      report = { ...data, commits: [...previous.commits, ...data.commits.filter((commit) => !seen.has(commit.sha))] };
    } else if (!reset && previous && previous.head === data.head && previous.upstream === data.upstream && previous.commits.length > data.commits.length) {
      // Preserve loaded older pages during background refreshes of the same
      // snapshot, while refreshing decorations on the records we reread.
      const updates = new Map(data.commits.map((commit) => [commit.sha, commit]));
      report = { ...data, has_more: previous.has_more, commits: previous.commits.map((commit) => updates.get(commit.sha) ?? commit) };
    }
    gitHistories.value = { ...gitHistories.value, [root]: report };
  } catch {
    if (epoch === generation && historyRequests.get(root) === request)
      historyErrors.value = { ...historyErrors.value, [root]: t("workspace.historyLoadFailed") };
  } finally {
    if (epoch === generation && historyRequests.get(root) === request)
      historyLoading.value = { ...historyLoading.value, [root]: false };
  }
}

function canPollRemotes() {
  return (
    !!api.gitRemote &&
    props.active &&
    !!props.sessionId &&
    !!gitRepos.value?.length &&
    (typeof document === "undefined" || document.visibilityState !== "hidden")
  );
}

async function pollRemotes(epoch: number) {
  if (epoch !== remotePollGeneration || !canPollRemotes()) return;
  await Promise.all(
    (gitRepos.value ?? []).map((repo) => refreshGitRemote(true, repo.root)),
  );
  if (epoch !== remotePollGeneration || !canPollRemotes()) return;
  remotePollTimer = setTimeout(() => {
    void pollRemotes(epoch);
  }, 60_000);
}

function restartRemotePolling() {
  const epoch = ++remotePollGeneration;
  clearTimeout(remotePollTimer);
  if (canPollRemotes())
    remotePollTimer = setTimeout(() => {
      void pollRemotes(epoch);
    }, 250);
}

watch(
  () =>
    [
      props.sessionId,
      props.active,
      gitRepos.value?.map((repo) => repo.root).join("\x00") ?? "",
    ] as const,
  restartRemotePolling,
  { immediate: true },
);
onMounted(() => {
  if (typeof document !== "undefined")
    document.addEventListener("visibilitychange", restartRemotePolling);
  if (typeof window !== "undefined")
    window.addEventListener("focus", restartRemotePolling);
});

async function submitPull(root: string, realign = false) {
  if (
    !api.gitPull ||
    pullPending.value[root] ||
    pushPending.value[root] ||
    commitPending.value[root] ||
    branchSwitching.value[root]
  )
    return;
  const epoch = generation;
  pullPending.value = { ...pullPending.value, [root]: true };
  pullErrors.value = { ...pullErrors.value, [root]: "" };
  try {
    const result = await api.gitPull(props.sessionId, root, realign);
    if (epoch !== generation) return;
    if (result.pulled) {
      if (result.conflicts || result.restore_failed) {
        const detail = result.conflicts ? t('workspace.pullAlignedConflicts') : result.detail || t('workspace.pullRestoreFailed');
        pullErrors.value = { ...pullErrors.value, [root]: detail };
        MessagePlugin.warning(detail);
        fileView.value = 'diff';
      } else {
        pullErrors.value = { ...pullErrors.value, [root]: '' };
        MessagePlugin.success(t(result.aligned ? "workspace.pullAligned" : "workspace.pullSuccess"));
      }
      void refreshGitHistory(root, false, true);
      // A pull updates HEAD and files; reload the open file and tree as well
      // as status and the remote strip.
      void refresh();
    } else {
      pullErrors.value = {
        ...pullErrors.value,
        [root]: result.detail || t("workspace.pullFailed"),
      };
      if (realign) {
        void refresh();
        void refreshGitHistory(root, false, true);
      }
    }
  } catch {
    if (epoch !== generation) return;
    pullErrors.value = {
      ...pullErrors.value,
      [root]: t("workspace.pullFailed"),
    };
  } finally {
    if (epoch === generation)
      pullPending.value = { ...pullPending.value, [root]: false };
  }
}

async function refresh() {
  await refreshInternal(false);
}

/** Reload the tree, then reopen either the focus target or the current file. */
async function refreshInternal(preferFocus: boolean) {
  const epoch = generation,
    request = ++refreshRequest;
  const loaded = Object.keys(childrenByPath.value);
  // Snapshot git status in parallel with the directory loads below: the
  // changes panel's diff button depends only on this probe, so it appears as
  // soon as git answers instead of waiting for every directory to reload.
  void refreshGitStatus();
  await Promise.all(
    (loaded.length ? loaded : [""]).map((path) => loadDirectory(path)),
  );
  if (epoch !== generation || request !== refreshRequest || error.value) return;
  const focus = relativePath(props.focusPath);
  const restored = savedOpenFile();
  // A live-edit path may still carry the workspace root (the tree is not
  // loaded when the agent reports it); strip it now that root is known.
  const rawLiveFocus = pendingDiffPath;
  pendingDiffPath = "";
  const liveFocus = rawLiveFocus
    ? relativePath(rawLiveFocus) ||
      (rawLiveFocus.startsWith("/") ? "" : rawLiveFocus)
    : "";
  // A manual refresh keeps the file the user opened; an agent-driven refresh
  // follows focusPath (or a live edit) so the view jumps to whichever file is
  // changing, even when the panel only mounted after the change was recorded.
  const follow = preferFocus && focus ? focus : liveFocus;
  const candidate =
    follow || file.value?.path || restored || focus || defaultFile();
  if (!candidate) return;
  await revealPath(candidate);
  if (epoch !== generation || request !== refreshRequest) return;
  // An agent-driven refresh lands on the changed file and shows its diff; a
  // manual refresh keeps whatever view the reader was on.
  await openFile(candidate, follow ? "diff" : undefined);
}

// Live edit: follow the file the agent just wrote. The store keeps one entry
// per path, so A → B → A lands on the right file each time instead of staying
// on whatever was open when the turn started.
let lastFollowedChange = "";
watch(
  [activeFileChange, gitLinked],
  async ([id, linked]) => {
    if (!linked) return;
    if (!id || id === lastFollowedChange) return;
    lastFollowedChange = id;
    // The live edit can arrive before the next Git status snapshot includes
    // its path. Select Changes from the recorded event instead of waiting
    // for the merged dirty-file list to catch up.
    const change = fileChanges.value.find((item) => item.id === id);
    if (!change?.live) return;
    fileView.value = "diff";
    // Stash the raw path before awaiting: a fresh mount runs the refresh watcher
    // in the same tick, and it must find this file rather than a restored one.
    pendingDiffPath = change.path;
    if (!root.value) await loadDirectory("");
    const target = relativePath(change.path) || change.path;
    // An absolute path that does not sit under the tree root is not addressable
    // by the file API; the agent-driven refresh above is the fallback for it.
    if (!target || target.startsWith("/")) return;
    await revealPath(target);
    await openFile(target, "diff");
  },
  { immediate: true },
);

async function preparePreview() {
  const current = file.value,
    request = ++previewRequest,
    epoch = generation,
    session = props.sessionId;
  previewHtml.value = "";
  previewWarnings.value = [];
  previewError.value = "";
  previewLoading.value = false;
  if (!current || !canPreviewSource(current.path)) return;
  previewLoading.value = true;
  try {
    let source = current.content;
    if (/\.(?:md|markdown)$/i.test(current.path)) {
      source = `<html><head><style>body{font:15px/1.8 system-ui;padding:24px;color:var(--td-text-color-primary,#292824);max-width:760px;margin:auto;background:var(--td-bg-color-container,#fff)}pre{white-space:pre-wrap;background:var(--td-bg-color-secondarycontainer,#f5f5f3);padding:16px}img{max-width:100%}::selection{background:#cfe1f7}@media(prefers-color-scheme:dark){body{color:#e5e5e5;background:#141414}pre{background:#2a2a2a}::selection{background:#365073;color:#e8e8e8}}</style></head><body>${sanitizeMarkdownHTML(marked.parse(source, { async: false }) as string)}</body></html>`;
    }
    const result = await buildWorkspacePreview(
      current.path,
      source,
      async (path) => (await api.file(session, path)).content,
    );
    if (epoch !== generation || request !== previewRequest) return;
    previewHtml.value = result.html;
    previewWarnings.value = result.warnings;
    previewVersion.value++;
  } catch {
    if (epoch === generation && request === previewRequest)
      previewError.value = t("workspace.previewError");
  } finally {
    if (epoch === generation && request === previewRequest)
      previewLoading.value = false;
  }
}
async function copySource() {
  try {
    await navigator.clipboard.writeText(file.value?.content || "");
    copied.value = true;
    copyError.value = "";
  } catch {
    copyError.value = t("workspace.copyFailed");
  }
}
watch(
  () => props.sessionId,
  () => {
    generation++;
    gitLinked.value = false;
    gitStatusLoading.value = false;
    gitRepos.value = null;
    commitMessages.value = {};
    commitPending.value = {};
    commitErrors.value = {};
    commitPushRequested.value = {};
    commitSelections.value = {};
    pushPending.value = {};
    pushErrors.value = {};
    remotes.value = {};
    remoteLoading.value = {};
    remoteErrors.value = {};
    gitHistories.value = {};
    historyLoading.value = {};
    historyErrors.value = {};
    historyOpen.value = {};
    historyRequests.clear();
    fetchingRemoteRoots.clear();
    pullPending.value = {};
    pullErrors.value = {};
    resetPushAuth();
    branchOptions.value = {};
    remoteBranchOptions.value = {};
    branchLoading.value = {};
    branchRequests.clear();
    branchSwitching.value = {};
    branchErrors.value = {};
    discardForRepo.value = null;
    discardPending.value = false;
    discardError.value = "";
    remoteRequests.clear();
    fileRequest++;
    previewRequest++;
    treeRequests.clear();
    nodes.value = [];
    file.value = null;
    directory.value = "";
    root.value = "";
    error.value = "";
    childrenByPath.value = {};
    expandedPaths.value = [];
    pendingPaths.value = [];
    workspaceOrigin.value = "sandbox";
    workspaceState.value = "live";
    fileView.value = "source";
    previewHtml.value = "";
    previewWarnings.value = [];
    previewError.value = "";
    previewLoading.value = false;
  },
);
let lastAutoRefresh = { sessionId: "", revision: -1 };
watch(
  () => [props.sessionId, props.active, props.revision] as const,
  ([sessionId, active, revision]) => {
    clearTimeout(refreshTimer);
    if (!active || !sessionId) return;
    // Follow the freshly-written file only when the agent changed something in
    // this same session; a fresh mount, a session switch, or a plain tab switch
    // restores/keeps the file the user last had open.
    const isNewSession = sessionId !== lastAutoRefresh.sessionId;
    const revisionChanged = revision !== lastAutoRefresh.revision;
    lastAutoRefresh = { sessionId, revision };
    const followFocus = revisionChanged && !isNewSession;
    // A brand-new mount has nothing to debounce (revision is not yet streaming
    // in), so the changes panel's git probe starts immediately; only rapid
    // revision changes from a live turn need the delay.
    const delay = isNewSession ? 0 : 180;
    refreshTimer = setTimeout(() => {
      void refreshInternal(followFocus);
    }, delay);
  },
  { immediate: true },
);
watch(
  () => showFilePreview.value,
  (preview) => {
    if (preview) void preparePreview();
    else {
      previewRequest++;
      previewLoading.value = false;
    }
  },
);
onBeforeUnmount(() => {
  generation++;
  resetPushAuth();
  remotePollGeneration++;
  clearTimeout(remotePollTimer);
  if (typeof document !== "undefined")
    document.removeEventListener("visibilitychange", restartRemotePolling);
  if (typeof window !== "undefined")
    window.removeEventListener("focus", restartRemotePolling);
  clearTimeout(refreshTimer);
  treeRequests.clear();
});
</script>

<style scoped lang="less">
.conversation-workspace {
  flex: 1;
  min-height: 0;
  min-width: 0;
  display: flex;
  flex-direction: column;
  color: var(--td-text-color-primary);
  font-size: var(--app-text-sm);
  --ws-code-keyword: #d73a49;
  --ws-code-entity: #348080;
  --ws-code-string: #0f2f63;
  --ws-code-accent: #245cc5;
  --ws-code-comment: #6a737d;
}

button {
  font: inherit;
  color: inherit;
  cursor: pointer;
  border: 0;
  background: transparent;
  border-radius: var(--app-radius-sm);
  transition: background var(--app-motion-base) ease;

  // &:hover {
  //   background: var(--td-bg-color-container-hover);
  // }

  &:focus-visible {
    outline: 2px solid var(--td-text-color-secondary);
    outline-offset: -2px;
  }

  &:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
}

.icon-button {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  border: 1px solid transparent;

  &.active,
  &.is-active {
    background: var(--td-bg-color-secondarycontainer);
  }
}

.workspace-body {
  flex: 1;
  min-height: 0;
  display: flex;
}

.file-tree {
  width: clamp(132px, 28%, 220px);
  // The divider can widen the tree, but the code view always keeps room.
  max-width: calc(100% - 200px);
  flex-shrink: 0;
  position: relative;
  min-height: 0;
  display: flex;
  flex-direction: column;
  border-right: 1px solid var(--td-component-stroke);
}

.file-tree__scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 8px 6px 16px;
}

.file-tree__hint {
  padding: 10px 8px;
  color: var(--td-text-color-placeholder);
  font-size: var(--app-text-2xs);
  line-height: 1.8;
}

.file-tree__list {
  list-style: none;
  margin: 0;
  padding: 0;
}

.file-tree__row {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 28px;
  padding: 4px 8px;
  text-align: left;
  color: var(--td-text-color-primary);
  font-size: var(--app-text-md);
  border-radius: var(--app-radius-sm);

  &.is-dir {
    font-weight: 500;
  }

  &.is-selected {
    background: var(--td-bg-color-secondarycontainer-active);
  }
}

.file-tree__lead {
  width: 14px;
  height: 14px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
  color: var(--td-text-color-secondary);
}

.file-tree__glyph {
  font-family: var(--app-font-family-mono);
  font-size: var(--app-text-2xs);
  font-weight: 600;
  line-height: 1;
  color: var(--td-text-color-secondary);
}

.file-tree__name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}

.workspace-main {
  flex: 1;
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.workspace-notice {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 12px 16px;
  margin: 0;
  color: var(--td-text-color-secondary);
  background: var(--td-bg-color-secondarycontainer);
  font-size: var(--app-text-xs);
  line-height: 1.8;
  overflow-wrap: anywhere;
  flex-shrink: 0;

  .t-icon {
    margin-top: 4px;
    flex-shrink: 0;
  }

  button {
    margin-left: auto;
    white-space: nowrap;
    text-decoration: underline;
  }
}

.workspace-empty {
  flex: 1;
  min-height: 260px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  padding: 32px;
  text-align: center;

  h2 {
    font-size: var(--app-text-2xl);
    font-weight: 500;
    margin: 24px 0 8px;
  }

  p {
    max-width: 290px;
    font-size: var(--app-text-sm);
    line-height: 1.9;
    color: var(--td-text-color-secondary);
  }
}

.empty-art {
  width: 80px;
  height: 68px;
  border: 1px solid var(--td-component-border);
  border-radius: var(--app-radius-lg);
  transform: rotate(-6deg);
  display: grid;
  place-items: center;
  position: relative;
  background: var(--td-bg-color-container);

  &:before {
    content: "";
    position: absolute;
    inset: -9px 8px 8px -8px;
    border: 1px solid var(--td-component-stroke);
    border-radius: var(--app-radius-lg);
    z-index: -1;
    transform: rotate(12deg);
  }

  span {
    font: 24px monospace;
    color: var(--td-text-color-secondary);
  }
}

.empty-steps {
  display: flex;
  gap: 12px;
  align-items: center;
  margin-top: 28px;
  font-size: var(--app-text-2xs);
  color: var(--td-text-color-placeholder);

  span {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  b {
    font-weight: 400;
    font-size: var(--app-text-xs);
  }

  i {
    width: 18px;
    border-top: 1px solid var(--td-component-border);
  }
}

.file-status {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  padding: 6px 10px;
  border-top: 1px solid var(--td-component-stroke);
  font-size: 11px;
  color: var(--td-text-color-placeholder);
}
.file-toolbar {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  padding: 8px 10px;
  border-bottom: 1px solid var(--td-component-stroke);
}
.file-toolbar__path {
  flex: 1;
  min-width: 60px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: var(--app-font-family-mono, monospace);
  font-size: 12px;
}
.file-toolbar button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 0;
  border-radius: 5px;
  padding: 6px;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  font: inherit;
}
.file-toolbar button:hover,
.file-toolbar button[aria-pressed="true"] {
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-primary);
}
.file-toolbar button:focus-visible {
  outline: 2px solid var(--td-brand-color);
  outline-offset: 2px;
}
.file-view-switch {
  display: flex;
  gap: 2px;
  padding: 2px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 7px;
}
.diff-file-list {
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow: auto;
  padding: 0 10px 16px;
}
// The expanded history panel fills the whole area below the remote strip: the
// repo group becomes a column so the panel can take all the leftover height,
// while every other block keeps its natural height.
.diff-file-list > .repo-group {
  flex: 1 1 auto;
  min-height: 0;
  display: flex;
  flex-direction: column;
}
.repo-group > :not(.git-history) {
  flex: none;
}
.remote-strip {
  position: sticky;
  top: 0;
  z-index: 1;
  padding: 8px 0 4px;
  background: var(--td-bg-color-container);
  font-size: 12px;
}
.remote-strip__row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.remote-strip__branch-select {
  flex: 0 1 180px;
  min-width: 112px;
  max-width: 220px;
  font-family: var(--app-font-family-mono, monospace);
  font-size: 12px;

  :deep(.t-input) {
    height: 28px;
    padding: 0 8px;
    border-radius: 7px;
    border-color: var(--td-component-stroke);
    background: var(--td-bg-color-secondarycontainer);
    font: inherit;
    transition:
      background 0.15s ease,
      border-color 0.15s ease;
  }
  :deep(.t-input:hover),
  :deep(.t-input--focused) {
    border-color: var(--td-brand-color);
    background: var(--td-bg-color-container);
  }
  :deep(.t-input__prefix) {
    color: var(--td-text-color-secondary);
  }
  :deep(.t-input__inner) {
    font: inherit;
  }
}
.remote-strip__discard:not(:disabled) {
  color: var(--td-error-color);
}
.remote-strip__branch {
  font-family: var(--app-font-family-mono, monospace);
  color: var(--td-text-color-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.remote-strip--loading {
  padding: 8px 10px;
}
.remote-strip__placeholder {
  display: inline-flex;
  align-items: center;
  width: 180px;
  height: 28px;
  padding: 0 8px;
  border: 1px solid var(--td-component-border);
  border-radius: 5px;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-placeholder);
}
.remote-strip__loading-text {
  color: var(--td-text-color-placeholder);
  font-size: 12px;
  margin-right: auto;
}
.remote-strip__arrow {
  flex: none;
  color: var(--td-text-color-placeholder);
}
.remote-strip__upstream {
  font-family: var(--app-font-family-mono, monospace);
  color: var(--td-text-color-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.remote-strip__no-upstream {
  color: var(--td-text-color-placeholder);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.remote-strip__badge {
  flex: none;
  padding: 0 5px;
  border-radius: 8px;
  font-size: 11px;
  line-height: 16px;
  font-family: var(--app-font-family-mono, monospace);
}
.remote-strip__badge--ahead {
  background: var(--td-success-color-light);
  color: var(--td-success-color);
}
.remote-strip__badge--behind {
  background: var(--td-warning-color-light);
  color: var(--td-warning-color);
}
.remote-strip__action {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  flex: none;
  margin-left: auto;
  padding: 3px 7px;
  border: 0;
  border-radius: 5px;
  background: transparent;
  color: var(--td-text-color-secondary);
  cursor: pointer;
  font: inherit;
}
.remote-strip__action:hover:not(:disabled) {
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-primary);
}
.remote-strip__action:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.remote-strip__row .remote-strip__action:first-of-type {
  margin-left: auto;
}
.remote-strip__row .remote-strip__action + .remote-strip__action {
  margin-left: 0;
}
.remote-strip__pull:not(:disabled) {
  color: var(--td-brand-color);
}
.remote-strip__push:not(:disabled) {
  color: var(--td-brand-color);
}
.remote-strip__error {
  margin: 4px 0 0;
  color: var(--td-error-color);
  font-size: 12px;
  line-height: 1.5;
  /* Same wrapping as the commit error: long git output must not stretch the
   * strip or hide the pull action. */
  white-space: pre-wrap;
  word-break: break-word;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  overflow: hidden;
}
.remote-strip__notice {
  margin: 6px 0 0;
  color: var(--td-brand-color);
  font-size: 12px;
  line-height: 1.5;
}
.remote-strip__incoming {
  margin: 6px 0 0;
  padding: 0;
  list-style: none;
  border-top: 1px dashed var(--td-component-stroke);
}
.remote-strip__commit {
  display: flex;
  align-items: baseline;
  gap: 7px;
  padding: 3px 0 0;
  min-width: 0;
}
.remote-strip__sha {
  flex: none;
  font-family: var(--app-font-family-mono, monospace);
  font-size: 11px;
  color: var(--td-text-color-secondary);
}
.remote-strip__subject {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--td-text-color-primary);
}
.commit-box {
  padding: 6px 0 10px;
  background: var(--td-bg-color-container);
}
.commit-box__selection {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px 0 2px;
  color: var(--td-text-color-secondary);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
}
.commit-box__input {
  box-sizing: border-box;
  width: 100%;
  resize: vertical;
  min-height: 56px;
  padding: 7px 9px;
  border: 1px solid var(--td-component-stroke);
  border-radius: 6px;
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-primary);
  font: inherit;
  font-size: 12px;
  line-height: 1.5;
}
.commit-box__input:focus {
  outline: none;
  border-color: var(--td-brand-color);
}
.commit-box__input:disabled {
  opacity: 0.6;
}
.commit-box__footer {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 6px;
}
.commit-box__actions {
  display: flex;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: 6px;
  margin-left: auto;
}
.commit-box__error {
  flex: 1;
  min-width: 0;
  color: var(--td-error-color);
  font-size: 12px;
  line-height: 1.5;
  /* Wrap long git errors (e.g. the missing-identity hint) instead of letting
   * one line push the submit button off the panel. Clamp to 3 lines with an
   * ellipsis so a hook dump still cannot flood the box. */
  white-space: pre-wrap;
  word-break: break-word;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  overflow: hidden;
}
.commit-box__submit {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  margin-left: auto;
  flex: none;
  align-self: flex-start;
  padding: 5px 12px;
  border: 0;
  border-radius: 6px;
  background: var(--td-brand-color);
  color: var(--td-text-color-anti);
  cursor: pointer;
  font: inherit;
  font-size: 12px;
  white-space: nowrap;
}
.commit-box__submit:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.commit-box__submit:not(:disabled):hover {
  background: var(--td-brand-color-hover);
}
.commit-box__submit--local {
  background: var(--td-bg-color-secondarycontainer);
  color: var(--td-text-color-secondary);
}
.commit-box__submit--local:not(:disabled):hover {
  background: var(--td-bg-color-container-hover);
}
.push-auth__remote {
  font-size: 12px;
  color: var(--td-text-color-secondary);
  overflow-wrap: anywhere;
  padding: 8px 10px;
  background: var(--td-bg-color-secondarycontainer);
  border-radius: 6px;
}
.identity-dialog__hint {
  margin: 0 0 12px;
  color: var(--td-text-color-secondary);
  font-size: 13px;
  line-height: 1.5;
}
.identity-dialog__field {
  margin-bottom: 12px;
}
.identity-dialog__label {
  display: block;
  margin-bottom: 4px;
  color: var(--td-text-color-primary);
  font-size: 13px;
}
.identity-dialog__error {
  margin: 4px 0 0;
  color: var(--td-error-color);
  font-size: 12px;
  white-space: pre-wrap;
  word-break: break-word;
}

/* 干净工作区空态：沿用 .workspace-empty 的插画 + 标题 + 副文案节奏。 */
.diff-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  min-height: 320px;
  padding: var(--app-space-10) var(--app-space-6);
}

.diff-empty__icon {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  border-radius: var(--app-radius-pill);
  color: var(--td-brand-color);
  background: color-mix(in srgb, var(--td-brand-color) 8%, transparent);
  box-shadow: 0 0 0 10px
    color-mix(in srgb, var(--td-brand-color) 4%, transparent);
}

.diff-empty__hint {
  margin: var(--app-space-5) 0 0;
  max-width: 300px;
  font-size: var(--app-text-sm);
  line-height: 1.8;
  color: var(--td-text-color-secondary);
}
.diff-empty--history {
  min-height: 80px;
  padding: 16px 12px;
}
.diff-empty--history .diff-empty__icon {
  width: 30px;
  height: 30px;
  box-shadow: none;
  background: transparent;
}
.diff-empty--history .diff-empty__hint {
  margin-top: 8px;
  font-size: 12px;
}
.source-scroll {
  overflow: auto;
  flex: 1;
  min-height: 0;
  background: var(--td-bg-color-secondarycontainer);
  padding: 14px 0 24px;

  pre {
    margin: 0;
    font: var(--app-text-md) / 1.4 var(--app-font-family-mono, monospace);
    tab-size: 2;
  }
}

.source-lines {
  width: 100%;
  min-width: 0;
}
.source-row {
  display: grid;
  grid-template-columns: 4.5em minmax(0, 1fr);
}
.source-gutter {
  font: var(--app-text-md) / 1.4 var(--app-font-family-mono, monospace);
  color: var(--td-text-color-disabled);
  text-align: right;
  padding: 0 14px 0 16px;
  user-select: none;
  background: var(--td-bg-color-secondarycontainer);
}

.source-code {
  min-width: 0;
  max-height: none;
  overflow: visible;
  padding: 0 16px 0 8px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  code {
    font: inherit;
    white-space: inherit;
  }
}

.source-code {
  :deep(.hljs-keyword),
  :deep(.hljs-doctag),
  :deep(.hljs-meta) {
    color: var(--ws-code-keyword);
  }

  :deep(.hljs-string),
  :deep(.hljs-regexp) {
    color: var(--ws-code-string);
  }

  :deep(.hljs-comment),
  :deep(.hljs-quote) {
    color: var(--ws-code-comment);
  }

  :deep(.hljs-number),
  :deep(.hljs-literal),
  :deep(.hljs-operator),
  :deep(.hljs-punctuation),
  :deep(.hljs-params) {
    color: var(--ws-code-accent);
  }

  :deep(.hljs-title),
  :deep(.hljs-name),
  :deep(.hljs-built_in),
  :deep(.hljs-type),
  :deep(.hljs-section),
  :deep(.hljs-selector-tag),
  :deep(.hljs-selector-id),
  :deep(.hljs-selector-pseudo),
  :deep(.hljs-selector-class),
  :deep(.hljs-variable) {
    color: var(--ws-code-entity);
  }

  :deep(.hljs-attr),
  :deep(.hljs-attribute),
  :deep(.hljs-property),
  :deep(.hljs-tag) {
    color: var(--ws-code-accent);
  }
}

:global(:root[theme-mode="dark"] .conversation-workspace) {
  --ws-code-keyword: #ff7b72;
  --ws-code-entity: #56d4dd;
  --ws-code-string: #a5d6ff;
  --ws-code-accent: #79c0ff;
  --ws-code-comment: #8b949e;
}

:global(:root[theme-mode="dark"])
  .conversation-workspace
  .file-tree__row.is-selected {
  background: var(--td-bg-color-container-active);
}

:global(:root[theme-mode="dark"])
  .conversation-workspace
  .preview-stage
  iframe {
  background: var(--td-bg-color-page);
}

:global(:root[theme-mode="dark"]) .conversation-workspace .empty-art {
  background: var(--td-bg-color-secondarycontainer);
}

:global(:root[theme-mode="dark"]) .conversation-workspace .workspace-notice {
  background: var(--td-bg-color-container-active);
  color: var(--td-text-color-secondary);
}

:global(:root[theme-mode="dark"]) .conversation-workspace .run-command {
  color: var(--td-text-color-primary);
}

:global(:root[theme-mode="dark"]) .conversation-workspace .file-tree__hint {
  color: var(--td-text-color-disabled);
}

.image-preview {
  flex: 1;
  min-height: 0;
  min-width: 0;
  overflow: auto;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
  background: var(--td-bg-color-secondarycontainer);

  img {
    display: block;
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
  }
}

.preview-stage {
  flex: 1;
  min-height: 0;
  padding: 0;
  display: flex;
  justify-content: center;
  background: var(--td-bg-color-secondarycontainer);

  iframe {
    flex: 1;
    min-width: 0;
    width: 100%;
    height: 100%;
    border: 0;
    background: var(--td-bg-color-container, #fff);
  }

  &.mobile {
    padding: 16px 12px;

    iframe {
      max-width: 375px;
      border-radius: var(--app-radius-2xl);
      border: 1px solid var(--td-component-border);
    }
  }
}

.preview-loading {
  padding: 24px;
  color: var(--td-text-color-placeholder);
}

.spinning {
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to {
    transform: rotate(360deg);
  }
}

@keyframes pulse {
  to {
    opacity: 0.3;
  }
}

@media (prefers-reduced-motion: reduce) {
  *,
  *::before {
    animation: none !important;
    transition: none !important;
  }
}
</style>

<!-- TDesign renders its popup outside the component's scoped style boundary. -->
<style lang="less">
.workspace-branch-popup {
  .t-popup__content {
    padding: 6px;
    border: 1px solid var(--td-component-stroke);
    border-radius: 10px;
    box-shadow: var(--td-shadow-2);
  }
  .t-select-option-group__header {
    height: 30px;
    padding: 0 10px;
    color: var(--td-text-color-placeholder);
    font-size: 11px;
    font-weight: 500;
    line-height: 30px;
  }
  .t-select-option {
    height: 36px;
    margin: 2px 0;
    padding: 0 10px;
    border-radius: 6px;
    line-height: 36px;
  }
  .branch-option {
    display: flex;
    align-items: center;
    gap: 9px;
    min-width: 0;
    width: 100%;
  }
  .branch-option__icon {
    flex: none;
    color: var(--td-text-color-placeholder);
  }
  .branch-option__name {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    font-family: var(--app-font-family-mono, monospace);
    font-size: 12px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .branch-option__current {
    flex: none;
    padding: 1px 6px;
    border-radius: 4px;
    background: var(--td-brand-color-light);
    color: var(--td-brand-color);
    font-size: 10px;
    line-height: 16px;
  }
  .t-select-option.t-is-selected .branch-option__icon {
    color: var(--td-brand-color);
  }
}
</style>
