import { t } from '/modules/i18n.js';

export function createNotifyChannels({ state, api, $, esc, toast, renderWorkspace }) {
  let generation = 0;

  async function open() {
    state.workspaceView = 'management';
    state.workspaceManagementPage = 'settings';
    state.app = null;
    state.table = null;
    await renderWorkspace();
    await load(true);
    $('#workspace-notify-channels').scrollIntoView({ block: 'start', behavior: 'smooth' });
  }

  async function load(reset = false) {
    const tenant = state.tenant?.id;
    const current = ++generation;
    const root = $('#workspace-notify-channels');
    root.classList.remove('hidden');
    root.textContent = t('正在读取通知渠道…');
    try {
      const config = await api('/api/workspace/notify-channels');
      if (tenant !== state.tenant?.id || current !== generation) return;
      root.innerHTML = `<h2>${esc(t('通知渠道'))}</h2><p class="admin-page-description">${esc(t('自动化触发站内通知的同时，向以下外部渠道转发提醒；外部发送失败不影响站内通知。'))}</p>
        <form data-notify-channels-form class="notify-channels-form">
          <label>${esc(t('通知邮箱'))}<textarea class="textarea" name="emails" rows="2" placeholder="${esc(t('每行一个邮箱，最多 20 个'))}" ${config.can_manage ? '' : 'disabled'}>${esc((config.emails || []).join('\n'))}</textarea></label>
          <label>企业微信 Webhook<input class="input" name="wecom_webhook" type="url" placeholder="https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=…" value="${esc(config.wecom_webhook || '')}" ${config.can_manage ? '' : 'disabled'} /></label>
          <label>钉钉 Webhook<input class="input" name="dingtalk_webhook" type="url" placeholder="https://oapi.dingtalk.com/robot/send?access_token=…" value="${esc(config.dingtalk_webhook || '')}" ${config.can_manage ? '' : 'disabled'} /></label>
          <label>飞书 Webhook<input class="input" name="feishu_webhook" type="url" placeholder="https://open.feishu.cn/open-apis/bot/v2/hook/…" value="${esc(config.feishu_webhook || '')}" ${config.can_manage ? '' : 'disabled'} /></label>
          ${config.can_manage ? `<div class="task-buttons"><button class="btn btn-primary btn-sm" type="submit">${esc(t('保存通知渠道'))}</button><button class="btn btn-outline btn-sm" type="button" data-notify-test>${esc(t('发送测试通知'))}</button></div>` : `<p class="settings-muted">${esc(t('只有工作区所有者可以修改通知渠道。'))}</p>`}
        </form>
        <div data-notify-channels-result aria-live="polite"></div>`;
    } catch (error) {
      if (tenant !== state.tenant?.id || current !== generation) return;
      root.innerHTML = `<div class="alert alert-error">${esc(error.message)}</div><button class="btn btn-sm" data-notify-retry>${esc(t('重新读取'))}</button>`;
    }
  }

  function bind() {
    const root = $('#workspace-notify-channels');
    root.addEventListener('click', (event) => {
      if (event.target.closest('[data-notify-retry]')) { load(); return; }
      if (!event.target.closest('[data-notify-test]')) return;
      const button = event.target.closest('[data-notify-test]');
      const result = $('[data-notify-channels-result]');
      button.disabled = true;
      if (result) result.textContent = t('正在发送测试通知…');
      api('/api/workspace/notify-channels/test', { method: 'POST', body: JSON.stringify({}) }).then((outcome) => {
        if (!result?.isConnected) return;
        if (outcome.ok) result.innerHTML = `<p class="settings-muted">${esc(t('测试通知已发送到所有已配置渠道。'))}</p>`;
        else result.innerHTML = `<div class="alert alert-error">${esc(t('部分渠道发送失败：'))}<code>${esc(outcome.failures.join('；'))}</code></div>`;
      }).catch((error) => { if (result?.isConnected) result.innerHTML = `<div class="alert alert-error">${esc(error.message)}</div>`; })
        .finally(() => { if (button.isConnected) button.disabled = false; });
    });
    root.addEventListener('submit', async (event) => {
      event.preventDefault();
      const form = event.target;
      if (!form.matches('[data-notify-channels-form]')) return;
      const button = form.querySelector('[type="submit"]');
      button.disabled = true;
      try {
        const values = Object.fromEntries(new FormData(form));
        const body = {
          emails: values.emails.split('\n').map((email) => email.trim()).filter(Boolean),
          wecom_webhook: values.wecom_webhook.trim(),
          dingtalk_webhook: values.dingtalk_webhook.trim(),
          feishu_webhook: values.feishu_webhook.trim()
        };
        await api('/api/workspace/notify-channels', { method: 'PUT', body: JSON.stringify(body) });
        toast(t('通知渠道已保存'));
        await load();
      } catch (error) { toast(error.message, true); button.disabled = false; }
    });
  }

  function reset() { generation++; $('#workspace-notify-channels').classList.add('hidden'); $('#workspace-notify-channels').replaceChildren(); }
  return { open, bind, reset };
}
