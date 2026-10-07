import { t, fmtDateTime } from '/modules/i18n.js';

const labels = () => ({
  queued: t('排队中'), draft: t('草稿'), enabled: t('已启用'), paused: t('已暂停'), running: t('处理中'),
  completed: t('已完成'), partial: t('部分完成'), failed: t('失败'),
  created: t('已新增'), changed: t('已更新'), skipped: t('已跳过'),
  would_created: t('预计新增'), would_changed: t('预计更新'), error: t('错误')
});

export function createCollectionResults({ esc }) {
  const time = (value) => value && Number.isFinite(new Date(value).getTime()) ? fmtDateTime(value) : '—';
  const label = (value) => esc(labels()[value] || value || t('未知'));
  const value = (item) => esc((typeof item === 'object' ? JSON.stringify(item) : String(item ?? '—')).slice(0, 2000));

  function summary(script, members) {
    const source = script.source || script.definition?.source || {};
    const sourceUrl = source.url || t('未配置来源 URL');
    const schedule = script.schedule || script.definition?.schedule || {};
    const recipients = (script.recipients || script.definition?.recipients || []).map((id) => {
      const member = members.find((row) => row.id === id || row.user_id === id);
      return member?.name || member?.email || id;
    });
    const plan = schedule.type === 'manual' ? t('手动运行') : t('{type} · {timezone}', { type: schedule.type || t('未配置'), timezone: schedule.timezone || '' });
    return `<div class="collection-summary"><p>${esc(t('来源：{source} · 目标表：{table}', { source: sourceUrl, table: script.target?.table || script.definition?.target?.table || '—' }))}</p><p>${esc(t('计划：{plan} · 下次运行：{next}', { plan, next: time(script.next_run_at) }))}</p><p>${esc(t('接收人：{recipients} · 首次基线：{baseline}', { recipients: recipients.join(t('、')) || t('未配置'), baseline: script.definition?.baseline === 'silent' ? t('静默入库') : t('新增时通知') }))}</p>${script.pause_reason ? `<p>${esc(script.pause_reason)}</p>` : ''}</div>`;
  }

  function run(result) {
    const counts = result.counts || result.result?.counts || {};
    const sample = (result.result?.items || []).slice(0, 25);
    const preview = result.mode === 'preview';
    const source = result.source || result.snapshot?.source || {};
    const errors = [...(Array.isArray(result.errors) ? result.errors : []), ...(result.error ? [result.error] : [])];
    const stats = [['pages', t('页')], ['requests', t('请求')], ['items', t('来源记录')], ['filtered', t('筛选后')], ['created', preview ? t('预计新增') : t('新增')], ['changed', preview ? t('预计更新') : t('更新')], ['written', t('写入')], ['skipped', t('跳过')], ['notifications', t('待投递通知回执')], ['errors', t('记录错误')]];
    const delivery = result.delivery || {};
    const deliveryErrors = Array.isArray(delivery.errors) ? delivery.errors : [];
    const retry = !preview && (Number(delivery.failed || 0) + Number(delivery.blocked || 0)) > 0 && result.id ? `<button type="button" class="btn btn-warning btn-sm" data-settings-action="retry-script-notifications" data-run="${esc(result.id)}">${esc(t('仅重试通知'))}</button>` : '';
    return `<section class="collection-result" aria-live="polite"><p><strong>${preview ? esc(t('只读试运行')) : esc(t('采集运行'))}</strong> <span class="badge badge-ghost">${label(result.status)}</span> · v${esc(result.version || result.snapshot?.version || '—')}</p>
      <p class="settings-muted">${esc(source.url || t('来源 URL 未记录'))} · ${time(result.started_at || result.created)}${result.finished_at ? t(' 至 {time}', { time: time(result.finished_at) }) : ''}${typeof result.result?.baseline === 'boolean' ? ` · ${result.result.baseline ? t('首次基线') : t('后续运行')}` : ''}</p>
      <p class="collection-counts">${stats.map(([key, text]) => `<span>${text} <b>${esc(counts[key] ?? 0)}</b></span>`).join('')}</p>
      ${!preview ? `<p class="settings-muted">${esc(t('通知投递：待处理 {pending} · 已投递 {delivered} · 失败 {failed} · 无权限 {blocked}', { pending: delivery.pending || 0, delivered: delivery.delivered || 0, failed: delivery.failed || 0, blocked: delivery.blocked || 0 }))} ${retry}</p>` : ''}
      ${errors.length ? `<ul class="collection-errors">${[...new Set(errors)].map((error) => `<li>${esc(String(error))}</li>`).join('')}</ul>` : ''}
      ${deliveryErrors.length ? `<ul class="collection-errors">${deliveryErrors.map((item) => `<li>${esc(t('通知 {id}：{status}', { id: item.id, status: item.error || item.status }))}</li>`).join('')}</ul>` : ''}
      ${sample.length ? `<div class="collection-table overflow-x-auto"><table class="table table-sm"><caption>${esc(t('结果样本（最多 25 条）'))}</caption><thead><tr><th scope="col">${esc(t('结果'))}</th><th scope="col">${esc(t('记录或来源键'))}</th><th scope="col">${esc(t('数据或说明'))}</th></tr></thead><tbody>${sample.map((item) => `<tr><td>${label(item.status)}</td><td>${value(item.record_id || item.dedup_key)}</td><td>${value(item.error || item.data || '—')}${item.extract === 'list+detail' ? `<br>${esc(t('字段含详情页提取'))}` : ''}${item.notify ? `<br>${esc(t('正式运行将通知接收人'))}` : ''}</td></tr>`).join('')}</tbody></table></div>` : `<p class="settings-muted">${esc(t('本次没有可展示的记录样本。'))}</p>`}
      ${result.id ? `<p class="settings-muted">${esc(t('运行编号：{id}', { id: result.id }))}</p>` : ''}</section>`;
  }

  function history(scriptId, result) {
    const items = result.items || [];
    const page = Number(result.page) || 1, pages = Math.max(1, Number(result.totalPages) || 1);
    return `<div class="collection-table overflow-x-auto"><table class="table table-sm"><caption>${esc(t('采集运行记录'))}</caption><thead><tr><th scope="col">${esc(t('时间 / 版本'))}</th><th scope="col">${esc(t('状态'))}</th><th scope="col">${esc(t('写入 / 通知'))}</th><th scope="col">${esc(t('回执'))}</th></tr></thead><tbody>${items.map((item) => `<tr><td>${time(item.started_at || item.created)} · v${esc(item.version)}</td><td>${label(item.status)}</td><td>${esc(item.counts?.written ?? 0)} / ${esc(item.counts?.notifications ?? 0)}</td><td><button type="button" class="btn btn-ghost btn-sm" data-settings-action="view-script-run" data-id="${esc(scriptId)}" data-run="${esc(item.id)}">${esc(t('查看结果'))}</button></td></tr>`).join('') || `<tr><td colspan="4">${esc(t('暂无运行记录。'))}</td></tr>`}</tbody></table></div>
      <div class="settings-actions"><button type="button" class="btn btn-ghost btn-sm" data-settings-action="runs-script" data-id="${esc(scriptId)}" data-page="${page - 1}" ${page <= 1 ? 'disabled' : ''}>${esc(t('上一页'))}</button><span>${page} / ${pages}</span><button type="button" class="btn btn-ghost btn-sm" data-settings-action="runs-script" data-id="${esc(scriptId)}" data-page="${page + 1}" ${page >= pages ? 'disabled' : ''}>${esc(t('下一页'))}</button></div>`;
  }
  return { summary, run, history };
}
