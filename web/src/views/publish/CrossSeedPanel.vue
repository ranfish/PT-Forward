<template>
  <a-drawer
    :open="open"
    :title="null"
    width="92%"
    placement="right"
    :body-style="{ padding: '0' }"
    :header-style="{ display: 'none' }"
    :mask-closable="!taskRunning"
    :keyboard="!taskRunning"
    destroy-on-close
    @close="handleClose"
  >
    <div class="csp-shell">
      <!-- ═══ Header ═══ -->
      <div class="csp-header">
        <div class="csp-header-left">
          <span class="csp-header-title">转种发布</span>
          <a-tag v-if="selectedTorrent" color="blue">{{ selectedTorrent.name }}</a-tag>
        </div>
        <div class="csp-header-right">
          <!-- 源站切换 -->
          <a-select
            v-if="cachedSites.length > 0"
            v-model:value="currentSourceSite"
            size="small"
            style="width: 200px"
            placeholder="选择源站"
            @change="onSourceSiteChange"
          >
            <a-select-option v-for="s in cachedSites" :key="s.siteName" :value="s.siteName">
              {{ s.siteName }}
              <CheckCircleFilled v-if="s.reviewed" style="color: #52c41a; margin-left: 4px" />
            </a-select-option>
          </a-select>
        </div>
      </div>

      <!-- ═══ Steps ═══ -->
      <div v-if="!maintenanceOnly" class="csp-steps">
        <a-steps :current="currentStep" size="small" style="padding: 8px 24px">
          <a-step title="编辑详情" />
          <a-step title="参数预览" />
          <a-step title="选择站点" />
          <a-step title="发布结果" />
        </a-steps>
      </div>

      <!-- ═══ Body ═══ -->
      <div ref="bodyRef" class="csp-body">
        <!-- Loading -->
        <div v-if="loading" class="csp-loading">
          <a-spin size="large" />
          <p style="margin-top: 16px; color: #666">{{ loadingText }}</p>
          <a-progress v-if="loadingProgress > 0" :percent="loadingProgress" status="active" style="max-width: 400px; margin-top: 12px" />
        </div>

        <!-- Error -->
        <a-result v-else-if="loadError" status="error" :title="loadError">
          <template #extra>
            <a-button @click="handleClose">关闭</a-button>
          </template>
        </a-result>

        <!-- Step 0: 编辑详情（5 Tab） -->
        <div v-else-if="currentStep === 0" class="csp-step-content">
          <!-- §59.20 ⑨: maintenanceOnly 预览模式 -->
          <template v-if="maintenanceOnly && seedPreviewMode">
            <div style="max-width: 1100px">
              <a-typography-title :level="5">发布预览</a-typography-title>

              <!-- 标题 + 副标题 -->
              <div style="margin-bottom: 16px">
                <h3 style="margin: 0">{{ form.title || '—' }}</h3>
                <div v-if="form.subtitle" style="color: #666; font-size: 14px">{{ form.subtitle }}</div>
              </div>

              <!-- §59.81: v1.05 全字段参数区（4 列紧凑） -->
              <a-descriptions :column="4" bordered size="small" style="margin-bottom: 12px">
                <a-descriptions-item label="季集">{{ pv('season_episode') }}</a-descriptions-item>
                <a-descriptions-item label="年份">{{ pv('year') }}</a-descriptions-item>
                <a-descriptions-item label="分辨率">{{ pv('resolution') }}</a-descriptions-item>
                <a-descriptions-item label="HDR">{{ pv('hdr') }}</a-descriptions-item>
                <a-descriptions-item label="bit">{{ pv('bit_depth') }}</a-descriptions-item>
                <a-descriptions-item label="视频编码">{{ pv('video_codec') }}</a-descriptions-item>
                <a-descriptions-item label="音频编码">{{ pv('audio_codec') }}</a-descriptions-item>
                <a-descriptions-item label="声道">{{ pv('audio_channels') }}</a-descriptions-item>
                <a-descriptions-item label="对象信息">{{ pv('audio_tech') }}</a-descriptions-item>
                <a-descriptions-item label="音轨数">{{ pv('audio_tracks') }}</a-descriptions-item>
                <a-descriptions-item label="片源">{{ pv('source_type') }}</a-descriptions-item>
                <a-descriptions-item label="规格">{{ pv('specification') }}</a-descriptions-item>
                <a-descriptions-item label="分发方">{{ pv('source_platform') }}</a-descriptions-item>
                <a-descriptions-item label="版本">{{ pv('edition_info') }}</a-descriptions-item>
                <a-descriptions-item label="地区码">{{ pv('region_code') }}</a-descriptions-item>
                <a-descriptions-item label="Encode">{{ pv('encode') }}</a-descriptions-item>
              </a-descriptions>

              <!-- §59.81: 产地 / 类型 -->
              <div v-if="pvRegion.length || pvGenre.length" style="margin-bottom: 12px">
                <span v-if="pvRegion.length" style="margin-right: 16px">
                  产地：<a-tag v-for="r in pvRegion" :key="r" color="geekblue">{{ r }}</a-tag>
                </span>
                <span v-if="pvGenre.length">
                  类型：<a-tag v-for="g in pvGenre" :key="g" color="purple">{{ g }}</a-tag>
                </span>
              </div>

              <!-- §59.81: 标签区（着色: 禁转类红 / 其余蓝） -->
              <div v-if="previewTags.length" style="margin-bottom: 12px">
                标签：
                <a-tag v-for="t in previewTags" :key="t" :color="isRestrictedTag(t) ? 'red' : 'blue'">
                  {{ tagDisplayName(t) }}
                </a-tag>
              </div>

              <!-- 海报 -->
              <div v-if="form.poster" style="margin-bottom: 16px; text-align: center">
                <img :src="form.poster" style="max-height: 300px; border-radius: 4px" />
              </div>

              <!-- MediaInfo -->
              <div v-if="form.mediaInfo" style="margin-bottom: 16px">
                <div style="font-weight: 600; margin-bottom: 4px">MediaInfo</div>
                <pre style="background: #f5f5f5; padding: 12px; border-radius: 4px; font-size: 12px; max-height: 300px; overflow: auto; white-space: pre-wrap">{{ form.mediaInfo }}</pre>
              </div>

              <!-- BDInfo -->
              <div v-if="form.bdinfo" style="margin-bottom: 16px">
                <div style="font-weight: 600; margin-bottom: 4px">BDInfo</div>
                <pre style="background: #f5f5f5; padding: 12px; border-radius: 4px; font-size: 12px; max-height: 200px; overflow: auto; white-space: pre-wrap">{{ form.bdinfo }}</pre>
              </div>

              <!-- §59.81: 截图（点击放大） -->
              <div v-if="form.screenshots.length > 0" style="margin-bottom: 16px">
                <div style="font-weight: 600; margin-bottom: 8px">截图</div>
                <div style="display: flex; flex-wrap: wrap; gap: 8px">
                  <img
                    v-for="(url, i) in form.screenshots" :key="i" :src="url"
                    style="width: 200px; border-radius: 4px; cursor: zoom-in"
                    @click="previewShotPreview = url"
                  />
                </div>
              </div>
              <a-modal :open="!!previewShotPreview" :footer="null" width="900px" @cancel="previewShotPreview = ''">
                <img v-if="previewShotPreview" :src="previewShotPreview" style="width: 100%" />
              </a-modal>

              <!-- §59.81: 发布简介（四段结构化 + 源码/渲染切换） -->
              <div v-if="previewRenderedDesc || previewStatement" style="margin-bottom: 16px">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
                  <div style="font-weight: 600">发布简介（按发布描述组装顺序）</div>
                  <a-radio-group v-model:value="previewDescMode" size="small">
                    <a-radio-button value="rendered">渲染效果</a-radio-button>
                    <a-radio-button value="source">BBCode 源码</a-radio-button>
                  </a-radio-group>
                </div>
                <template v-if="previewDescMode === 'rendered'">
                  <div v-if="previewStatement" style="padding: 12px; background: #fafafa; border-radius: 4px; margin-bottom: 8px">
                    <div style="color: #999; font-size: 12px; margin-bottom: 4px">— 声明 —</div>
                    <div style="line-height: 1.8" v-html="previewStatementHTML"></div>
                  </div>
                  <div v-if="form.poster" style="text-align: center; margin-bottom: 8px">
                    <img :src="form.poster" style="max-height: 260px; border-radius: 4px" />
                  </div>
                  <div v-if="previewRenderedDesc" style="padding: 12px; background: #fafafa; border-radius: 4px; line-height: 1.8" v-html="previewRenderedDesc"></div>
                </template>
                <pre v-else style="background: #f5f5f5; padding: 12px; border-radius: 4px; font-size: 12px; max-height: 500px; overflow: auto; white-space: pre-wrap">{{ previewDescSource }}</pre>
              </div>

              <!-- 校验状态 -->
              <a-alert
                :type="seedMissingFields.length === 0 ? 'success' : 'warning'"
                show-icon
                style="margin-top: 16px"
                :message="seedMissingFields.length === 0 ? '✓ 9 必需字段齐全，已自动审核' : `⚠ 仍缺 ${seedMissingFields.length} 个字段：${seedMissingFields.join(', ')}`"
              />
            </div>
          </template>

          <!-- 编辑模式 -->
          <template v-else>
          <a-alert
            v-if="forbiddenFlag" type="error" show-icon style="margin-bottom: 16px"
            :message="`⛔ 禁止转载：${forbiddenFlag}`" description="该种子被源站标记为禁止转载，无法继续发布。"
          />

          <a-tabs v-model:active-key="activeTab">
            <!-- Tab 1: 种子详情 -->
            <a-tab-pane key="detail" tab="种子详情">
              <!-- §59.20 maintenanceOnly 模式：18 TechProfile 字段只读展示 -->
              <template v-if="maintenanceOnly">
                <a-descriptions :column="3" bordered size="small" style="max-width: 900px">
                  <a-descriptions-item label="主标题" :span="3">{{ form.title || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="副标题" :span="3">{{ form.subtitle || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="中文名">{{ form.titleComponents.chinese_prefix || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="剧名">{{ form.titleComponents.main_title || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="季集">{{ form.titleComponents.season_episode || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="年份">{{ form.titleComponents.year || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="制作组">{{ form.titleComponents.release_group || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="类型">{{ categoryLabel(form.titleComponents.category) }}</a-descriptions-item>
                  <!-- §59.82: 分组重排——媒介→视频→音频 聚类; 站点媒介合成行 -->
                  <a-descriptions-item label="媒介(站点)">{{ siteMediumDisplay }}</a-descriptions-item>
                  <a-descriptions-item label="片源">{{ form.titleComponents.source_type || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="规格">{{ specDisplay }}</a-descriptions-item>
                  <a-descriptions-item label="分发方">
                    {{ form.titleComponents.source_platform || '—' }}
                    <a-tooltip v-if="PLATFORM_FULLNAMES[form.titleComponents.source_platform]" :title="PLATFORM_FULLNAMES[form.titleComponents.source_platform]">
                      <InfoCircleOutlined style="color: #999; margin-left: 4px" />
                    </a-tooltip>
                  </a-descriptions-item>
                  <a-descriptions-item label="分辨率">{{ form.titleComponents.resolution || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="视频编码">{{ form.titleComponents.video_codec || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="HDR">{{ form.titleComponents.hdr || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="bit">{{ form.titleComponents.bit_depth || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="音频编码">{{ form.titleComponents.audio_codec || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="声道">{{ form.titleComponents.audio_channels || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="音频技术">{{ form.titleComponents.audio_technology || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="音轨数">{{ form.titleComponents.audio_tracks || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="版本">{{ form.titleComponents.edition_info || '—' }}</a-descriptions-item>
                  <a-descriptions-item label="地区码">{{ form.titleComponents.region_code || '—' }}</a-descriptions-item>
                </a-descriptions>
                <!-- §59.75: 产地/类型（PTGen 源归一只读展示——发布映射消费 canonical） -->
                <a-form-item v-if="seedRegionGenre.region.length || seedRegionGenre.genre.length" label="产地 / 类型" style="max-width: 900px; margin-top: 16px">
                  <span v-if="seedRegionGenre.region.length" style="margin-right: 16px">
                    产地：<a-tag v-for="r in seedRegionGenre.region" :key="r" color="geekblue">{{ r }}</a-tag>
                  </span>
                  <span v-if="seedRegionGenre.genre.length">
                    类型：<a-tag v-for="g in seedRegionGenre.genre" :key="g" color="purple">{{ g }}</a-tag>
                  </span>
                </a-form-item>
                <!-- §59.26: 标签（可编辑，供发布使用） -->
                <a-form-item label="标签" style="max-width: 900px; margin-top: 16px">
                  <TagSelector v-model="form.tags" />
                </a-form-item>
                <div v-if="seedMissingFields.length > 0" style="margin-top: 12px; padding: 8px 12px; background: #fffbe6; border-radius: 4px; font-size: 13px">
                  <span style="color: #faad14">⚠ 缺失字段：</span>{{ seedMissingFields.join(', ') }}
                </div>
                <div v-else-if="seedReviewed" style="margin-top: 12px; padding: 8px 12px; background: #f6ffed; border-radius: 4px; font-size: 13px; color: #52c41a">
                  ✓ 已审核（9 必需字段齐全）
                </div>
              </template>
              <!-- 普通发布流程：可编辑表单 -->
              <template v-else>
                <a-form layout="vertical" style="max-width: 800px">
                  <a-row :gutter="16">
                    <a-col :span="12">
                      <a-form-item label="主标题（英文）">
                        <a-input v-model:value="form.title" placeholder="English.Title" />
                      </a-form-item>
                    </a-col>
                    <a-col :span="12">
                      <a-form-item label="中文副标题">
                        <a-input v-model:value="form.subtitle" placeholder="中文名" />
                        <div v-if="subtitleWarning" style="color: #faad14; font-size: 12px; margin-top: 4px">{{ subtitleWarning }}</div>
                      </a-form-item>
                    </a-col>
                  </a-row>
                  <div style="margin-bottom: 8px">
                    <a-button size="small" :loading="reparsing" @click="reparseTitle">
                      <template #icon><ReloadOutlined /></template>
                      重新解析标题
                    </a-button>
                    <span v-if="reparseResult" style="margin-left: 8px; font-size: 12px" :style="{ color: reparseResult.includes('失败') ? '#cf1322' : '#52c41a' }">{{ reparseResult }}</span>
                  </div>
                  <a-row :gutter="16">
                    <a-col :span="6">
                      <a-form-item label="分辨率">
                        <a-input v-model:value="form.titleComponents.resolution" placeholder="1080p" />
                      </a-form-item>
                    </a-col>
                    <a-col :span="6">
                      <a-form-item label="视频编码">
                        <a-input v-model:value="form.titleComponents.video_codec" placeholder="x265" />
                      </a-form-item>
                    </a-col>
                    <a-col :span="6">
                      <a-form-item label="音频编码">
                        <a-input v-model:value="form.titleComponents.audio_codec" placeholder="AC3" />
                      </a-form-item>
                    </a-col>
                    <a-col :span="6">
                      <a-form-item label="媒介">
                        <a-input v-model:value="form.titleComponents.medium" placeholder="BluRay" />
                      </a-form-item>
                    </a-col>
                  </a-row>
                  <a-row :gutter="16">
                    <a-col :span="6">
                      <a-form-item label="制作组">
                        <a-input v-model:value="form.titleComponents.release_group" placeholder="-GROUP" />
                      </a-form-item>
                    </a-col>
                    <a-col :span="6">
                      <a-form-item label="HDR">
                        <a-input v-model:value="form.titleComponents.hdr" placeholder="HDR" />
                      </a-form-item>
                    </a-col>
                    <a-col :span="6">
                      <a-form-item label="年份/季集">
                        <a-input v-model:value="form.titleComponents.year" placeholder="2024 / S01E01" />
                      </a-form-item>
                    </a-col>
                    <a-col :span="6">
                      <a-form-item label="版本信息">
                        <a-input v-model:value="form.titleComponents.edition_info" placeholder="Remux" />
                      </a-form-item>
                    </a-col>
                  </a-row>
                  <a-form-item label="标签">
                    <TagSelector v-model="form.tags" />
                  </a-form-item>
                </a-form>
              </template><!-- v-else (non-maintenanceOnly) -->
            </a-tab-pane>

            <!-- Tab 2: 海报与声明 -->
            <a-tab-pane key="poster" tab="海报与声明">
              <a-form layout="vertical" style="max-width: 800px">
                <a-form-item label="海报 URL">
                  <div style="display: flex; gap: 8px; align-items: flex-start">
                    <a-input v-model:value="form.poster" placeholder="https://..." style="flex: 1" />
                    <a-button :loading="refreshing === 'poster'" @click="doRefresh('poster')">重新获取</a-button>
                  </div>
                  <a-image v-if="form.poster" :src="form.poster" :width="120" style="margin-top: 8px" />
                </a-form-item>
                <a-form-item label="声明">
                  <a-textarea v-model:value="form.statement" :auto-size="{ minRows: 3, maxRows: 25 }" placeholder="源站官组声明（只读）" :disabled="maintenanceOnly" />
                </a-form-item>
                <a-form-item v-if="!maintenanceOnly" label="豆瓣 / IMDb / TMDb">
                  <a-row :gutter="8">
                    <a-col :span="8"><a-input v-model:value="form.doubanLink" placeholder="豆瓣链接" /></a-col>
                    <a-col :span="8"><a-input v-model:value="form.imdbLink" placeholder="IMDb 链接" /></a-col>
                    <a-col :span="8"><a-input v-model:value="form.tmdbLink" placeholder="TMDb 链接" /></a-col>
                  </a-row>
                </a-form-item>
              </a-form>
            </a-tab-pane>

            <!-- Tab 3: 视频截图 -->
            <a-tab-pane key="screenshots" tab="视频截图">
              <div style="margin-bottom: 12px; display: flex; gap: 8px">
                <a-button :loading="refreshing === 'screenshots'" @click="doRefresh('screenshots')">{{ seedIsLocal ? '重新获取截图' : '从源站重新获取截图' }}</a-button>
                <a-button :loading="refreshing === 'rehost_screenshots'" :disabled="form.screenshots.length === 0" @click="doRefresh('rehost_screenshots')">一键转存到图床</a-button>
              </div>
              <ScreenshotManager
                ref="shotManagerRef"
                v-model:screenshots="form.screenshots"
                :screenshot-in-desc="form.screenshotInDesc"
                @update:screenshot-in-desc="form.screenshotInDesc = $event"
              />
            </a-tab-pane>

            <!-- Tab 4: 简介详情 -->
            <a-tab-pane key="intro" tab="简介详情">
              <div style="margin-bottom: 8px; display: flex; gap: 8px">
                <!-- §59.45: 简介重获是数据源修复动作（与 Tab3/Tab5 同性质），维护模式放开 -->
                <a-button :loading="refreshing === 'intro'" @click="doRefresh('intro')">重新获取简介（PTGen）</a-button>
              </div>
              <a-textarea v-model:value="form.description" :rows="20" placeholder="BBCode 简介正文" style="font-family: monospace" />
              <!-- §59.20: maintenanceOnly 模式下外部链接只读展示 -->
              <div v-if="maintenanceOnly && (form.doubanLink || form.imdbLink || form.tmdbLink)" style="margin-top: 12px">
                <span style="color: #999; font-size: 12px; margin-right: 12px">外部链接：</span>
                <a v-if="form.doubanLink" :href="form.doubanLink" target="_blank" style="margin-right: 8px; font-size: 12px">豆瓣</a>
                <a v-if="form.imdbLink" :href="form.imdbLink" target="_blank" style="margin-right: 8px; font-size: 12px">IMDb</a>
                <a v-if="form.tmdbLink" :href="form.tmdbLink" target="_blank" style="font-size: 12px">TMDb</a>
              </div>
            </a-tab-pane>

            <!-- Tab 5: 媒体信息 -->
            <a-tab-pane key="mediainfo" tab="媒体信息">
              <div style="margin-bottom: 8px; display: flex; gap: 8px">
                <!-- §59.36: MI 重获是数据源修复动作（与 Tab3 截图同性质），维护模式放开 -->
                <a-button :loading="refreshing === 'mediainfo'" @click="doRefresh('mediainfo')">{{ seedIsLocal ? '重新获取 MediaInfo' : '从源站重新获取 MediaInfo' }}</a-button>
              </div>
              <!-- §59.36: 维护模式 MI 只读展示，重获走上方按钮（数据修复动作） -->
              <a-textarea v-model:value="form.mediaInfo" :rows="20" placeholder="MediaInfo 文本" style="font-family: monospace; font-size: 12px" :disabled="maintenanceOnly" />
              <a-form-item v-if="form.bdinfo" label="BDInfo" style="margin-top: 12px">
                <a-textarea v-model:value="form.bdinfo" :rows="10" style="font-family: monospace; font-size: 12px" :disabled="maintenanceOnly" />
              </a-form-item>
            </a-tab-pane>

            <!-- §59.20 Tab 6: 已过滤声明（只读预览） -->
            <a-tab-pane v-if="maintenanceOnly" key="filtered" tab="已过滤声明">
              <a-alert
                type="info" show-icon style="margin-bottom: 12px"
                message="以下声明将在发布时被自动过滤。可在「发布规则 → 声明过滤规则」中管理过滤模式。"
              />
              <div v-if="filteredDeclarations.length > 0">
                <div v-for="(item, idx) in filteredDeclarations" :key="idx" style="margin-bottom: 8px; padding: 8px; background: #fafafa; border-radius: 4px; border-left: 3px solid #ff4d4f">
                  <div style="font-size: 12px; color: #ff4d4f; margin-bottom: 4px">命中模式: {{ item.pattern }}</div>
                  <pre style="margin: 0; font-size: 12px; white-space: pre-wrap; color: #666">{{ item.text }}</pre>
                </div>
              </div>
              <a-empty v-else description="当前简介无匹配的过滤声明" />
            </a-tab-pane>
           </a-tabs>
          </template><!-- v-else (editing mode) -->
        </div>

        <!-- Step 1: 参数预览 -->
        <div v-else-if="currentStep === 1" class="csp-step-content">
          <PublishFieldPreview
            :target-site="previewTargetSite"
            :mode="previewMode"
            :fields="previewFields"
            :completeness="previewCompleteness"
            :loading="previewLoading"
            error=""
          />
        </div>

        <!-- Step 2: 选择站点 -->
        <div v-else-if="currentStep === 2" class="csp-step-content">
          <WizardStepSelectTargets
            v-model="selectedTargets"
            :site-list="siteList"
            :targets-loading="targetsLoading"
            :anonymous="form.anonymous"
            :title-components="form.titleComponents"
            :info-hash="selectedTorrent?.info_hash || ''"
            :mode="analyzeResult?.last_merge_mode || 'ptgen_first'"
            @update:anonymous="form.anonymous = $event"
          />
        </div>

        <!-- Step 3: 发布结果 -->
        <div v-else-if="currentStep === 3" class="csp-step-content">
          <WizardStepResult
            :submit-error="submitError"
            :submitted-candidate-id="submittedCandidateId"
            :candidate-status="candidateStatus"
            :selected-targets="selectedTargets"
            :result-records="resultRecords"
            @back="handleClose"
          />
        </div>
      </div>

      <!-- ═══ Footer ═══ -->
      <div class="csp-footer">
        <div class="csp-footer-left">
          <a-button v-if="currentStep > 0 && !maintenanceOnly" @click="prevStep">上一步</a-button>
        </div>
        <div class="csp-footer-right">
          <a-button @click="handleClose">取消</a-button>
          <template v-if="maintenanceOnly">
            <!-- §59.20 ⑨: 预览模式 → 返回编辑 + 确认完成 -->
            <template v-if="seedPreviewMode">
              <a-button @click="backToEdit">返回编辑</a-button>
              <a-button type="primary" @click="confirmDone">确认完成</a-button>
            </template>
            <!-- 编辑模式 → 预览按钮 -->
            <template v-else>
              <a-button type="primary" :loading="saving" :disabled="loading || !!loadError" @click="saveOnly">
                预览
              </a-button>
            </template>
          </template>
          <template v-else>
            <a-button v-if="currentStep === 0" type="primary" :disabled="!canProceed" :loading="saving" @click="nextStep">
              保存并预览
            </a-button>
            <a-button v-else-if="currentStep === 1" type="primary" @click="nextStep">下一步</a-button>
            <a-button v-else-if="currentStep === 2" type="primary" :loading="submitting" :disabled="selectedTargets.length === 0" @click="doSubmit">
              立即发布（{{ selectedTargets.length }} 站）
            </a-button>
          </template>
        </div>
      </div>
    </div>
  </a-drawer>
</template>

<script setup lang="ts">
import { ref, computed, watch, onUnmounted } from 'vue'
import { message } from 'ant-design-vue'
import { CheckCircleFilled, ReloadOutlined } from '@ant-design/icons-vue'
import { manualForwardApi, publishDataApi, publishApi, publishTorrentsApi, seedConfigApi } from '@/api/publish'
import type { SeedDetail } from '@/api/publish'
import { parseBBCode } from '@/utils/bbcode'
import { TAG_GROUPS } from '@/generated/dict'
import { CATEGORY_LABELS, PLATFORM_FULLNAMES } from '@/generated/dict'
import { InfoCircleOutlined } from '@ant-design/icons-vue'
import type { ManualForwardSubmitRequest, PreviewField, PreviewCompleteness, PublishResultRecord } from '@/api/types'
import TagSelector from './TagSelector.vue'
import ScreenshotManager from './ScreenshotManager.vue'

// §59.54: 截图管理器引用（转存前快照）
const shotManagerRef = ref<InstanceType<typeof ScreenshotManager> | null>(null)
import PublishFieldPreview from './PublishFieldPreview.vue'
import WizardStepSelectTargets from './WizardStepSelectTargets.vue'
import WizardStepResult from './WizardStepResult.vue'

interface PresetTorrent {
  info_hash: string
  name: string
  size: number
  save_path: string
  client_id: number
  state?: string
  source_site?: string
  source_site_id?: number
}

const props = defineProps<{
  open: boolean
  presetTorrent?: PresetTorrent | null
  maintenanceOnly?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', val: boolean): void
  (e: 'success'): void
}>()

// --- State ---
const currentStep = ref(0)
const activeTab = ref('detail')
const loading = ref(false)
const loadingText = ref('')
const loadingProgress = ref(0)
const loadError = ref('')
const saving = ref(false)
const submitting = ref(false)
const reparsing = ref(false)
const reparseResult = ref('')
const refreshing = ref('')
const bodyRef = ref<HTMLElement>()

const selectedTorrent = ref<PresetTorrent | null>(null)
const analyzeResult = ref<Record<string, any> | null>(null)
const cachedSites = ref<Array<{ id: number; siteName: string; torrentId: string; reviewed: boolean; fetchedAt: string; title: string; subtitle: string }>>([])
const currentSourceSite = ref<string>('')

const form = ref({
  title: '',
  subtitle: '',
  mediaInfo: '',
  description: '',
  screenshots: [] as string[],
  statement: '',
  poster: '',
  doubanLink: '',
  imdbLink: '',
  tmdbLink: '',
  tags: [] as string[],
  removedDeclarations: [] as string[],
  bdinfo: '',
  anonymous: false,
  screenshotInDesc: false,
  titleComponents: {} as Record<string, string>,
  // §59.75: 产地/类型（label 形态只读展示）
  region: [] as string[],
  genre: [] as string[],
})

// Preview
const previewTargetSite = ref('')
const previewMode = ref('ptgen_first')
const previewFields = ref<PreviewField[]>([])
const previewCompleteness = ref<PreviewCompleteness | null>(null)
const previewLoading = ref(false)

// Target sites
interface SiteItem { name: string; domain: string; blocked: boolean; blockReason: string }
const siteList = ref<SiteItem[]>([])
const selectedTargets = ref<string[]>([])
const targetsLoading = ref(false)

// Submit results
interface CandidateStatus { status: string; total_count: number; done_count: number; fail_count: number }
const submitError = ref('')
const submittedCandidateId = ref(0)
const candidateStatus = ref<CandidateStatus | null>(null)
const resultRecords = ref<Record<string, PublishResultRecord>>({})
let pollTimer: ReturnType<typeof setTimeout> | null = null
let candidatePollTimer: ReturnType<typeof setInterval> | null = null

// --- Computed ---
const forbiddenFlag = computed(() => analyzeResult.value?.forbidden || '')
const canProceed = computed(() => !!selectedTorrent.value && !forbiddenFlag.value)

const subtitleWarning = computed(() => {
  if (!form.value.subtitle) return ''
  const sub = form.value.subtitle
  const warnings: string[] = []
  if (/[\uFF00-\uFFEF]/.test(sub)) warnings.push('含全角符号')
  const firstChar = sub.charAt(0)
  if (!/[\u4e00-\u9fff]/.test(firstChar)) warnings.push('未以中文开头')
  if (/转自|转载自|来源[:：]/.test(sub)) warnings.push('包含转载来源')
  return warnings.join('； ')
})

// --- Lifecycle ---
watch(() => props.open, (val) => {
  if (val) {
    // §59.32: 恢复进行中任务（分析/发布）——不重置、不重新发起
    const active = restoreActiveTask()
    if (active) {
      resetPanel()
      if (props.presetTorrent) selectedTorrent.value = props.presetTorrent
      if (active.candidateId) {
        // 发布阶段恢复：直接进入结果步骤，续轮询
        currentStep.value = 2
        submittedCandidateId.value = active.candidateId
        startCandidatePoll(active.candidateId)
        return
      }
      if (active.taskId) {
        // 分析阶段恢复：loading 视图 + 续轮询
        loading.value = true
        loadingProgress.value = 0
        loadingText.value = '正在分析（已恢复）...'
        pollAnalyze(active.taskId)
        return
      }
    }
    resetPanel()
    if (props.presetTorrent) {
      selectedTorrent.value = props.presetTorrent
      if (props.maintenanceOnly) {
        fillFormFromPreset()
        loadDeclPatterns()
      } else {
        enterAnalyze()
      }
    }
  }
})

watch(currentStep, () => {
  if (bodyRef.value) bodyRef.value.scrollTop = 0
})

// §59.32: 后台任务运行态（分析 loading 或 发布轮询中）
const taskRunning = computed(() => loading.value || !!candidatePollTimer)

// §59.32: 任务恢复——sessionStorage 记录进行中任务（infoHash 匹配才恢复，防跨种子串台）
const SS_TASK_KEY = 'csp-active-task'

function saveActiveTask(infoHash: string, taskId?: number, candidateId?: number) {
  try {
    const prev = JSON.parse(sessionStorage.getItem(SS_TASK_KEY) || '{}')
    sessionStorage.setItem(SS_TASK_KEY, JSON.stringify({
      infoHash,
      taskId: taskId ?? prev.taskId,
      candidateId: candidateId ?? prev.candidateId,
    }))
  } catch { /* silent */ }
}

function clearActiveTask() {
  try { sessionStorage.removeItem(SS_TASK_KEY) } catch { /* silent */ }
}

function restoreActiveTask(): { infoHash: string; taskId?: number; candidateId?: number } | null {
  try {
    const raw = sessionStorage.getItem(SS_TASK_KEY)
    if (!raw) return null
    const t = JSON.parse(raw)
    if (!t?.infoHash) return null
    if (!t.taskId && !t.candidateId) return null
    // infoHash 匹配当前预设种子才恢复（跨种子不串台）
    if (props.presetTorrent?.info_hash && t.infoHash !== props.presetTorrent.info_hash) {
      sessionStorage.removeItem(SS_TASK_KEY)
      return null
    }
    return t
  } catch { return null }
}

function resetPanel() {
  stopCandidatePoll()
  currentStep.value = 0
  activeTab.value = 'detail'
  selectedTorrent.value = null
  analyzeResult.value = null
  cachedSites.value = []
  currentSourceSite.value = ''
  siteList.value = []
  selectedTargets.value = []
  submittedCandidateId.value = 0
  candidateStatus.value = null
  resultRecords.value = {}
  previewFields.value = []
  previewCompleteness.value = null
  previewLoading.value = false
  submitError.value = ''
  seedPreviewMode.value = false
  previewRenderedDesc.value = ''
  form.value = {
    title: '', subtitle: '', mediaInfo: '', description: '', screenshots: [],
    statement: '', poster: '', doubanLink: '', imdbLink: '', tmdbLink: '',
    tags: [], removedDeclarations: [], bdinfo: '', anonymous: false, screenshotInDesc: false,
    titleComponents: {},
    region: [],
    genre: [],
  }
}

function handleClose() {
  if (taskRunning.value) {
    message.info('任务将在后台继续，重新打开此种子可恢复进度')
  }
  emit('update:open', false)
}

function fillFormFromPreset() {
  const t = props.presetTorrent
  if (!t) return
  form.value = {
    title: t.name || '',
    subtitle: '',
    mediaInfo: '',
    description: '',
    screenshots: [],
    statement: '',
    poster: '',
    doubanLink: '', imdbLink: '', tmdbLink: '',
    tags: [],
    removedDeclarations: [],
    bdinfo: '',
    anonymous: false,
    screenshotInDesc: false,
    titleComponents: {},
    region: [],
    genre: [],
  }
  loading.value = false
  // §59.20: maintenanceOnly 模式从后端加载已存 metadata
  if (t.info_hash) {
    loadSeedDetail(t.info_hash)
  }
}

// §59.20: 从 GET /publish/seeds/:info_hash 加载种子配置
async function loadSeedDetail(infoHash: string) {
  loading.value = true
  loadError.value = ''
  try {
    const resp = await seedConfigApi.getSeed(infoHash, String(props.presetTorrent?.client_id || ''))
    const d: SeedDetail | undefined = resp.data?.data
    if (d) {
      form.value.title = d.title || form.value.title
      form.value.subtitle = d.subtitle || ''
      form.value.mediaInfo = d.mediainfo || ''
      form.value.description = d.description || ''
      form.value.screenshots = d.screenshots || []
      form.value.statement = d.statement || ''
      form.value.poster = d.poster || ''
      form.value.bdinfo = d.bdinfo || ''
      form.value.doubanLink = d.douban_url || ''
      form.value.imdbLink = d.imdb_url || ''
      form.value.tmdbLink = d.tmdb_url || ''
      // 18 TechProfile 字段 → titleComponents（Tab 1 只读展示）
      form.value.titleComponents = {
        main_title: d.main_title || '',
        season_episode: d.season_episode || '',
        year: d.year || '',
        release_group: d.release_group || '',
        chinese_prefix: d.chinese_prefix || '',
        resolution: d.resolution || '',
        video_codec: d.video_codec || '',
        audio_codec: d.audio_codec || '',
        audio_channels: d.audio_channels || '',
        audio_technology: d.audio_tech || '',
        hdr: d.hdr || '',
        bit_depth: d.bit_depth || '',
        source_type: d.source_type || '',
        specification: d.specification || '',
        source_platform: d.source_platform || '',
        edition_info: d.edition_info || '',
        region_code: d.region_code || '',
        category: d.category || '',
        form: d.form || '',
      }
      // 状态
      seedMissingFields.value = d.missing_fields || []
      // §59.75: 产地/类型（labels 只读展示）
      form.value.region = d.region?.labels || []
      form.value.genre = d.genre?.labels || []
      seedReviewed.value = d.reviewed || false
      seedEncode.value = d.encode ?? false
      seedIsLocal.value = (d as any).is_local ?? true
      currentSourceSite.value = d.site_name || ''
      // §59.26: 标签（获取时推断，编辑时可修正）
      form.value.tags = d.tags || []
    }
  } catch (e: unknown) {
    loadError.value = (e as Error).message
  } finally {
    loading.value = false
  }
}

// §59.20: 种子配置页状态
const seedMissingFields = ref<string[]>([])
// §59.75: 产地/类型展示（label 形态）
const seedRegionGenre = computed(() => ({
  region: form.value.region || [],
  genre: form.value.genre || [],
}))
const seedReviewed = ref(false)
// §59.82: 站点媒介合成显示——v1.05 source_type×specification 二维 → 站点单选视角
// （站点媒介下拉把两维压成一维枚举: remux/webdl/hdtv/uhd_bluray/bluray/encode）
const siteMediumDisplay = computed(() => {
  const tc = form.value.titleComponents
  const spec = (tc.specification || '').toLowerCase()
  const st = (tc.source_type || '').toLowerCase()
  if (spec === 'remux') return 'Remux'
  if (spec === 'web-dl' || spec === 'webdl') return 'WEB-DL'
  if (spec === 'webrip') return 'WEBRip'
  if (spec === 'hdtv') return 'HDTV'
  if (spec === 'uhdtv') return 'UHDTV'
  if (spec === 'bdrip') return 'Encode'
  if (st.includes('uhd')) return 'UHD Blu-ray 原盘'
  if (st.includes('blu')) return 'Blu-ray 原盘'
  if (st.includes('dvd')) return 'DVD'
  if (tc.encode) return 'Encode'
  return '—'
})

// §59.34: Encode 派生标识（后端真相源；v1.05 Encode 规格为空，规格栏显示 Encode）
const seedEncode = ref(false)
const specDisplay = computed(() =>
  form.value.titleComponents.specification || (seedEncode.value ? 'Encode' : '—')
)
const seedIsLocal = ref(true) // §59.21: 默认 true（向后兼容）
// §59.20 ⑨: 预览模式（保存即预览）
const seedPreviewMode = ref(false)
const previewRenderedDesc = ref('')
// §59.81: 发布预览增强——全字段/标签着色/分段渲染/源码切换
const previewFieldsData = ref<Record<string, unknown>>({})
const previewShotPreview = ref('')
const previewDescMode = ref<'rendered' | 'source'>('rendered')
const previewStatement = ref('')
const previewDescSource = ref('')

const pv = (key: string): string => {
  const v = previewFieldsData.value[key]
  if (v === undefined || v === null || v === '' || v === 0) return '—'
  return String(v)
}
const pvRegion = computed(() => {
  const r = previewFieldsData.value.region as { labels?: string[] } | undefined
  return r?.labels || []
})
const pvGenre = computed(() => {
  const g = previewFieldsData.value.genre as { labels?: string[] } | undefined
  return g?.labels || []
})
const previewTags = computed<string[]>(() => (previewFieldsData.value.tags as string[]) || [])
// §59.81: 禁转类标签红色（easy-upload getTagType 借鉴）
const isRestrictedTag = (t: string): boolean =>
  t === '禁转' || t === 'tag.禁转' || t === '限转' || t === 'tag.限转' || t === 'no_transfer'
// 标签显示名（dict label 优先）
const tagDisplayName = (t: string): string => {
  for (const g of TAG_GROUPS) {
    for (const tk of g.tags) {
      if (tk.key === t) return tk.label
    }
  }
  return t
}
const previewStatementHTML = computed(() => parseBBCode(previewStatement.value))

// §59.20: 已过滤声明 Tab 预览
const declPatterns = ref<string[]>([])
const filteredDeclarations = computed(() => {
  if (!form.value.description || declPatterns.value.length === 0) return []
  const results: Array<{ pattern: string; text: string }> = []
  const quoteRe = /\[quote\]([\s\S]*?)\[\/quote\]/g
  let match: RegExpExecArray | null
  while ((match = quoteRe.exec(form.value.description)) !== null) {
    const blockText = match[1].trim()
    for (const pattern of declPatterns.value) {
      if (blockText.toLowerCase().includes(pattern.toLowerCase())) {
        results.push({ pattern, text: blockText })
        break
      }
    }
  }
  return results
})

async function loadDeclPatterns() {
  try {
    const resp = await publishTorrentsApi.getDeclarationFilters()
    declPatterns.value = resp.data?.data?.patterns || []
  } catch { /* silent */ }
}

// --- Analyze ---
async function enterAnalyze() {
  if (!selectedTorrent.value) return
  loading.value = true
  loadingText.value = '正在分析种子信息...'
  loadingProgress.value = 0
  loadError.value = ''

  try {
    // 先查缓存站点
    const csResp = await publishDataApi.cachedSites(selectedTorrent.value.info_hash)
    cachedSites.value = csResp.data?.data?.sites || []
    if (cachedSites.value.length > 0 && !currentSourceSite.value) {
      currentSourceSite.value = cachedSites.value[0].siteName
    }

    const t = selectedTorrent.value
    const resp = await manualForwardApi.startAnalyze({
      clientId: t.client_id,
      infoHash: t.info_hash,
      name: t.name,
      savePath: t.save_path,
      size: t.size,
      sourceSite: t.source_site || currentSourceSite.value,
      sourceTorrentId: t.source_site_id ? String(t.source_site_id) : undefined,
    })
    const taskId = (resp.data?.data as Record<string, unknown>)?.task_id as number
    if (!taskId) {
      loadError.value = '分析任务创建失败'
      loading.value = false
      return
    }
    if (props.presetTorrent?.info_hash) {
      saveActiveTask(props.presetTorrent.info_hash, taskId)
    }
    pollAnalyze(taskId)
  } catch (e: unknown) {
    loadError.value = (e as Error).message
    loading.value = false
  }
}

function pollAnalyze(taskId: number) {
  async function poll() {
    try {
      const resp = await manualForwardApi.pollAnalyze(taskId)
      const task = resp.data?.data as Record<string, any> | undefined
      if (!task) return
      if (task.status === 'done' && task.result) {
        const r = task.result as Record<string, any>
        analyzeResult.value = r
        fillForm(r)
        loading.value = false
        loadingProgress.value = 0
        clearActiveTask()
      } else if (task.status === 'failed') {
        loadError.value = task.error || '分析失败'
        loading.value = false
        clearActiveTask()
      } else {
        loadingProgress.value = task.progress || 0
        loadingText.value = task.progressText || '正在分析...'
        pollTimer = setTimeout(poll, 2000)
      }
    } catch (e: unknown) {
      loadError.value = (e as Error).message
      loading.value = false
    }
  }
  pollTimer = setTimeout(poll, 1500)
}

function fillForm(r: Record<string, any>) {
  form.value.title = r.title || ''
  form.value.subtitle = r.subtitle || ''
  form.value.mediaInfo = r.media_info || ''
  form.value.description = r.description || ''
  form.value.screenshots = r.screenshots || []
  form.value.poster = r.poster_url || r.poster || ''
  form.value.statement = r.statement || ''
  form.value.doubanLink = r.douban_link || ''
  form.value.imdbLink = r.imdb_link || ''
  form.value.tmdbLink = r.tmdb_link || ''
  form.value.tags = r.tags || []
  form.value.bdinfo = r.bdinfo || ''
  form.value.titleComponents = r.title_components || {}
  if (r.removed_declarations) {
    form.value.removedDeclarations = r.removed_declarations
  }
}

// --- Source site switch ---
async function onSourceSiteChange() {
  if (!selectedTorrent.value || !currentSourceSite.value) return
  // Re-analyze with new source site
  selectedTorrent.value.source_site = currentSourceSite.value
  await enterAnalyze()
}

// --- Refresh ---
// §59.51: 后台截图任务——启动 + 2s 轮询 + 会话一致性校验
let capturePollTimer: ReturnType<typeof setInterval> | null = null

async function startScreenshotCaptureTask() {
  if (!selectedTorrent.value) return
  refreshing.value = 'screenshots'
  const taskName = selectedTorrent.value.name
  try {
    await manualForwardApi.startScreenshotCapture({
      name: taskName,
      savePath: selectedTorrent.value.save_path || '',
      clientId: String(selectedTorrent.value.client_id || ''),
      infoHash: selectedTorrent.value.info_hash,
      siteName: currentSourceSite.value || selectedTorrent.value.source_site || '',
    })
    message.info('截图中…（约 1-2 分钟）')
    capturePollTimer = setInterval(async () => {
      try {
        const resp = await manualForwardApi.screenshotCaptureProgress()
        const st = resp.data?.data
        if (!st || st.active) return
        if (capturePollTimer) { clearInterval(capturePollTimer); capturePollTimer = null }
        refreshing.value = ''
        if (st.status === 'done' && st.screenshots && st.screenshots.length > 0) {
          // §59.51 遗漏5: 会话一致性——用户可能已切换到另一个种子
          if (selectedTorrent.value && selectedTorrent.value.name === taskName) {
            form.value.screenshots = st.screenshots
            message.success(`截图完成（${st.screenshots.length} 张）`)
          } else {
            message.info(`「${taskName.slice(0, 30)}」截图已完成，重新打开编辑器可见`)
          }
        } else {
          message.error('截图失败: ' + (st.error || '未知错误'))
        }
      } catch {
        // 轮询单次失败忽略（网络抖动），下轮继续
      }
    }, 2000)
  } catch (e: unknown) {
    refreshing.value = ''
    message.error(`截图任务启动失败: ${(e as Error).message}`)
  }
}

async function doRefresh(type: string) {
  if (!selectedTorrent.value) return
  // §59.51: 本地截图走后台任务（长任务轮询），源站截图保持旧同步链
  if (type === 'screenshots' && seedIsLocal.value) {
    await startScreenshotCaptureTask()
    return
  }
  refreshing.value = type
  try {
    const payload: { type: string; name: string; savePath?: string; infoHash?: string; siteName?: string; screenshots?: string[]; clientId?: string } = {
      type,
      name: selectedTorrent.value.name,
      savePath: selectedTorrent.value.save_path,
      infoHash: selectedTorrent.value.info_hash,
      siteName: currentSourceSite.value || selectedTorrent.value.source_site || '',
      clientId: String(selectedTorrent.value.client_id || ''),
    }
    if (type === 'rehost_screenshots') {
      payload.screenshots = form.value.screenshots
      // §59.54: 转存前快照（恢复引用按钮的还原源）
      const sm = shotManagerRef.value as unknown as { snapshotBeforeRehost?: () => void } | null
      sm?.snapshotBeforeRehost?.()
    }
    const resp = await manualForwardApi.refresh(payload)
    const data = (resp.data?.data || {}) as Record<string, unknown>
    if (type === 'poster') {
      if (data.poster) form.value.poster = data.poster as string
      if (data.douban_link) form.value.doubanLink = data.douban_link as string
      if (data.imdb_link) form.value.imdbLink = data.imdb_link as string
      if (data.tmdb_link) form.value.tmdbLink = data.tmdb_link as string
    } else if (type === 'intro') {
      if (data.description) form.value.description = data.description as string
      if (data.subtitle) form.value.subtitle = data.subtitle as string
    } else if (type === 'mediainfo') {
      if (data.mediainfo) form.value.mediaInfo = data.mediainfo as string
    } else if (type === 'screenshots') {
      if (data.screenshots) form.value.screenshots = data.screenshots as string[]
    } else if (type === 'rehost_screenshots') {
      if (data.screenshots) form.value.screenshots = data.screenshots as string[]
    }
    message.success(`${type} 刷新成功`)
  } catch (e: unknown) {
    message.error(`刷新失败: ${(e as Error).message}`)
  } finally {
    refreshing.value = ''
  }
}

// --- Step navigation ---
async function nextStep() {
  if (currentStep.value === 0) {
    // Save to DB before proceeding
    await saveToDB()
    // Load preview
    await loadPreview()
    currentStep.value = 1
  } else if (currentStep.value === 1) {
    await enterSelectSites()
    currentStep.value = 2
  }
}

function prevStep() {
  if (currentStep.value > 0) currentStep.value--
}

async function saveToDB() {
  if (!selectedTorrent.value) return
  // 直接从 cachedSites 找到 metadata ID，避免脆弱的搜索
  const site = cachedSites.value.find(s => s.siteName === (currentSourceSite.value || selectedTorrent.value?.source_site))
  if (!site || !site.id) return
  saving.value = true
  try {
    await publishDataApi.saveSeedData(site.id, {
      title: form.value.title,
      subtitle: form.value.subtitle,
      description: form.value.description,
      screenshots: JSON.stringify(form.value.screenshots),
      poster: form.value.poster,
      mediainfo: form.value.mediaInfo,
      tags: JSON.stringify(form.value.tags),
    })
  } catch (e: unknown) {
    // §59.56 审计: 静默吞错导致列名笔误长期不可见——保存失败必须提示
    message.error('保存失败: ' + (e as Error).message)
  } finally {
    saving.value = false
  }
}

// §59.20: maintenanceOnly 模式保存——调 PUT /publish/seeds/:info_hash → 保存+预览
async function saveOnly() {
  if (!selectedTorrent.value?.info_hash) {
    message.error('缺少 info_hash')
    return
  }
  saving.value = true
  try {
    const resp = await seedConfigApi.putSeed(selectedTorrent.value.info_hash, {
      poster: form.value.poster,
      screenshots: form.value.screenshots,
      description: form.value.description,
      tags: form.value.tags,
      siteName: currentSourceSite.value || undefined,
    })
    const result = resp.data?.data
    if (result) {
      seedReviewed.value = result.reviewed || false
      seedMissingFields.value = result.missing_fields || []
      // §59.28 C（方案A ④）: 服务端渲染的完整描述（声明+致谢+海报+正文+截图）
      if (result.rendered_description) {
        previewRenderedDesc.value = parseBBCode(result.rendered_description)
        previewDescSource.value = result.rendered_description
      } else {
        previewRenderedDesc.value = parseBBCode(form.value.description)
        previewDescSource.value = form.value.description
      }
      // §59.81: 预览增强素材——全字段/标签/产地类型/声明段
      previewFieldsData.value = { ...result } as Record<string, unknown>
      previewStatement.value = (result as { statement?: string }).statement || form.value.statement
      // §59.28 C（方案A ②）: 标准化重组标题回填预览
      if (result.reassembled_title) {
        form.value.title = result.reassembled_title
      }
    }
    seedPreviewMode.value = true
  } catch (e: unknown) {
    message.error('保存失败: ' + (e as Error).message)
  } finally {
    saving.value = false
  }
}

// §59.20 ⑨: 确认完成——数据已在预览时存好，直接关闭
function confirmDone() {
  emit('success')
  emit('update:open', false)
}

// §59.20 ⑨: 返回编辑
function backToEdit() {
  seedPreviewMode.value = false
}

async function loadPreview() {
  if (!selectedTorrent.value) return
  previewLoading.value = true
  try {
    const resp = await manualForwardApi.previewFields({
      infoHash: selectedTorrent.value.info_hash,
      targetSite: '',
      mode: analyzeResult.value?.last_merge_mode || 'ptgen_first',
    })
    const data = resp.data?.data as unknown as Record<string, unknown> | undefined
    if (data) {
      previewFields.value = (data.fields as PreviewField[]) || []
      previewCompleteness.value = (data.completeness as PreviewCompleteness) || null
      previewMode.value = (data.mode as string) || 'ptgen_first'
    }
  } catch { /* silent */ } finally {
    previewLoading.value = false
  }
}

// 覆盖缓存（遗漏 C：目标站三色排除）
const coveredSites = ref<Set<string>>(new Set())

async function enterSelectSites() {
  selectedTargets.value = []
  targetsLoading.value = true
  try {
    const blockedTargets = (analyzeResult.value?.blocked_targets as string[]) || []
    const resp = await manualForwardApi.eligibleTargets({
      sourceSite: analyzeResult.value?.source_site || '',
      blockedTargets: blockedTargets,
    })
    const raw = (resp.data?.data || []) as unknown[]

    // 查覆盖缓存（轻量级 DB 读，不触发慢查询）
    const coveredNames = new Set<string>()
    if (selectedTorrent.value?.info_hash) {
      try {
        const covResp = await publishDataApi.coverageCache(selectedTorrent.value.info_hash)
        const covSites = covResp.data?.data?.sites || []
        for (const s of covSites) coveredNames.add(s.siteName)
      } catch { /* silent — 无覆盖数据则不过滤 */ }
    }
    coveredSites.value = coveredNames

    // 过滤掉已覆盖站点（🟢做种中 + 🟡可辅种），保留 ⚪未发现 + 无覆盖数据
    siteList.value = raw
      .filter((item) => {
        const obj = item as Record<string, unknown>
        return !coveredNames.has(obj.name as string)
      })
      .map((item) => {
        const obj = item as Record<string, unknown>
        const blocked = !!obj.blocked
        const name = obj.name as string
        let blockReason = ''
        if (blocked) {
          blockReason = blockedTargets.includes(name) ? '互斥规则' : '缺少 cookie/passkey'
        }
        return { name, domain: (obj.domain as string) || '', blocked, blockReason }
      })

    if (coveredNames.size > 0) {
      message.info(`已排除 ${coveredNames.size} 个已覆盖站点`)
    }
  } catch (e: unknown) {
    message.error((e as Error).message)
  } finally {
    targetsLoading.value = false
  }
}

// --- helpers ---
// §59.35 P3: 分级 label——Layer 1 字典优先（generated/dict.ts，与后端同源），
// 扩展分类（adapter 源站直传的 category.mv/game/software 等）本地兜底
const extendedCategoryLabels: Record<string, string> = {
  'category.mv': 'MV',
  'category.audiobook': '有声读物',
  'category.ebook': '电子书',
  'category.game': '游戏',
  'category.software': '软件',
}
function categoryLabelOf(v?: string): string {
  if (!v) return '—'
  return CATEGORY_LABELS[v] || extendedCategoryLabels[v] || v
}
function categoryLabel(v?: string): string {
  return categoryLabelOf(v)
}

// --- Submit ---
async function reparseTitle() {
  const title = form.value.title || selectedTorrent.value?.name || ''
  if (!title) {
    message.warning('请先输入标题')
    return
  }
  reparsing.value = true
  reparseResult.value = ''
  try {
    const resp = await manualForwardApi.parseTitle(title)
    const data = resp.data?.data
    if (data?.title_components) {
      const tc = data.title_components
      // 只更新有值的字段（不覆盖用户手动修改的空字段）
      for (const [key, val] of Object.entries(tc)) {
        if (val) form.value.titleComponents[key] = val
      }
      reparseResult.value = `解析成功：${data.category || ''} ${tc.resolution || ''} ${tc.medium || ''} ${tc.video_codec || ''}`.trim()
    } else {
      reparseResult.value = '解析完成（无额外信息）'
    }
  } catch (e: unknown) {
    reparseResult.value = `解析失败: ${(e as Error).message}`
  } finally {
    reparsing.value = false
  }
}

async function doSubmit() {
  if (!selectedTorrent.value || selectedTargets.value.length === 0) return
  submitting.value = true
  try {
    const t = selectedTorrent.value
    const req: ManualForwardSubmitRequest = {
      clientId: t.client_id,
      infoHash: t.info_hash,
      title: t.name,
      sourceSite: analyzeResult.value?.source_site || t.source_site || '',
      sourceSiteId: t.source_site_id || 0,
      description: form.value.description,
      mediaInfo: form.value.mediaInfo,
      screenshots: form.value.screenshots,
      targetSites: selectedTargets.value,
      subtitle: form.value.subtitle,
      statement: form.value.statement,
      poster: form.value.poster,
      doubanLink: form.value.doubanLink,
      imdbLink: form.value.imdbLink,
      tmdbLink: form.value.tmdbLink,
      tags: form.value.tags,
      bdinfo: form.value.bdinfo,
      anonymous: form.value.anonymous,
      screenshotInDesc: form.value.screenshotInDesc,
      titleComponents: form.value.titleComponents,
    }
    const resp = await manualForwardApi.submit(req)
    const candId = (resp.data?.data as unknown as Record<string, unknown>)?.candidate_id as number
    if (candId) {
      submittedCandidateId.value = candId
      if (props.presetTorrent?.info_hash) {
        saveActiveTask(props.presetTorrent.info_hash, undefined, candId)
      }
      currentStep.value = 3
      startCandidatePoll(candId)
    }
  } catch (e: unknown) {
    message.error(`发布失败: ${(e as Error).message}`)
  } finally {
    submitting.value = false
  }
}

function startCandidatePoll(candidateId: number) {
  candidatePollTimer = setInterval(async () => {
    try {
      const resp = await publishApi.getCandidate(candidateId)
      const c = resp.data?.data
      if (!c) return
      const status = c.publish_status as string

      // 每次轮询都拉 result records（增量进度）
      await fetchResultRecords()

      // 从 resultRecords 计算逐站进度
      const records = Object.values(resultRecords.value)
      const doneCount = records.filter(r => ['completed', 'skipped', 'exists', 'edited'].includes(r.status)).length
      const failCount = records.filter(r => r.status === 'failed').length

      candidateStatus.value = {
        status,
        total_count: selectedTargets.value.length,
        done_count: doneCount,
        fail_count: failCount,
      }

      if (status === 'done' || status === 'failed') {
        stopCandidatePoll()
        clearActiveTask()
      }
    } catch { /* silent */ }
  }, 3000)
}

async function fetchResultRecords() {
  if (!submittedCandidateId.value) return
  try {
    const resp = await publishApi.listResults({ page: 1, pageSize: 100, candidateId: submittedCandidateId.value })
    const items = (resp.data?.data?.items || []) as PublishResultRecord[]
    const map: Record<string, PublishResultRecord> = {}
    for (const r of items) map[r.target_site] = r
    resultRecords.value = map
  } catch { /* silent */ }
}

function stopCandidatePoll() {
  if (candidatePollTimer) {
    clearInterval(candidatePollTimer)
    candidatePollTimer = null
  }
}
</script>

<style scoped>
.csp-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
}
.csp-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 24px;
  border-bottom: 1px solid #f0f0f0;
}
.csp-header-left {
  display: flex;
  align-items: center;
  gap: 8px;
}
.csp-header-title {
  font-size: 16px;
  font-weight: 600;
}
.csp-steps {
  border-bottom: 1px solid #f0f0f0;
}
.csp-body {
  flex: 1;
  overflow-y: auto;
  padding: 16px 24px;
}
.csp-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 0;
}
.csp-step-content {
  max-width: 1200px;
}
.csp-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 24px;
  border-top: 1px solid #f0f0f0;
  background: #fff;
}
.csp-footer-right {
  display: flex;
  gap: 8px;
}
</style>

// §59.51: 组件卸载清轮询
onUnmounted(() => {
  if (capturePollTimer) { clearInterval(capturePollTimer); capturePollTimer = null }
})
