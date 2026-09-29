import { createFxAgent, supportsJspi } from '/vendor/fx/browser.js';
import { createFxConversationStore, fxAuthorizationScope } from '/modules/fx-conversation-store.js';

export function createFxAssistant({ state, api, $, esc, toast, renderWorkspace, runtime, tokenKey, resetAgentConversation }) {
  const conversations = createFxConversationStore();
  state.fxConversationMessages ||= [];
  state.fxPreviewedVersions ||= new Map();
  state.fxTurnNumber ||= 0;

  function appendChat(message, role) {
    const wrap = document.createElement('div');
    wrap.className = `chat ${role === 'user' ? 'chat-end' : 'chat-start'}`;
    const bubble = document.createElement('div');
    bubble.className = `chat-bubble ${role === 'user' ? 'chat-bubble-primary' : 'chat-bubble-neutral'}`;
    bubble.textContent = message;
    wrap.append(bubble);
    $('#chat-messages').append(wrap);
    $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight;
    return bubble;
  }

  const conversationKey = () => state.user?.id && state.tenant?.id ? `${state.user.id}:${state.tenant.id}` : null;

  function showRestoredConversation(messages) {
    const root = $('#chat-messages');
    root.replaceChildren();
    const note = document.createElement('div');
    note.className = 'alert alert-info fx-resume-notice';
    note.setAttribute('role', 'status');
    note.textContent = '已续接此账号在当前工作区的 fx 对话。旧的真实数据预览卡片不会恢复；如需发布界面，请重新预览并在之后的消息中明确确认。';
    root.append(note);
    for (const message of messages) appendChat(message.content, message.role);
  }

  function discardActiveConversation() {
    if (state.fxAgent) state.fxAgent.close().catch(() => {});
    state.fxAgent = null;
    state.fxPendingCheckpoint = null;
    state.fxConversationMessages = [];
    state.fxConversationRevision = 0;
    state.fxConversationLoadedKey = null;
    state.fxConversationScope = null;
    state.fxPreviewedVersions = new Map();
    state.fxTurnNumber = 0;
    state.fxPersistenceConflict = false;
    state.fxBusy = false;
  }

  async function refreshAuthorization() {
    const me = await api('/api/me');
    const oldUserId = state.user?.id;
    const oldTenantId = state.tenant?.id;
    const oldKey = conversationKey();
    const nextScope = fxAuthorizationScope(me.tenant, me.apps);
    const nextKey = me.user?.id && me.tenant?.id ? `${me.user.id}:${me.tenant.id}` : null;
    const scopeChanged = oldKey && oldKey === nextKey && state.fxConversationLoadedKey === oldKey && state.fxConversationScope && state.fxConversationScope !== nextScope;
    if (oldUserId && oldUserId !== me.user?.id) {
      await conversations.clearAll();
      discardActiveConversation();
    } else if (scopeChanged) {
      await conversations.clear(me.user.id, me.tenant.id);
      discardActiveConversation();
      resetAgentConversation();
      toast('工作区角色或应用权限已变化，旧 fx 对话已清除；请基于当前权限重新开始。', true);
    } else if (oldTenantId && oldTenantId !== me.tenant?.id) {
      discardActiveConversation();
    }

    await conversations.retainAccount(me.user.id);
    await conversations.retainWorkspaces(me.user.id, (me.workspaces || []).map((workspace) => workspace.id));
    state.user = me.user;
    state.tenant = me.tenant;
    state.workspaces = me.workspaces || [];
    state.apps = me.apps || [];
    state.aiConfigured = me.ai_configured;
    state.isPlatformAdmin = me.is_platform_admin;
    if (state.app) {
      state.app = state.apps.find((item) => item.id === state.app.id) || null;
      if (!state.app && state.workspaceView === 'app') state.workspaceView = 'home';
    }
    state.fxConversationScope = nextScope;
    return { scopeChanged, key: nextKey, scope: nextScope };
  }

  async function enterConversation() {
    const access = await refreshAuthorization();
    const { key, scope } = access;
    if (!key || key !== conversationKey()) return false;
    if (state.fxConversationLoadedKey === key && state.fxConversationScope === scope) return true;
    state.fxPendingCheckpoint = null;
    state.fxConversationMessages = [];
    state.fxConversationRevision = 0;
    state.fxPreviewedVersions = new Map();
    state.fxTurnNumber = 0;
    const [userId, tenantId] = key.split(':');
    const saved = await conversations.load(userId, tenantId, scope);
    if (key !== conversationKey()) return false;
    state.fxConversationLoadedKey = key;
    state.fxConversationScope = scope;
    if (!saved) return false;
    state.fxPendingCheckpoint = saved.checkpoint;
    state.fxConversationRevision = saved.revision;
    state.fxConversationMessages = saved.messages;
    showRestoredConversation(saved.messages);
    $('#agent-status').textContent = '对话已续接';
    $('#agent-status').className = 'badge badge-info';
    return true;
  }

  async function persistConversation() {
    if (!state.fxAgent || state.fxPersistenceConflict || !state.fxConversationLoadedKey) return;
    const access = await refreshAuthorization();
    if (access.scopeChanged) return;
    if (access.key !== state.fxConversationLoadedKey || access.scope !== state.fxConversationScope) return;
    const checkpoint = await state.fxAgent.checkpoint();
    const [userId, tenantId] = access.key.split(':');
    const result = await conversations.save({
      userId,
      tenantId,
      scope: access.scope,
      expectedRevision: state.fxConversationRevision,
      checkpoint,
      messages: state.fxConversationMessages,
    });
    if (result.conflict) {
      state.fxPersistenceConflict = true;
      toast('此 fx 对话已在另一个标签页更新。本次对话不会覆盖已保存版本；请刷新页面后继续。', true);
      return;
    }
    if (result.changed) {
      await conversations.clear(userId, tenantId);
      discardActiveConversation();
      resetAgentConversation();
      toast('工作区权限已变化，fx 对话未保存。请刷新权限后重新开始。', true);
      return;
    }
    state.fxConversationRevision = result.revision;
  }

  async function clearSavedConversation() {
    const key = conversationKey();
    if (!key) return;
    const [userId, tenantId] = key.split(':');
    await conversations.clear(userId, tenantId);
    discardActiveConversation();
    resetAgentConversation();
    state.fxConversationLoadedKey = key;
    state.fxConversationScope = fxAuthorizationScope(state.tenant, state.apps);
    toast('当前工作区的私人 fx 对话已从此浏览器清除。');
  }

  async function clearSavedConversations() {
    await conversations.clearAll();
    discardActiveConversation();
  }

  const toolResult = (value) => JSON.stringify(value);
  function agentTools() {
    const request = (path, options) => {
      if (!state.app && !path.startsWith('/apps')) throw new Error('请先通过 create_app 创建或通过 activate_app 选择一个工具。');
      return api(state.app ? `/api/apps/${state.app.id}${path}` : `/api${path}`, options);
    };
    const requireFxEditor = () => {
      if (!state.app) throw new Error('请先选择要修改的应用。');
      if (state.app.permission === 'viewer') throw new Error('你只有此应用的查看权限，不能创建或发布界面。');
    };
    const tableSchema = { type: 'object', required: ['name', 'fields'], properties: { name: { type: 'string' }, fields: { type: 'array', items: { type: 'object', required: ['name', 'label'], properties: { name: { type: 'string' }, label: { type: 'string' }, type: { type: 'string', enum: ['text', 'number', 'bool', 'date', 'email', 'url', 'select', 'relation'] }, required: { type: 'boolean' }, options: { type: 'array', items: { type: 'string' } }, target: { type: 'string' } } } } } };
    const uiDefinitionSchema = { type: 'object', required: ['schema_version', 'title', 'collection', 'fields'], additionalProperties: false, properties: { schema_version: { type: 'integer', enum: [1] }, title: { type: 'string', maxLength: 120 }, collection: { type: 'string', description: '当前应用中已存在的数据表 slug' }, fields: { type: 'array', minItems: 1, maxItems: 12, uniqueItems: true, items: { type: 'string' } } } };
    return [
      { name: 'list_apps', description: '查看当前工作区可用的工具，帮助用户继续已有工作。', inputSchema: { type: 'object', properties: {} }, async execute() { return toolResult(state.apps); } },
      { name: 'create_app', description: '根据用户确认的工作目标创建新工具。创建后它自动成为当前工具。', inputSchema: { type: 'object', required: ['name', 'description'], properties: { name: { type: 'string' }, description: { type: 'string' } } }, async execute(input) {
        const app = await api('/api/apps', { method: 'POST', body: JSON.stringify(input) });
        state.apps = [app, ...state.apps.filter((item) => item.id !== app.id)]; state.app = app; state.appPanel = 'runtime'; state.table = null;
        await renderWorkspace(); return toolResult({ created: app, next: '工具已创建。可以继续梳理工作流程，并在用户认可后创建所需数据结构。' });
      } },
      { name: 'activate_app', description: '切换当前对话正在处理的工具。先用 list_apps 找到目标工具。', inputSchema: { type: 'object', required: ['app_id'], properties: { app_id: { type: 'string' } } }, async execute({ app_id }) {
        const found = state.apps.find((item) => item.id === app_id); if (!found) throw new Error('当前工作区找不到这个工具');
        state.app = found; state.table = null; await renderWorkspace(); return toolResult({ active_app: found });
      } },
      { name: 'list_tables', description: '了解当前工具的数据结构，为后续工作做准备。', inputSchema: { type: 'object', properties: {} }, async execute() { return toolResult(await request('/collections')); } },
      { name: 'list_ui_versions', description: '查看当前应用的界面草稿和发布历史，发布前必须先确认目标草稿及当前版本。', inputSchema: { type: 'object', properties: {} }, async execute() { return toolResult(await request('/versions')); } },
      { name: 'get_ui_version', description: '读取某个界面版本的具体标题、数据表和字段配置。用它检查历史草稿；如果用户尚未在当前对话看过该草稿，先展示配置并等待明确批准后再发布。', inputSchema: { type: 'object', required: ['version_id'], properties: { version_id: { type: 'string' } } }, async execute({ version_id }) { return toolResult(await request(`/versions/${encodeURIComponent(version_id)}`)); } },
      { name: 'preview_ui_version', description: '在当前 fx 对话中显示一个版本的真实只读界面预览。只读取当前用户有权访问的数据，不写入或更改任何记录。保存新草稿后应立即预览，供用户审阅；不要把预览视为发布授权。', inputSchema: { type: 'object', required: ['version_id'], properties: { version_id: { type: 'string' } } }, async execute({ version_id }) {
        if (!state.app) throw new Error('请先选择要预览的应用。');
        const card = runtime.createPreviewCard(version_id);
        const appName = state.app.name;
        const preview = await runtime.loadPreview(card);
        state.fxPreviewedVersions.set(version_id, state.fxTurnNumber);
        return toolResult({ previewed_version: preview.version, title: preview.title, app: appName, displayed_records: preview.displayed_records, total_records: preview.total_records, note: '对话中已显示只读预览，未修改业务记录。' });
      } },
      { name: 'create_ui_draft', description: '保存一个应用业务列表界面的草稿。只有用户认可界面结构后调用；这不会发布或更改正式界面。保存后应调用 preview_ui_version 显示只读预览。界面定义仅支持 schema_version=1 的单数据表列表，不包含自定义表单布局或任意代码；fields 必须引用当前应用中真实存在的非附件字段。兼容的已发布界面会自动提供基础新增记录表单，但只在界面包含全部可支持的必填字段时开放。', inputSchema: { type: 'object', required: ['definition', 'summary'], properties: { definition: uiDefinitionSchema, summary: { type: 'string', maxLength: 1000 } } }, async execute(input) { requireFxEditor(); return toolResult(await request('/versions', { method: 'POST', body: JSON.stringify(input) })); } },
      { name: 'revise_ui_draft', description: '基于一个尚未发布的草稿创建修订版草稿，保留原草稿与当前已发布界面不变。先读取目标草稿，把拟修改的标题、数据表和字段变更向用户说明；用户认可后才调用。保存成功后立即调用 preview_ui_version 展示新版本的真实只读预览；保存或预览失败时说明错误和恢复办法，不发布、不覆盖已有版本。', inputSchema: { type: 'object', required: ['base_version_id', 'definition', 'summary'], properties: { base_version_id: { type: 'string', description: '刚读取并确认仍处于草稿状态的来源版本 ID' }, definition: uiDefinitionSchema, summary: { type: 'string', maxLength: 1000 } } }, async execute({ base_version_id, definition, summary }) { requireFxEditor(); return toolResult(await request('/versions', { method: 'POST', body: JSON.stringify({ definition, summary, based_on_version_id: base_version_id }) })); } },
      { name: 'restore_ui_version', description: '仅在用户明确要求恢复一个已发布的历史界面后调用。先用 list_ui_versions 和 get_ui_version 确认目标版本，并向用户展示其标题、数据表和字段；说明这是把界面配置另存为新的前向草稿，不会回滚记录或恢复已删除的数据。服务端会核验目标确为更早的已发布版本，并按当前表和字段重新校验。校验不通过则不创建草稿；成功时当前正式界面和历史版本保持不变，应立即展示新草稿的真实只读预览，随后等待用户明确确认发布。', inputSchema: { type: 'object', required: ['version_id'], properties: { version_id: { type: 'string', description: '已读取且用户明确要求恢复的已发布历史版本 ID' } } }, async execute({ version_id }) {
        requireFxEditor();
        const draft = await request(`/versions/${encodeURIComponent(version_id)}/restore`, { method: 'POST' });
        const card = runtime.createPreviewCard(draft.id);
        try {
          const preview = await runtime.loadPreview(card);
          state.fxPreviewedVersions.set(draft.id, state.fxTurnNumber);
          return toolResult({ status: 'draft_created_and_previewed', restored_from_version_id: version_id, draft, preview, note: '历史界面已另存为前向草稿并显示真实只读预览；当前正式界面未更改，业务记录没有恢复或修改。' });
        } catch (error) {
          return toolResult({ status: 'draft_created_preview_failed', restored_from_version_id: version_id, draft, preview_error: error.message || '预览失败', retry: `调用 preview_ui_version，version_id=${draft.id}`, note: '恢复草稿已保存但预览失败；当前正式界面未更改，也未发布。告知用户真实错误并提供重试。' });
        }
      } },
      { name: 'publish_ui_version', description: '将一个已由用户明确批准的界面草稿发布为正式应用界面。只有用户看过当前对话中刚保存的具体草稿并明确要求发布/上线后才可调用；创建或描述草稿不构成发布授权。expected_published_version_id 必须来自刚读取的版本列表，用于阻止覆盖他人的新发布。当前页面必须在上一条用户消息之前成功显示过同一版本的真实预览；同一条消息中刚生成的预览不能作为发布授权。', inputSchema: { type: 'object', required: ['version_id', 'expected_published_version_id'], properties: { version_id: { type: 'string' }, expected_published_version_id: { type: ['string', 'null'] } } }, async execute({ version_id, expected_published_version_id }) {
        requireFxEditor();
        const previewTurn = state.fxPreviewedVersions.get(version_id);
        if (!Number.isInteger(previewTurn) || previewTurn >= state.fxTurnNumber) throw new Error('请先显示并审阅该版本的真实预览，然后在下一条消息中明确确认发布。');
        const result = await request(`/versions/${encodeURIComponent(version_id)}/publish`, { method: 'POST', body: JSON.stringify({ expected_published_version_id }) });
        state.fxPreviewedVersions.delete(version_id);
        state.app = { ...state.app, has_published_version: true };
        state.apps = state.apps.map((item) => item.id === state.app.id ? state.app : item);
        return toolResult(result);
      } },
      { name: 'create_table', description: '按已讨论的工作流程建立数据结构。字段名使用英文 snake_case，label 使用清晰的中文名称；关系字段 target 必须是当前工具中已存在的数据表 slug。', inputSchema: tableSchema, async execute(input) { return toolResult(await request('/collections', { method: 'POST', body: JSON.stringify(input) })); } },
      { name: 'list_records', description: '按用户问题读取当前工具的数据记录。可用搜索和分页缩小结果。', inputSchema: { type: 'object', required: ['table'], properties: { table: { type: 'string', description: '数据表 slug' }, search: { type: 'string' }, page: { type: 'number' }, perPage: { type: 'number' } } }, async execute(input) {
        const params = new URLSearchParams(); for (const key of ['search', 'page', 'perPage']) if (input[key] !== undefined) params.set(key, String(input[key]));
        return toolResult(await request(`/collections/${encodeURIComponent(input.table)}/records?${params}`));
      } },
      { name: 'add_record', description: '根据用户提供的信息新增业务记录；缺失的事实必须先询问，不可猜测。', inputSchema: { type: 'object', required: ['table', 'data'], properties: { table: { type: 'string' }, data: { type: 'object', additionalProperties: true } } }, async execute(input) { return toolResult(await request(`/collections/${encodeURIComponent(input.table)}/records`, { method: 'POST', body: JSON.stringify({ data: input.data }) })); } },
      { name: 'update_record', description: '根据用户明确的指示修改业务记录。先定位并复述目标记录和改动，再调用工具。', inputSchema: { type: 'object', required: ['table', 'record_id', 'data'], properties: { table: { type: 'string' }, record_id: { type: 'string' }, data: { type: 'object', additionalProperties: true } } }, async execute(input) { return toolResult(await request(`/collections/${encodeURIComponent(input.table)}/records/${encodeURIComponent(input.record_id)}`, { method: 'PATCH', body: JSON.stringify({ data: input.data }) })); } },
      { name: 'delete_record', description: '永久删除业务记录。只可在用户明确确认删除具体记录后调用。', inputSchema: { type: 'object', required: ['table', 'record_id'], properties: { table: { type: 'string' }, record_id: { type: 'string' } } }, async execute(input) { return toolResult(await request(`/collections/${encodeURIComponent(input.table)}/records/${encodeURIComponent(input.record_id)}`, { method: 'DELETE' })); } }
    ];
  }

  async function getAgent() {
    if (!state.aiConfigured) throw new Error('企业尚未配置 AI Gateway，请联系管理员。');
    if (!supportsJspi()) throw new Error('当前浏览器不支持 agent 所需的 WebAssembly JSPI。请使用较新的 Chrome、Edge 或 Safari 后重试。');
    if (!state.fxAgent) {
      state.fxAgent = await createFxAgent({
        apiKey: 'miao-server-managed',
        wasm: '/vendor/fx/fx-core.wasm',
        checkpoint: state.fxPendingCheckpoint || undefined,
        instructions: `你是 MIAO 的工作协作 agent，帮助用户把真实工作从目标推进到完成。不要把自己描述成低代码/建表助手，也不要默认每个问题都要做应用或数据表。先理解目标、现状、约束和成功标准；复杂任务先提出清晰的步骤或方案，信息不足时只问最关键的问题。你可以梳理和改进流程、创建并切换工作工具、检查结构、查询和整理数据、录入或更新记录。只在确有需要且用户认可方案后才创建工具或结构。更新前确认目标记录与具体变更；删除属于破坏性操作，必须先说清对象与后果并取得明确确认。设计业务界面时先读取当前数据表和字段，向用户展示确切的标题、数据表和字段清单；只在用户认可方案后调用 create_ui_draft。用户要求修改现有草稿时，先调用 list_ui_versions 和 get_ui_version 读取并展示当前草稿，再说明拟修改的标题、数据表和字段变化；用户认可修改方案后调用 revise_ui_draft，基于草稿创建新的修订版本，不修改旧草稿或当前已发布界面。用户明确要求恢复历史界面时，先调用 list_ui_versions 和 get_ui_version 确认目标是已发布过的历史版本，并展示目标与当前正式版的标题、数据表和字段差异；说明恢复只复制界面配置，不恢复或回滚业务记录。只有用户明确要求恢复后才调用 restore_ui_version；服务端会按当前数据表和字段校验，若失败须说明原因且不会创建草稿。恢复成功会另存为新的前向草稿，当前正式界面和旧版本不变，工具会立即尝试展示真实只读预览。修订或恢复预览失败时保留草稿、说明实际错误并提供重试，不发布。v1 界面定义仅支持当前应用内真实存在的非附件字段和单数据表列表，不包含自定义表单布局或任意代码。兼容的已发布界面会自动提供基础新增和编辑表单，但只有界面字段包含所有可支持的必填字段时才开放；记录写入仍由服务端检查当前应用权限和数据校验。没有兼容表单时可通过数据检查页操作，不能宣称该应用界面支持相应操作。每次首次创建草稿后也要立即调用 preview_ui_version，说明这不是发布，不会修改业务数据，并等待用户审阅具体预览。发布必须针对当前对话中刚展示的确切草稿版本；使用 get_ui_version 再次读取时重新预览，之后等待用户明确要求发布/上线才可调用 publish_ui_version。发布前读取版本列表，把当前发布版本 ID 原样传入；并发冲突或其他保存/发布错误时展示服务端返回的具体原因，保留原草稿与当前正式界面，不猜测成功，也不盲目重试或覆盖他人版本。绝不编造业务事实、执行结果或外部能力。先用 list_apps 理解可继续的工作，有明确对象后再用 activate_app。当前工具会随这些工具调用动态切换。仅访问当前用户有权限的工作区与工具。每次工具执行后说明实际结果与未完成项。`,
        tools: agentTools(),
        fetch(url, init) {
          const headers = new Headers(init.headers);
          headers.delete('authorization');
          headers.set('Authorization', `Bearer ${state.token}`);
          headers.set('X-Miao-Tenant-Id', state.tenant.id);
          if (state.app?.id) headers.set('X-Miao-App-Id', state.app.id);
          else headers.delete('X-Miao-App-Id');
          headers.set('X-Fx-Path', new URL(url).pathname);
          return fetch('/api/fx/gateway', { ...init, headers }).then((response) => {
            const nextToken = response.headers.get('X-PocketBase-Token');
            if (nextToken) {
              state.token = nextToken;
              localStorage.setItem(tokenKey, nextToken);
            }
            return response;
          });
        }
      });
      state.fxPendingCheckpoint = null;
    }
    $('#agent-status').textContent = '在线';
    $('#agent-status').className = 'badge badge-success';
    return state.fxAgent;
  }

  async function submitPrompt(event) {
    event.preventDefault();
    if (state.fxBusy) return;
    const composer = event.currentTarget;
    const prompt = new FormData(composer).get('prompt').toString().trim();
    if (!prompt) return;
    state.fxBusy = true;
    let response = null;
    for (const form of [$('#agent-form'), $('#home-agent-form')]) {
      form.querySelector('[name="prompt"]').disabled = true;
      form.querySelector('button[type="submit"]').disabled = true;
    }
    $('#workspace-switcher').disabled = true;
    try {
      if (composer.id === 'home-agent-form') {
        state.workspaceView = 'assistant';
        await enterConversation();
        await renderWorkspace();
      } else {
        await refreshAuthorization();
        if (state.fxConversationLoadedKey !== conversationKey()) await enterConversation();
      }
      composer.reset();
      appendChat(prompt, 'user');
      response = appendChat('', 'assistant');
      $('#agent-status').textContent = '思考中';
      $('#agent-status').className = 'badge badge-info';
      const agent = await getAgent();
      state.fxTurnNumber = (state.fxTurnNumber || 0) + 1;
      const turn = agent.prompt(prompt);
      for await (const event of turn) {
        if (event.type === 'text_delta') response.textContent += event.delta;
        if (event.type === 'tool_start') {
          const note = document.createElement('small');
          note.className = 'tool-note';
          const labels = { list_apps: '正在查看已有工具', create_app: '正在创建工具', activate_app: '正在切换工作上下文', list_tables: '正在了解现有结构', list_ui_versions: '正在读取界面版本', get_ui_version: '正在读取目标草稿', create_ui_draft: '正在保存界面草稿', revise_ui_draft: '正在保存草稿修订', restore_ui_version: '正在校验并预览历史界面草稿', preview_ui_version: '正在生成界面只读预览', publish_ui_version: '正在发布已确认的界面', create_table: '正在建立工作所需结构', list_records: '正在查找相关信息', add_record: '正在新增记录', update_record: '正在更新记录', delete_record: '正在删除记录' };
          note.textContent = labels[event.name] || '正在处理下一步';
          $('#chat-messages').append(note);
        }
        $('#chat-messages').scrollTop = $('#chat-messages').scrollHeight;
      }
      await turn.result;
      if (!response.textContent) response.textContent = '已完成。';
      state.fxConversationMessages = [...(state.fxConversationMessages || []), { role: 'user', content: prompt }, { role: 'assistant', content: response.textContent }];
      try {
        await persistConversation();
      } catch (error) {
        toast(`fx 已完成，但私人对话未能保存到此浏览器：${error.message || '本地存储不可用'}`, true);
      }
      $('#agent-status').textContent = '在线';
      $('#agent-status').className = 'badge badge-success';
      await renderWorkspace();
    } catch (error) {
      if (response) {
        response.textContent = error.message || 'Agent 暂时无法响应。';
        $('#agent-form [name="prompt"]').value = prompt;
      }
      else toast(error.message || 'Agent 暂时无法响应。', true);
      if ($('#agent-status')) {
        $('#agent-status').textContent = '连接失败';
        $('#agent-status').className = 'badge badge-error';
      }
    } finally {
      state.fxBusy = false;
      $('#workspace-switcher').disabled = false;
      for (const form of [$('#agent-form'), $('#home-agent-form')]) {
        form.querySelector('[name="prompt"]').disabled = false;
        form.querySelector('button[type="submit"]').disabled = false;
      }
    }
  }

  return { submitPrompt, enterConversation, clearSavedConversation, clearSavedConversations };
}
