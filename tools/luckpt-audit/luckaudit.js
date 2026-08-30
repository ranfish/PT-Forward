/**
 * LuckAudit控制面板
 * 提供种子预审核功能和实用工具
 */
class LuckAuditPanel {
    constructor() {
        this.panel = null;
        this.isDragging = false;
        this.dragOffset = { x: 0, y: 0 };
        this.isVisible = false;
        this.isMinimized = false; // 添加收起状态记录
        this.torrentData = {};

        this.init();
    }

    init() {
        this.createPanel();
        this.bindEvents();
        this.loadPosition();
    }

    createPanel() {
        // 创建面板HTML结构
        const panelHTML = `
            <div id="luck-audit-panel" class="luck-audit-panel">
                <div class="luck-audit-header" id="luck-audit-header">
                    <div class="luck-audit-title">LuckAudit控制面板</div>
                    <div class="luck-audit-controls">
                        <button class="luck-audit-btn" id="luck-audit-minimize" title="最小化">−</button>
                        <button class="luck-audit-btn" id="luck-audit-close" title="关闭">×</button>
                    </div>
                </div>
                <div class="luck-audit-content" id="luck-audit-content">
                    <div class="luck-audit-status info" id="luck-audit-status">
                        准备就绪，等待收集种子信息...
                    </div>
                    <div class="luck-audit-section">
                        <div class="luck-audit-section-title">基本信息</div>
                        <div id="basic-info-content">
                            <div class="luck-audit-field">
                                <span class="luck-audit-field-label">标题:</span>
                                <span class="luck-audit-field-value" id="field-name">未填写</span>
                            </div>
                            <div class="luck-audit-field">
                                <span class="luck-audit-field-label">副标题:</span>
                                <span class="luck-audit-field-value" id="field-small-descr">未填写</span>
                            </div>
                            <div class="luck-audit-field">
                                <span class="luck-audit-field-label">IMDb链接:</span>
                                <span class="luck-audit-field-value" id="field-imdb">未填写</span>
                            </div>
                        </div>
                    </div>
                    <div class="luck-audit-section">
                        <div class="luck-audit-section-title">分类信息</div>
                        <div id="category-info-content">
                            <div class="luck-audit-field">
                                <span class="luck-audit-field-label">类型:</span>
                                <span class="luck-audit-field-value" id="field-type">未选择</span>
                            </div>
                            <div class="luck-audit-field">
                                <span class="luck-audit-field-label">质量:</span>
                                <span class="luck-audit-field-value" id="field-quality">未选择</span>
                            </div>
                            <div class="luck-audit-field">
                                <span class="luck-audit-field-label">标签:</span>
                                <span class="luck-audit-field-value" id="field-tags">未选择</span>
                            </div>
                        </div>
                    </div>
                    <div class="luck-audit-section">
                        <div class="luck-audit-section-title">技术信息</div>
                        <div id="technical-info-content">
                            <div class="luck-audit-field">
                                <span class="luck-audit-field-label">MediaInfo:</span>
                                <span class="luck-audit-field-value" id="field-mediainfo">未填写</span>
                            </div>
                            <div class="luck-audit-field">
                                <span class="luck-audit-field-label">简介长度:</span>
                                <span class="luck-audit-field-value" id="field-descr-length">0字符</span>
                            </div>
                        </div>
                    </div>
                </div>
                <div class="luck-audit-actions" id="luck-audit-actions">
                    <button class="luck-audit-action-btn primary" id="luck-audit-submit">预审核</button>
                    <button class="luck-audit-action-btn success" id="luck-audit-preview">预览</button>
                    <button class="luck-audit-action-btn secondary" id="luck-audit-export">导出JSON</button>
                    <button class="luck-audit-action-btn secondary" id="luck-audit-preview-json">预览JSON</button>
                </div>
            </div>
        `;

        // 添加到页面
        document.body.insertAdjacentHTML('beforeend', panelHTML);
        this.panel = document.getElementById('luck-audit-panel');
    }

    bindEvents() {
        // 拖拽功能
        const header = document.getElementById('luck-audit-header');
        header.addEventListener('mousedown', this.startDrag.bind(this));
        document.addEventListener('mousemove', this.drag.bind(this));
        document.addEventListener('mouseup', this.endDrag.bind(this));

        // 控制按钮
        document.getElementById('luck-audit-minimize').addEventListener('click', this.minimize.bind(this));
        document.getElementById('luck-audit-close').addEventListener('click', this.hide.bind(this));

        // 功能按钮
        document.getElementById('luck-audit-submit').addEventListener('click', this.submitForAudit.bind(this));
        document.getElementById('luck-audit-preview').addEventListener('click', this.previewInfo.bind(this));
        document.getElementById('luck-audit-export').addEventListener('click', this.exportJSON.bind(this));
        document.getElementById('luck-audit-preview-json').addEventListener('click', this.previewJSON.bind(this));

        // 监听表单变化
        this.watchFormChanges();
    }

    startDrag(e) {
        this.isDragging = true;
        this.panel.classList.add('dragging');

        const rect = this.panel.getBoundingClientRect();
        this.dragOffset.x = e.clientX - rect.left;
        this.dragOffset.y = e.clientY - rect.top;

        e.preventDefault();
    }

    drag(e) {
        if (!this.isDragging) return;

        const x = e.clientX - this.dragOffset.x;
        const y = e.clientY - this.dragOffset.y;

        // 限制在视窗内
        const maxX = window.innerWidth - this.panel.offsetWidth;
        const maxY = window.innerHeight - this.panel.offsetHeight;

        // 立即更新位置，无延迟
        requestAnimationFrame(() => {
            this.panel.style.left = Math.max(0, Math.min(x, maxX)) + 'px';
            this.panel.style.top = Math.max(0, Math.min(y, maxY)) + 'px';
            this.panel.style.right = 'auto';
        });
    }

    endDrag() {
        if (this.isDragging) {
            this.isDragging = false;
            this.panel.classList.remove('dragging');
            this.savePosition();
        }
    }

    minimize() {
        const content = document.getElementById('luck-audit-content');
        const minimizeBtn = document.getElementById('luck-audit-minimize');

        if (content.classList.contains('collapsed')) {
            // 展开 - 只展开信息显示部分
            content.classList.remove('collapsed');
            this.panel.classList.remove('minimized');
            minimizeBtn.textContent = '−';
            minimizeBtn.title = '收起';
            this.isMinimized = false; // 更新状态记录
        } else {
            // 收起 - 只收起信息显示部分，保留按钮
            content.classList.add('collapsed');
            this.panel.classList.add('minimized');
            minimizeBtn.textContent = '+';
            minimizeBtn.title = '展开';
            this.isMinimized = true; // 更新状态记录
        }
    }

    show() {
        // 先设置display以触发动画
        this.panel.style.display = 'block';

        // 使用requestAnimationFrame确保动画流畅
        requestAnimationFrame(() => {
            this.panel.classList.add('show');
            this.isVisible = true;

            // 根据记录的状态决定是否保持收起状态
            const content = document.getElementById('luck-audit-content');
            const minimizeBtn = document.getElementById('luck-audit-minimize');

            if (this.isMinimized) {
                // 保持收起状态
                this.panel.classList.add('minimized');
                content.classList.add('collapsed');
                minimizeBtn.textContent = '+';
                minimizeBtn.title = '展开';
            } else {
                // 展开状态
                this.panel.classList.remove('minimized');
                content.classList.remove('collapsed');
                minimizeBtn.textContent = '−';
                minimizeBtn.title = '收起';
            }

            // 立即收集一次信息
            setTimeout(() => {
                this.collectTorrentInfo();
            }, 100);
        });
    }

    hide() {
        this.panel.classList.remove('show');
        this.isVisible = false;

        // 等待动画完成后再隐藏
        setTimeout(() => {
            if (!this.isVisible) { // 再次检查，避免快速切换时的问题
                this.panel.style.display = 'none';
            }
        }, 400); // 与CSS transition时间一致

        // 确保没有遗留的模态窗口
        const modals = document.querySelectorAll('.luck-audit-modal');
        modals.forEach(modal => {
            if (modal.parentNode) {
                modal.remove();
            }
        });
    }

    toggle() {
        if (this.isVisible) {
            this.hide();
        } else {
            this.show();
        }
    }

    savePosition() {
        const rect = this.panel.getBoundingClientRect();
        localStorage.setItem('luckaudit-position', JSON.stringify({
            left: rect.left,
            top: rect.top
        }));
    }

    loadPosition() {
        const saved = localStorage.getItem('luckaudit-position');
        if (saved) {
            const pos = JSON.parse(saved);
            this.panel.style.left = pos.left + 'px';
            this.panel.style.top = pos.top + 'px';
            this.panel.style.right = 'auto';
        }
    }

    updateStatus(message, type = 'info') {
        const status = document.getElementById('luck-audit-status');
        status.textContent = message;
        status.className = `luck-audit-status ${type}`;
    }

    watchFormChanges() {
        // 防抖函数，避免频繁更新
        let debounceTimer = null;
        const debouncedCollect = () => {
            if (debounceTimer) {
                clearTimeout(debounceTimer);
            }
            debounceTimer = setTimeout(() => {
                this.collectTorrentInfo();
            }, 300); // 300ms防抖延迟
        };

        // 监听表单字段变化
        const fields = ['name', 'small_descr', 'url', 'descr', 'technical_info'];

        fields.forEach(fieldName => {
            const field = document.querySelector(`[name="${fieldName}"]`);
            if (field) {
                field.addEventListener('input', debouncedCollect);
            }
        });

        // 监听表单内的选择框变化
        const formContainer = document.querySelector('form') || document.body;
        const selects = formContainer.querySelectorAll('select');
        selects.forEach(select => {
            select.addEventListener('change', debouncedCollect);
        });

        // 监听表单内的复选框变化
        const checkboxes = formContainer.querySelectorAll('input[type="checkbox"]');
        checkboxes.forEach(checkbox => {
            checkbox.addEventListener('change', debouncedCollect);
        });

        // 存储observer引用以便后续清理
        this.formObserver = new MutationObserver((mutations) => {
            // 只监听特定类型的变化，减少不必要的触发
            const shouldUpdate = mutations.some(mutation => {
                // 只关注属性变化或新增/删除节点
                if (mutation.type === 'attributes') {
                    const attrName = mutation.attributeName;
                    return attrName === 'value' || attrName === 'checked' || attrName === 'selected';
                }
                return mutation.type === 'childList';
            });

            if (shouldUpdate) {
                debouncedCollect();
            }
        });

        // 只监听表单容器，减小监听范围
        this.formObserver.observe(formContainer, {
            childList: false, // 不监听子节点添加/删除
            subtree: false,   // 不监听子树
            attributes: true,
            attributeFilter: ['value', 'checked', 'selected']
        });
    }

    // 清理资源的方法
    cleanup() {
        if (this.formObserver) {
            this.formObserver.disconnect();
            this.formObserver = null;
        }
    }

    collectTorrentInfo() {
        // 只有在面板可见时才收集信息，提高性能
        if (!this.isVisible) {
            return;
        }

        this.updateStatus('正在收集种子信息...', 'info');

        try {
            // 收集基本信息
            const name = this.getFieldValue('name') || '';
            const smallDescr = this.getFieldValue('small_descr') || '';
            const imdbUrl = this.getFieldValue('url') || '';
            const description = this.getFieldValue('descr') || '';
            const technicalInfo = this.getFieldValue('technical_info') || '';

            // 收集类型信息
            const typeInfo = this.getTypeInfo();

            // 收集质量信息
            const qualityInfo = this.getQualityInfo();

            // 收集标签信息
            const tags = this.getTagsInfo();

            // 更新torrentData
            this.torrentData = {
                name: name.trim(),
                small_descr: smallDescr.trim(),
                imdb_url: imdbUrl.trim(),
                description: description.trim(),
                technical_info: technicalInfo.trim(),
                type: typeInfo,
                quality: qualityInfo,
                tags: tags,
                collected_at: new Date().toISOString()
            };

            // 更新显示
            this.updateDisplay();
            this.updateStatus('信息收集完成', 'success');

        } catch (error) {
            console.error('收集种子信息时出错:', error);
            this.updateStatus('收集信息时出错: ' + error.message, 'error');
        }
    }

    getFieldValue(fieldName) {
        // 尝试多种方式查找字段
        let field = document.querySelector(`[name="${fieldName}"]`);

        // 如果是简介字段，可能在iframe中
        if (!field && fieldName === 'descr') {
            // 查找BBCode编辑器
            const iframe = document.querySelector('iframe[name="descr"]');
            if (iframe && iframe.contentDocument) {
                const textArea = iframe.contentDocument.querySelector('textarea[name="descr"]');
                if (textArea) {
                    field = textArea;
                }
            }
            // 或者查找可见的textarea
            if (!field) {
                field = document.querySelector('textarea[name="descr"]');
            }
        }

        if (!field) return '';

        if (field.tagName === 'TEXTAREA') {
            return field.value;
        } else if (field.tagName === 'INPUT') {
            return field.value;
        }
        return '';
    }

    getTypeInfo() {
        const typeSelects = document.querySelectorAll('select[name="type"]');
        let selectedType = null;

        for (const select of typeSelects) {
            if (select.value && select.value !== '0') {
                const selectedOption = select.options[select.selectedIndex];
                selectedType = {
                    id: select.value,
                    name: selectedOption.text,
                    mode: select.getAttribute('data-mode') || ''
                };
                break;
            }
        }

        return selectedType;
    }

    getQualityInfo() {
        const quality = {};

        // 收集各种质量相关的选择框 - 使用正确的字段名
        const qualityFields = [
            'source_sel', 'medium_sel', 'codec_sel', 'standard_sel',
            'processing_sel', 'audiocodec_sel', 'team_sel'
        ];

        qualityFields.forEach(fieldName => {
            // 尝试多种方式查找字段
            let selects = document.querySelectorAll(`select[name="${fieldName}"]`);

            // 如果没找到，尝试查找带数组索引的字段名
            if (selects.length === 0) {
                selects = document.querySelectorAll(`select[name^="${fieldName}["]`);
            }

            selects.forEach(select => {
                if (select && select.value && select.value !== '0' && !select.disabled) {
                    const selectedOption = select.options[select.selectedIndex];
                    const cleanFieldName = fieldName.replace('_sel', '');
                    quality[cleanFieldName] = {
                        id: select.value,
                        name: selectedOption.text.trim()
                    };
                }
            });
        });

        return quality;
    }

    getTagsInfo() {
        const tags = [];
        const tagCheckboxes = document.querySelectorAll('input[type="checkbox"][name^="tags"]');

        tagCheckboxes.forEach(checkbox => {
            if (checkbox.checked) {
                const label = checkbox.parentElement;
                tags.push({
                    id: checkbox.value,
                    name: label.textContent.trim()
                });
            }
        });

        return tags;
    }

    updateDisplay() {
        // 更新基本信息显示
        document.getElementById('field-name').textContent = this.torrentData.name || '未填写';
        document.getElementById('field-small-descr').textContent = this.torrentData.small_descr || '未填写';
        document.getElementById('field-imdb').textContent = this.torrentData.imdb_url || '未填写';

        // 更新分类信息显示
        document.getElementById('field-type').textContent =
            this.torrentData.type ? this.torrentData.type.name : '未选择';

        // 更新质量信息显示
        const qualityText = Object.values(this.torrentData.quality || {})
            .map(q => q.name)
            .join(', ') || '未选择';
        document.getElementById('field-quality').textContent = qualityText;

        // 更新标签信息显示
        const tagsText = this.torrentData.tags
            .map(tag => tag.name)
            .join(', ') || '未选择';
        document.getElementById('field-tags').textContent = tagsText;

        // 更新技术信息显示
        const mediaInfoLength = this.torrentData.technical_info.length;
        document.getElementById('field-mediainfo').textContent =
            mediaInfoLength > 0 ? `已填写 (${mediaInfoLength}字符)` : '未填写';

        // 更新简介长度显示
        const descrLength = this.torrentData.description.length;
        document.getElementById('field-descr-length').textContent = `${descrLength}字符`;
    }

    async submitForAudit() {
        // Always submit the current form state, including fields filled by PTGen.
        this.collectTorrentInfo();

        if (!this.torrentData || !this.torrentData.name) {
            this.updateStatus('请先收集种子信息', 'warning');
            return;
        }

        // 检查是否已存在预审核结果窗口
        const existingModal = document.getElementById('luck-audit-result-modal');

        // 只有在预审核结果窗口不存在时才收起控制面板
        if (!existingModal) {
            this.minimize();
        }

        // 显示预审核弹出层
        this.showAuditModal();

        try {
            // 准备发送的数据 - 使用完整的种子数据格式
            const auditData = {
                ...this.torrentData,
                user_info: window.LUCK_AUDIT_USER || null,
                export_time: new Date().toISOString(),
                page_url: this.getAuditPageUrl()
            };

            // 发送到预审核接口
            const response = await fetch('/api/auto-audit/pre-audit', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                    'Accept': 'application/json',
                    'X-Requested-With': 'XMLHttpRequest',
                },
                credentials: 'same-origin',
                body: JSON.stringify(auditData)
            });

            const contentType = (response.headers.get('content-type') || '').toLowerCase();
            const responseBody = await response.text();
            if (!contentType.includes('json')) {
                if (response.redirected && response.url.includes('/login.php')) {
                    throw new Error('登录状态已失效，请刷新页面后重试');
                }
                throw new Error(`预审核接口返回了非 JSON 响应（HTTP ${response.status}）`);
            }

            let result;
            try {
                result = responseBody ? JSON.parse(responseBody) : {};
            } catch (parseError) {
                throw new Error('预审核接口返回了无效 JSON');
            }

            if (!response.ok || result.success === false) {
                let validationMessage = '';
                if (result.errors && typeof result.errors === 'object') {
                    const firstError = Object.values(result.errors).find(messages => {
                        return Array.isArray(messages) ? messages.length > 0 : Boolean(messages);
                    });
                    validationMessage = Array.isArray(firstError) ? firstError[0] : (firstError || '');
                }
                throw new Error(validationMessage || result.message || `HTTP ${response.status}: ${response.statusText}`);
            }

            console.log('预审核结果:', result);

            // 更新预审核弹出层显示结果
            this.updateAuditModal(result);

        } catch (error) {
            console.error('预审核提交失败:', error);
            this.updateAuditModalError('预审核提交失败: ' + error.message);
        }
    }

    getAuditPageUrl() {
        const fullUrl = window.location.href || '';
        if (fullUrl.length <= 2048) {
            return fullUrl;
        }

        try {
            const currentUrl = new URL(fullUrl);
            const compactUrl = `${currentUrl.origin}${currentUrl.pathname}`;
            return compactUrl.length <= 2048 ? compactUrl : compactUrl.slice(0, 2048);
        } catch (error) {
            return fullUrl.slice(0, 2048);
        }
    }

    previewInfo() {
        if (!this.torrentData || !this.torrentData.name) {
            this.updateStatus('请先收集种子信息', 'warning');
            return;
        }

        // 创建预览窗口
        const previewData = {
            基本信息: {
                标题: this.torrentData.name,
                副标题: this.torrentData.small_descr,
                'IMDb链接': this.torrentData.imdb_url,
                '简介长度': this.torrentData.description.length + '字符',
                'MediaInfo长度': this.torrentData.technical_info.length + '字符'
            },
            分类信息: {
                类型: this.torrentData.type ? this.torrentData.type.name : '未选择',
                质量: Object.values(this.torrentData.quality || {}).map(q => q.name).join(', ') || '未选择',
                标签: this.torrentData.tags.map(tag => tag.name).join(', ') || '未选择'
            }
        };

        // 生成预览HTML
        let previewHTML = '<div style="font-family: -apple-system, BlinkMacSystemFont, \'Segoe UI\', Roboto, sans-serif; font-size: 13px; line-height: 1.6;">';

        for (const [section, fields] of Object.entries(previewData)) {
            previewHTML += `
                <div style="margin-bottom: 20px;">
                    <h3 style="
                        color: #7a9e7a;
                        margin: 0 0 12px 0;
                        font-size: 15px;
                        font-weight: 600;
                        padding-bottom: 8px;
                        border-bottom: 2px solid #e8e1d2;
                        text-shadow: 0 1px 2px rgba(255, 255, 255, 0.8);
                    ">${section}</h3>
                    <div style="
                        background: linear-gradient(135deg, #ffffff 0%, #fafafa 100%);
                        border: 1px solid #e8e1d2;
                        border-radius: 8px;
                        padding: 12px;
                        box-shadow: inset 0 1px 3px rgba(0, 0, 0, 0.05);
                    ">
            `;

            for (const [key, value] of Object.entries(fields)) {
                previewHTML += `
                    <div style="
                        display: flex;
                        margin-bottom: 8px;
                        padding: 6px 0;
                        border-bottom: 1px solid rgba(232, 225, 210, 0.5);
                    ">
                        <span style="
                            color: #7a7a7a;
                            font-weight: 600;
                            min-width: 100px;
                            margin-right: 12px;
                        ">${key}:</span>
                        <span style="
                            color: #5a5a5a;
                            font-weight: 500;
                            flex: 1;
                            word-break: break-all;
                        ">${value || '未填写'}</span>
                    </div>
                `;
            }

            previewHTML += '</div></div>';
        }

        previewHTML += '</div>';

        // 显示预览窗口
        this.showModal('种子信息预览', previewHTML);
    }

    showModal(title, content, callback = null) {
        // 创建模态窗口
        const modal = document.createElement('div');
        modal.style.cssText = `
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            background: linear-gradient(135deg, rgba(90, 90, 90, 0.8) 0%, rgba(60, 60, 60, 0.9) 100%);
            backdrop-filter: blur(8px);
            -webkit-backdrop-filter: blur(8px);
            z-index: 10000;
            display: flex;
            align-items: center;
            justify-content: center;
            opacity: 0;
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
        `;

        const modalContent = document.createElement('div');
        modalContent.style.cssText = `
            background: linear-gradient(135deg, #fdf8ec 0%, #f7f3e8 100%);
            border: 2px solid #d1c9b8;
            border-radius: 12px;
            padding: 0;
            max-width: 700px;
            max-height: 85vh;
            overflow: hidden;
            box-shadow:
                0 20px 60px rgba(0, 0, 0, 0.3),
                0 8px 25px rgba(0, 0, 0, 0.15),
                inset 0 1px 0 rgba(255, 255, 255, 0.8);
            transform: scale(0.9) translateY(-20px);
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen-Sans, Ubuntu, Cantarell, "Helvetica Neue", sans-serif;
        `;

        // 创建标题栏
        const modalHeader = document.createElement('div');
        modalHeader.style.cssText = `
            background: linear-gradient(135deg, #7a9e7a 0%, #6a8e6a 100%);
            color: white;
            padding: 15px 20px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-bottom: 1px solid rgba(209, 201, 184, 0.3);
        `;

        const modalTitle = document.createElement('h2');
        modalTitle.style.cssText = `
            margin: 0;
            font-size: 16px;
            font-weight: 600;
            text-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
            letter-spacing: 0.3px;
        `;
        modalTitle.textContent = title;

        const closeButton = document.createElement('button');
        closeButton.style.cssText = `
            background: rgba(255, 255, 255, 0.15);
            border: 1px solid rgba(255, 255, 255, 0.2);
            color: white;
            width: 32px;
            height: 32px;
            border-radius: 8px;
            cursor: pointer;
            font-size: 18px;
            font-weight: bold;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            backdrop-filter: blur(5px);
        `;
        closeButton.innerHTML = '×';
        closeButton.title = '关闭';

        closeButton.addEventListener('mouseenter', () => {
            closeButton.style.background = 'rgba(255, 255, 255, 0.25)';
            closeButton.style.transform = 'scale(1.1)';
        });

        closeButton.addEventListener('mouseleave', () => {
            closeButton.style.background = 'rgba(255, 255, 255, 0.15)';
            closeButton.style.transform = 'scale(1)';
        });

        closeButton.addEventListener('click', () => {
            this.closeModal(modal);
        });

        modalHeader.appendChild(modalTitle);
        modalHeader.appendChild(closeButton);

        // 创建内容区域
        const modalBody = document.createElement('div');
        modalBody.style.cssText = `
            padding: 20px;
            max-height: calc(85vh - 80px);
            overflow-y: auto;
            color: #5a5a5a;
            line-height: 1.6;
        `;
        modalBody.innerHTML = content;

        modalContent.appendChild(modalHeader);
        modalContent.appendChild(modalBody);

        modal.className = 'luck-audit-modal';
        modal.appendChild(modalContent);
        document.body.appendChild(modal);

        // 动画显示
        requestAnimationFrame(() => {
            modal.style.opacity = '1';
            modalContent.style.transform = 'scale(1) translateY(0)';

            // 执行回调函数
            if (callback && typeof callback === 'function') {
                setTimeout(callback, 100); // 等待动画完成后执行
            }
        });

        // 点击背景关闭
        modal.addEventListener('click', (e) => {
            if (e.target === modal) {
                this.closeModal(modal);
            }
        });

        // ESC键关闭
        const handleEscape = (e) => {
            if (e.key === 'Escape') {
                this.closeModal(modal);
                document.removeEventListener('keydown', handleEscape);
            }
        };
        document.addEventListener('keydown', handleEscape);
    }

    closeModal(modal) {
        const modalContent = modal.querySelector('div');
        modal.style.opacity = '0';
        modalContent.style.transform = 'scale(0.9) translateY(-20px)';

        setTimeout(() => {
            if (modal.parentNode) {
                modal.remove();
            }
        }, 300);
    }

    // 显示预审核弹出层
    showAuditModal() {
        // 移除已存在的预审核弹出层
        const existingModal = document.getElementById('luck-audit-result-modal');
        if (existingModal) {
            existingModal.remove();
        }

        // 创建预审核弹出层
        const modal = document.createElement('div');
        modal.id = 'luck-audit-result-modal';
        modal.style.cssText = `
            position: fixed;
            bottom: 20px;
            right: 20px;
            width: 420px;
            height: 480px;
            background: linear-gradient(135deg, #fdf8ec 0%, #f7f3e8 100%);
            border: 2px solid #d1c9b8;
            border-radius: 12px;
            box-shadow:
                0 12px 35px rgba(0, 0, 0, 0.15),
                0 5px 15px rgba(0, 0, 0, 0.08),
                inset 0 1px 0 rgba(255, 255, 255, 0.8);
            z-index: 10001;
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen-Sans, Ubuntu, Cantarell, "Helvetica Neue", sans-serif;
            display: flex;
            flex-direction: column;
            opacity: 0;
            transform: translateY(20px) scale(0.95);
            transition: all 0.4s cubic-bezier(0.4, 0, 0.2, 1);
            backdrop-filter: blur(15px);
            -webkit-backdrop-filter: blur(15px);
        `;

        // 添加状态数据
        modal.dataset.locked = 'true';  // 默认锁定
        modal.dataset.pinned = 'true';  // 默认固定

        // 创建标题栏
        const header = document.createElement('div');
        header.style.cssText = `
            background: linear-gradient(135deg, #7a9e7a 0%, #6a8e6a 100%);
            color: white;
            padding: 10px 14px;
            border-radius: 10px 10px 0 0;
            cursor: move;
            user-select: none;
            display: flex;
            justify-content: space-between;
            align-items: center;
            font-weight: 600;
            font-size: 13px;
            text-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
            box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.2);
        `;

        const title = document.createElement('span');
        title.textContent = '🔍 预审核结果';
        title.style.cssText = 'display: flex; align-items: center; gap: 6px;';

        const controls = document.createElement('div');
        controls.style.cssText = 'display: flex; gap: 4px;';

        // 锁定按钮
        const lockBtn = document.createElement('button');
        lockBtn.innerHTML = '🔒';
        lockBtn.title = '锁定状态：窗口不会自动收起';
        lockBtn.style.cssText = `
            background: rgba(255, 255, 255, 0.2);
            border: 1px solid rgba(255, 255, 255, 0.3);
            color: white;
            width: 22px;
            height: 22px;
            border-radius: 5px;
            cursor: pointer;
            font-size: 10px;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.3s ease;
        `;

        // 固定按钮
        const pinBtn = document.createElement('button');
        pinBtn.innerHTML = '📌';
        pinBtn.title = '固定状态：窗口固定在右下角';
        pinBtn.style.cssText = `
            background: rgba(255, 255, 255, 0.2);
            border: 1px solid rgba(255, 255, 255, 0.3);
            color: white;
            width: 22px;
            height: 22px;
            border-radius: 5px;
            cursor: pointer;
            font-size: 10px;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.3s ease;
        `;

        // 收起/展开按钮
        const toggleBtn = document.createElement('button');
        toggleBtn.innerHTML = '−';
        toggleBtn.title = '收起/展开窗口';
        toggleBtn.style.cssText = `
            background: rgba(255, 255, 255, 0.15);
            border: 1px solid rgba(255, 255, 255, 0.2);
            color: white;
            width: 22px;
            height: 22px;
            border-radius: 5px;
            cursor: pointer;
            font-size: 14px;
            font-weight: bold;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.3s ease;
        `;

        // 关闭按钮
        const closeBtn = document.createElement('button');
        closeBtn.innerHTML = '×';
        closeBtn.title = '关闭窗口';
        closeBtn.style.cssText = `
            background: rgba(255, 255, 255, 0.15);
            border: 1px solid rgba(255, 255, 255, 0.2);
            color: white;
            width: 22px;
            height: 22px;
            border-radius: 5px;
            cursor: pointer;
            font-size: 16px;
            font-weight: bold;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.3s ease;
        `;

        controls.appendChild(lockBtn);
        controls.appendChild(pinBtn);
        controls.appendChild(toggleBtn);
        controls.appendChild(closeBtn);
        header.appendChild(title);
        header.appendChild(controls);

        // 创建内容区域
        const content = document.createElement('div');
        content.id = 'audit-modal-content';
        content.style.cssText = `
            flex: 1;
            padding: 14px;
            overflow-y: auto;
            display: flex;
            align-items: center;
            justify-content: center;
            color: #5a5a5a;
            font-size: 13px;
            transition: all 0.3s ease;
            background: rgba(255, 255, 255, 0.3);
            margin: 2px;
            border-radius: 0 0 10px 10px;
        `;

        // 初始加载状态
        content.innerHTML = `
            <div style="text-align: center;">
                <div style="
                    width: 36px;
                    height: 36px;
                    border: 3px solid #e8e1d2;
                    border-top: 3px solid #7a9e7a;
                    border-radius: 50%;
                    animation: spin 1s linear infinite;
                    margin: 0 auto 14px;
                "></div>
                <div style="
                    color: #7a9e7a;
                    font-weight: 600;
                    font-size: 14px;
                ">正在预审核，请稍候...</div>
                <div style="
                    color: #999;
                    font-size: 11px;
                    margin-top: 6px;
                ">正在分析种子信息...</div>
            </div>
            <style>
                @keyframes spin {
                    0% { transform: rotate(0deg); }
                    100% { transform: rotate(360deg); }
                }
            </style>
        `;

        modal.appendChild(header);
        modal.appendChild(content);
        document.body.appendChild(modal);

        // 事件处理
        let isMinimized = false;
        let isDragging = false;
        let dragOffset = { x: 0, y: 0 };

        // 锁定功能
        const toggleLock = () => {
            const isLocked = modal.dataset.locked === 'true';
            modal.dataset.locked = isLocked ? 'false' : 'true';
            lockBtn.innerHTML = isLocked ? '🔓' : '🔒';
            lockBtn.title = isLocked ? '解锁状态：窗口会自动收起' : '锁定状态：窗口不会自动收起';
            lockBtn.style.background = isLocked ? 'rgba(255, 255, 255, 0.15)' : 'rgba(255, 255, 255, 0.3)';
        };

        // 固定功能
        const togglePin = () => {
            const isPinned = modal.dataset.pinned === 'true';
            modal.dataset.pinned = isPinned ? 'false' : 'true';
            pinBtn.innerHTML = isPinned ? '📍' : '📌';
            pinBtn.title = isPinned ? '取消固定：可以拖动窗口' : '固定状态：窗口固定在右下角';
            pinBtn.style.background = isPinned ? 'rgba(255, 255, 255, 0.15)' : 'rgba(255, 255, 255, 0.3)';
            header.style.cursor = isPinned ? 'pointer' : 'move';

            if (!isPinned) {
                // 取消固定时重置位置
                modal.style.bottom = '20px';
                modal.style.right = '20px';
                modal.style.left = 'auto';
                modal.style.top = 'auto';
            }
        };

        // 收起/展开功能
        const toggleModal = () => {
            isMinimized = !isMinimized;
            if (isMinimized) {
                modal.style.height = '44px';
                content.style.display = 'none';
                toggleBtn.innerHTML = '+';
                modal.classList.add('minimized');
            } else {
                modal.style.height = '480px';
                content.style.display = 'flex';
                toggleBtn.innerHTML = '−';
                modal.classList.remove('minimized');
            }
        };

        // 拖拽功能
        const startDrag = (e) => {
            if (modal.dataset.pinned === 'true') return;

            isDragging = true;
            const rect = modal.getBoundingClientRect();
            dragOffset.x = e.clientX - rect.left;
            dragOffset.y = e.clientY - rect.top;

            modal.style.transition = 'none';
            modal.style.cursor = 'grabbing';
            document.body.style.userSelect = 'none';
        };

        const drag = (e) => {
            if (!isDragging || modal.dataset.pinned === 'true') return;

            e.preventDefault();
            const x = e.clientX - dragOffset.x;
            const y = e.clientY - dragOffset.y;

            // 限制在视窗内
            const maxX = window.innerWidth - modal.offsetWidth;
            const maxY = window.innerHeight - modal.offsetHeight;

            modal.style.left = Math.max(0, Math.min(x, maxX)) + 'px';
            modal.style.top = Math.max(0, Math.min(y, maxY)) + 'px';
            modal.style.right = 'auto';
            modal.style.bottom = 'auto';
        };

        const endDrag = () => {
            if (!isDragging) return;

            isDragging = false;
            modal.style.transition = 'all 0.4s cubic-bezier(0.4, 0, 0.2, 1)';
            modal.style.cursor = 'default';
            document.body.style.userSelect = '';
        };

        // 事件绑定
        lockBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            toggleLock();
        });

        pinBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            togglePin();
        });

        toggleBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            toggleModal();
        });

        header.addEventListener('click', () => {
            if (isMinimized) {
                toggleModal();
            }
        });

        closeBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            modal.style.opacity = '0';
            modal.style.transform = 'translateY(20px) scale(0.95)';
            setTimeout(() => modal.remove(), 300);
        });

        // 拖拽事件
        header.addEventListener('mousedown', startDrag);
        document.addEventListener('mousemove', drag);
        document.addEventListener('mouseup', endDrag);

        // 失去焦点自动收起（仅在未锁定时）
        document.addEventListener('click', (e) => {
            if (!modal.contains(e.target) && !isMinimized && modal.dataset.locked === 'false') {
                toggleModal();
            }
        });

        // 按钮悬停效果
        [lockBtn, pinBtn, toggleBtn, closeBtn].forEach(btn => {
            btn.addEventListener('mouseenter', () => {
                btn.style.background = 'rgba(255, 255, 255, 0.3)';
                btn.style.transform = 'scale(1.1)';
            });

            btn.addEventListener('mouseleave', () => {
                const isActive = (btn === lockBtn && modal.dataset.locked === 'true') ||
                                (btn === pinBtn && modal.dataset.pinned === 'true');
                btn.style.background = isActive ? 'rgba(255, 255, 255, 0.3)' : 'rgba(255, 255, 255, 0.15)';
                btn.style.transform = 'scale(1)';
            });
        });

        // 显示动画
        requestAnimationFrame(() => {
            modal.style.opacity = '1';
            modal.style.transform = 'translateY(0) scale(1)';
        });

        return modal;
    }

    // 更新预审核弹出层显示结果
    updateAuditModal(result) {
        const content = document.getElementById('audit-modal-content');
        if (!content) return;

        const data = result.data;
        const passed = data.passed;
        const status = data.status;
        const totalScore = data.totalScore;
        const details = data.details || [];
        const suggestions = data.suggestions || [];

        // 构建结果HTML
        let resultHTML = `
            <div style="width: 100%; height: 100%; overflow-y: auto; padding: 2px;">
                <!-- 总体状态 -->
                <div style="
                    background: ${passed ? 'linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%)' : 'linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%)'};
                    border: 1px solid ${passed ? '#c3e6cb' : '#f5c6cb'};
                    border-radius: 6px;
                    padding: 10px;
                    margin-bottom: 12px;
                    text-align: center;
                    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.1);
                ">
                    <div style="
                        font-size: 16px;
                        font-weight: bold;
                        color: ${passed ? '#155724' : '#721c24'};
                        margin-bottom: 3px;
                        display: flex;
                        align-items: center;
                        justify-content: center;
                        gap: 6px;
                    ">${passed ? '✅' : '❌'} ${status}</div>
                    <div style="
                        font-size: 13px;
                        color: ${passed ? '#155724' : '#721c24'};
                        opacity: 0.9;
                        margin-bottom: 3px;
                    ">总分: <strong>${totalScore}/100</strong></div>
                    <div style="
                        font-size: 11px;
                        color: ${passed ? '#155724' : '#721c24'};
                        opacity: 0.8;
                        display: flex;
                        justify-content: center;
                        gap: 8px;
                    ">
                        <span>❌ ${data.errorCount || 0}</span>
                        <span>⚠️ ${data.warningCount || 0}</span>
                        <span>🔍 ${data.suspiciousCount || 0}</span>
                    </div>
                </div>

                <!-- 详细问题列表 -->
                ${details.length > 0 ? `
                    <div style="margin-bottom: 12px;">
                        <div style="
                            font-weight: 600;
                            color: #5a5a5a;
                            margin-bottom: 6px;
                            font-size: 12px;
                            display: flex;
                            align-items: center;
                            gap: 4px;
                        ">📋 问题详情 <span style="
                            background: #e9ecef;
                            color: #6c757d;
                            padding: 1px 5px;
                            border-radius: 8px;
                            font-size: 10px;
                        ">${details.length}</span></div>
                        ${details.map((detail, index) => `
                            <div style="
                                background: ${this.getDetailBackgroundColor(detail.level)};
                                border: 1px solid ${this.getDetailBorderColor(detail.level)};
                                border-radius: 5px;
                                padding: 8px;
                                margin-bottom: 6px;
                                font-size: 11px;
                                box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
                            ">
                                <div style="
                                    display: flex;
                                    justify-content: space-between;
                                    align-items: flex-start;
                                    margin-bottom: 3px;
                                    gap: 8px;
                                ">
                                    <span style="
                                        font-weight: 600;
                                        color: ${this.getDetailTextColor(detail.level)};
                                        line-height: 1.3;
                                        flex: 1;
                                    ">${this.getLevelIcon(detail.level)} ${detail.message}</span>
                                    <span style="
                                        background: ${this.getDetailTextColor(detail.level)};
                                        color: white;
                                        padding: 1px 5px;
                                        border-radius: 3px;
                                        font-size: 9px;
                                        font-weight: bold;
                                        white-space: nowrap;
                                    ">-${detail.score}</span>
                                </div>
                                <div style="
                                    color: ${this.getDetailTextColor(detail.level)};
                                    opacity: 0.85;
                                    margin-bottom: 3px;
                                    font-size: 10px;
                                    line-height: 1.3;
                                ">${detail.details}</div>
                                <div style="
                                    color: ${this.getDetailTextColor(detail.level)};
                                    font-size: 10px;
                                    font-style: italic;
                                    opacity: 0.9;
                                    padding: 3px 6px;
                                    background: rgba(255, 255, 255, 0.3);
                                    border-radius: 3px;
                                    border-left: 2px solid ${this.getDetailTextColor(detail.level)};
                                ">💡 ${detail.suggestion}</div>
                            </div>
                        `).join('')}
                    </div>
                ` : ''}

                <!-- 建议列表 -->
                ${suggestions.length > 0 ? `
                    <div style="margin-bottom: 12px;">
                        <div style="
                            font-weight: 600;
                            color: #5a5a5a;
                            margin-bottom: 6px;
                            font-size: 12px;
                            display: flex;
                            align-items: center;
                            gap: 4px;
                        ">💡 修改建议 <span style="
                            background: #e9ecef;
                            color: #6c757d;
                            padding: 1px 5px;
                            border-radius: 8px;
                            font-size: 10px;
                        ">${suggestions.length}</span></div>
                        ${suggestions.map((suggestion, index) => `
                            <div style="
                                background: linear-gradient(135deg, #fff3cd 0%, #ffeaa7 100%);
                                border: 1px solid #ffeaa7;
                                border-radius: 5px;
                                padding: 6px 8px;
                                margin-bottom: 4px;
                                font-size: 11px;
                                color: #856404;
                                line-height: 1.3;
                                box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
                                border-left: 3px solid #ffc107;
                            ">${suggestion}</div>
                        `).join('')}
                    </div>
                ` : ''}

                <!-- 审核时间和操作 -->
                <div style="
                    text-align: center;
                    font-size: 10px;
                    color: #999;
                    padding: 8px 0;
                    border-top: 1px solid rgba(209, 201, 184, 0.3);
                    background: rgba(255, 255, 255, 0.2);
                    border-radius: 0 0 8px 8px;
                    margin: 0 -14px -14px -14px;
                ">
                    <div style="margin-bottom: 4px;">
                        🕒 审核时间: ${data.auditTime}
                    </div>
                    <div style="
                        font-size: 9px;
                        opacity: 0.7;
                    ">
                        ${passed ? '恭喜通过预审核！可以正式发布' : '请根据上述提示修改后重新预审核'}
                    </div>
                </div>
            </div>
        `;

        content.innerHTML = resultHTML;
        content.style.display = 'block';
        content.style.alignItems = 'flex-start';
        content.style.justifyContent = 'flex-start';
        content.style.padding = '12px';
    }

    // 更新预审核弹出层显示错误
    updateAuditModalError(errorMessage) {
        const content = document.getElementById('audit-modal-content');
        if (!content) return;

        content.innerHTML = `
            <div style="
                width: 100%;
                text-align: center;
                padding: 16px;
                display: flex;
                flex-direction: column;
                align-items: center;
                justify-content: center;
                height: 100%;
            ">
                <div style="
                    width: 60px;
                    height: 60px;
                    background: linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%);
                    border-radius: 50%;
                    display: flex;
                    align-items: center;
                    justify-content: center;
                    font-size: 28px;
                    margin-bottom: 12px;
                    box-shadow: 0 4px 12px rgba(248, 215, 218, 0.4);
                ">❌</div>
                <div style="
                    font-size: 16px;
                    font-weight: 600;
                    color: #721c24;
                    margin-bottom: 8px;
                ">预审核失败</div>
                <div style="
                    font-size: 11px;
                    color: #721c24;
                    background: linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%);
                    border: 1px solid #f5c6cb;
                    border-radius: 6px;
                    padding: 10px 12px;
                    margin-top: 8px;
                    max-width: 90%;
                    line-height: 1.4;
                    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.1);
                    border-left: 3px solid #dc3545;
                ">${errorMessage}</div>
                <div style="
                    font-size: 10px;
                    color: #999;
                    margin-top: 12px;
                    opacity: 0.7;
                ">请检查网络连接或稍后重试</div>
            </div>
        `;

        content.style.display = 'flex';
        content.style.alignItems = 'center';
        content.style.justifyContent = 'center';
        content.style.padding = '12px';
    }

    // 辅助函数：获取详情背景色
    getDetailBackgroundColor(level) {
        switch (level) {
            case 'ERROR': return 'linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%)';
            case 'WARNING': return 'linear-gradient(135deg, #fff3cd 0%, #ffeaa7 100%)';
            case 'SUSPICIOUS': return 'linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%)';
            default: return 'linear-gradient(135deg, #e2e3e5 0%, #d6d8db 100%)';
        }
    }

    // 辅助函数：获取详情边框色
    getDetailBorderColor(level) {
        switch (level) {
            case 'ERROR': return '#f5c6cb';
            case 'WARNING': return '#ffeaa7';
            case 'SUSPICIOUS': return '#bee5eb';
            default: return '#d6d8db';
        }
    }

    // 辅助函数：获取详情文字色
    getDetailTextColor(level) {
        switch (level) {
            case 'ERROR': return '#721c24';
            case 'WARNING': return '#856404';
            case 'SUSPICIOUS': return '#0c5460';
            default: return '#6c757d';
        }
    }

    // 辅助函数：获取级别图标
    getLevelIcon(level) {
        switch (level) {
            case 'ERROR': return '🚫';
            case 'WARNING': return '⚠️';
            case 'SUSPICIOUS': return '🔍';
            default: return 'ℹ️';
        }
    }



    getPageType() {
        // 判断当前页面类型
        const path = window.location.pathname;
        if (path.includes('upload.php')) {
            return 'upload';
        } else if (path.includes('edit.php')) {
            return 'edit';
        }
        return 'unknown';
    }



    previewJSON() {
        if (!this.torrentData || !this.torrentData.name) {
            this.updateStatus('请先收集种子信息', 'warning');
            return;
        }

        // 创建预览数据
        const previewData = {
            ...this.torrentData,
            export_time: new Date().toISOString(),
            page_url: window.location.href
        };

        // 格式化JSON
        const jsonStr = JSON.stringify(previewData, null, 2);

        // 创建预览HTML
        const previewHTML = `
            <div style="
                font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Roboto Mono', 'Courier New', monospace;
                font-size: 12px;
                line-height: 1.5;
                max-height: 500px;
                overflow-y: auto;
                background: linear-gradient(135deg, #2a2a2a 0%, #1e1e1e 100%);
                padding: 20px;
                border-radius: 8px;
                border: 1px solid #d1c9b8;
                box-shadow: inset 0 2px 8px rgba(0, 0, 0, 0.3);
            ">
                <div style="
                    margin-bottom: 15px;
                    font-weight: 600;
                    color: #7a9e7a;
                    font-size: 14px;
                    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.5);
                ">📄 种子信息JSON预览</div>
                <pre style="
                    margin: 0;
                    white-space: pre-wrap;
                    word-wrap: break-word;
                    color: #e8e8e8;
                    background: transparent;
                    border: none;
                    font-size: 11px;
                    text-shadow: 0 1px 1px rgba(0, 0, 0, 0.5);
                ">${this.escapeHtml(jsonStr)}</pre>
            </div>
            <div style="margin-top: 20px; text-align: center; padding-top: 15px; border-top: 1px solid rgba(209, 201, 184, 0.3);">
                <button id="copy-json-btn" style="
                    background: linear-gradient(135deg, #7a9e7a 0%, #6a8e6a 100%);
                    color: white;
                    border: 1px solid #6a8e6a;
                    padding: 10px 20px;
                    border-radius: 8px;
                    cursor: pointer;
                    font-size: 13px;
                    font-weight: 600;
                    margin-right: 12px;
                    transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
                    text-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
                    box-shadow: 0 2px 8px rgba(122, 158, 122, 0.3);
                ">复制JSON</button>
            </div>
        `;

        // 显示预览窗口
        this.showModal('JSON预览', previewHTML, () => {
            // 模态窗口显示后的回调，设置复制按钮事件
            const copyBtn = document.getElementById('copy-json-btn');
            if (copyBtn) {
                copyBtn.addEventListener('click', () => {
                    this.copyToClipboard(jsonStr, copyBtn);
                });

                copyBtn.addEventListener('mouseenter', () => {
                    copyBtn.style.transform = 'translateY(-2px)';
                    copyBtn.style.boxShadow = '0 4px 12px rgba(122, 158, 122, 0.4)';
                });

                copyBtn.addEventListener('mouseleave', () => {
                    copyBtn.style.transform = 'translateY(0)';
                    copyBtn.style.boxShadow = '0 2px 8px rgba(122, 158, 122, 0.3)';
                });
            }
        });
    }

    exportJSON() {
        if (!this.torrentData || !this.torrentData.name) {
            this.updateStatus('请先收集种子信息', 'warning');
            return;
        }

        // 创建导出数据
        const exportData = {
            ...this.torrentData,
            export_time: new Date().toISOString(),
            page_url: window.location.href,
            user_agent: navigator.userAgent
        };

        // 创建下载链接
        const dataStr = JSON.stringify(exportData, null, 2);
        const dataBlob = new Blob([dataStr], { type: 'application/json' });
        const url = URL.createObjectURL(dataBlob);

        const link = document.createElement('a');
        link.href = url;
        link.download = `torrent_data_${new Date().getTime()}.json`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        URL.revokeObjectURL(url);

        this.updateStatus('JSON文件已导出', 'success');
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    async copyToClipboard(text, button) {
        try {
            await navigator.clipboard.writeText(text);

            // 更新按钮状态
            const originalText = button.textContent;
            const originalBackground = button.style.background;

            button.textContent = '已复制！';
            button.style.background = 'linear-gradient(135deg, #8aac8a 0%, #7a9e7a 100%)';
            button.style.transform = 'scale(1.05)';

            // 2秒后恢复
            setTimeout(() => {
                button.textContent = originalText;
                button.style.background = originalBackground;
                button.style.transform = 'scale(1)';
            }, 2000);

        } catch (err) {
            console.error('复制失败:', err);

            // 降级方案：使用传统方法
            const textArea = document.createElement('textarea');
            textArea.value = text;
            textArea.style.position = 'fixed';
            textArea.style.left = '-999999px';
            textArea.style.top = '-999999px';
            document.body.appendChild(textArea);
            textArea.focus();
            textArea.select();

            try {
                document.execCommand('copy');
                button.textContent = '已复制！';
                button.style.background = 'linear-gradient(135deg, #8aac8a 0%, #7a9e7a 100%)';

                setTimeout(() => {
                    button.textContent = '复制JSON';
                    button.style.background = 'linear-gradient(135deg, #7a9e7a 0%, #6a8e6a 100%)';
                }, 2000);
            } catch (err2) {
                button.textContent = '复制失败';
                button.style.background = 'linear-gradient(135deg, #d46a6a 0%, #c55a5a 100%)';

                setTimeout(() => {
                    button.textContent = '复制JSON';
                    button.style.background = 'linear-gradient(135deg, #7a9e7a 0%, #6a8e6a 100%)';
                }, 2000);
            }

            document.body.removeChild(textArea);
        }
    }

    clearForm() {
        if (!confirm('确定要清空表单吗？此操作不可撤销！')) {
            return;
        }

        try {
            // 清空基本字段
            const fields = ['name', 'small_descr', 'url', 'technical_info'];
            fields.forEach(fieldName => {
                const field = document.querySelector(`[name="${fieldName}"]`);
                if (field) {
                    field.value = '';
                }
            });

            // 清空简介
            const descrField = document.querySelector('textarea[name="descr"]');
            if (descrField) {
                descrField.value = '';
            }

            // 重置类型选择
            const typeSelects = document.querySelectorAll('select[name="type"]');
            typeSelects.forEach(select => {
                select.value = '0';
            });

            // 重置质量选择
            const qualitySelects = document.querySelectorAll('select[name^="source"], select[name^="medium"], select[name^="codec"], select[name^="standard"], select[name^="processing"], select[name^="audiocodec"]');
            qualitySelects.forEach(select => {
                select.value = '0';
            });

            // 取消所有标签选择
            const tagCheckboxes = document.querySelectorAll('input[type="checkbox"][name^="tags"]');
            tagCheckboxes.forEach(checkbox => {
                checkbox.checked = false;
            });

            // 清空内部数据
            this.torrentData = {};
            this.updateDisplay();
            this.updateStatus('表单已清空', 'info');

        } catch (error) {
            console.error('清空表单时出错:', error);
            this.updateStatus('清空表单时出错: ' + error.message, 'error');
        }
    }
}

// 全局实例
let luckAuditPanel = null;

// 初始化函数
function initLuckAudit() {
    // 检查是否在种子发布或编辑页面
    const path = window.location.pathname;
    if (path.includes('upload.php') || path.includes('edit.php')) {
        luckAuditPanel = new LuckAuditPanel();

        // 默认显示面板
        setTimeout(() => {
            luckAuditPanel.show();
        }, 500);

        // 添加快捷键支持 (Ctrl+Shift+L)
        document.addEventListener('keydown', (e) => {
            if (e.ctrlKey && e.shiftKey && e.key === 'L') {
                e.preventDefault();
                luckAuditPanel.toggle();
            }
        });

        // 添加页面按钮
        addLuckAuditButton();

        console.log('LuckAudit控制面板已初始化');
    }
}

// 添加控制按钮到页面
function addLuckAuditButton() {
    // 查找合适的位置添加按钮
    const targetElement = document.querySelector('form#compose') ||
                         document.querySelector('form[name="edittorrent"]') ||
                         document.querySelector('table');

    if (targetElement) {
        const button = document.createElement('button');
        button.type = 'button';
        button.innerHTML = '🔍 LuckAudit控制面板';
        button.style.cssText = `
            position: fixed;
            top: 20px;
            right: 20px;
            z-index: 9998;
            background: linear-gradient(135deg, #7a9e7a 0%, #6a8e6a 100%);
            color: white;
            border: 1px solid #6a8e6a;
            border-radius: 8px;
            padding: 12px 18px;
            font-size: 12px;
            font-weight: 600;
            cursor: pointer;
            box-shadow: 0 4px 15px rgba(122, 158, 122, 0.3);
            transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
            text-shadow: 0 1px 2px rgba(0, 0, 0, 0.2);
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen-Sans, Ubuntu, Cantarell, "Helvetica Neue", sans-serif;
            backdrop-filter: blur(10px);
            -webkit-backdrop-filter: blur(10px);
        `;

        button.addEventListener('click', () => {
            luckAuditPanel.toggle();
        });

        button.addEventListener('mouseenter', () => {
            button.style.transform = 'translateY(-3px) scale(1.05)';
            button.style.boxShadow = '0 6px 20px rgba(122, 158, 122, 0.4)';
            button.style.background = 'linear-gradient(135deg, #6a8e6a 0%, #5a7e5a 100%)';
        });

        button.addEventListener('mouseleave', () => {
            button.style.transform = 'translateY(0) scale(1)';
            button.style.boxShadow = '0 4px 15px rgba(122, 158, 122, 0.3)';
            button.style.background = 'linear-gradient(135deg, #7a9e7a 0%, #6a8e6a 100%)';
        });

        document.body.appendChild(button);
    }
}

// 页面加载完成后初始化
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initLuckAudit);
} else {
    initLuckAudit();
}

// 页面卸载时清理资源
window.addEventListener('beforeunload', () => {
    if (luckAuditPanel) {
        luckAuditPanel.cleanup();
        luckAuditPanel = null;
    }
});

// 导出给全局使用
window.LuckAuditPanel = LuckAuditPanel;
if (luckAuditPanel) {
    window.luckAuditPanel = luckAuditPanel;
}
