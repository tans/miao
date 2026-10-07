const labels = {
  draft: '草稿', enabled: '已启用', paused: '已暂停', running: '处理中',
  completed: '已完成', partial: '部分完成', failed: '失败',
  created: '已新增', changed: '已更新', skipped: '已跳过',
  would_created: '预计新增', would_changed: '预计更新', error: '错误'
};

export function createCollectionResults({ esc }) {
  const time = (value) => value && Number.isFinite(new Date(value).getTime()) ? new Date(value).toLocaleString() : '—';
  const label = (value) => esc(labels[value] || value || '未知');
  const value = (item) => esc((typeof item === 'object' ? JSON.stringify(item) : String(item ?? '—')).slice(0, 2000));

  function summary(script, members) {
    const source = script.source || script.definition?.source || {};
    const sourceUrl = source.url || '未配置来源 URL';
    const schedule = script.schedule || script.definition?.schedule || {};
    const recipients = (script.recipients || script.definition?.recipients || []).map((id) => {
      const member = members.find((row) => row.id === id || row.user_id === id);
      return member?.name || member?.email || id;
    });
    const plan = schedule.type === 'manual' ? '手动运行' : `${schedule.type || '未配置'} · ${schedule.timezone || ''}`;
    return `<div class="collection-summary"><p>来源：${esc(sourceUrl} · 目标表：${esc(script.target?.table || script.definition?.target?.table || '—')}</p><p>计划：${esc(plan)} · 下次运行：${time(script.next_run_at)}</p><p>接收人：${esc(recipients.join('、') || '未配置')} · 首次基线：${script.definition?.baseline === 'silent' ? '静默入库' : '新增时通知'}</p>${script.pause_reason ? `<p>${esc(script.pause_reason)}</p>` : ''}</div>`;
  }

  function run(result) {
    const counts = result.counts || result.result?.counts || {};
    const sample = (result.result?.items || []).slice(0, 25);
    const preview = result.mode === 'preview';
    const source = result.source || result.snapshot?.source || {};
    const errors = [...(Array.isArray(result.errors) ? result.errors : []), ...(result.error ? [result.error] : [])];
    const stats = [['pages', '页'], ['requests', '请求'], ['items', '来源记录'], ['filtered', '筛选后'], ['created', preview ? '预计新增' : '新增'], ['changed', preview ? '预计更新' : '更新'], ['written', '写入'], ['skipped', '跳过'], ['notifications', '已投递通知'], ['errors', '记录错误']];
    return `<section class="collection-result" aria-live="polite"><p><strong>${preview ? '只读试运行' : '采集运行'}</strong> <span class="badge badge-ghost">${label(result.status)}</span> · v${esc(result.version || result.snapshot?.version || '—')}</p>
      <p class="settings-muted">${esc(source.url || '来源 URL 未记录')} · ${time(result.started_at || result.created)}${result.finished_at ? ` 至 ${time(result.finished_at)}` : ''}${typeof result.result?.baseline === 'boolean' ? ` · ${result.result.baseline ? '首次基线' : '后续运行'}` : ''}</p>
      <p class="collection-counts">${stats.map(([key, text]) => `<span>${text} <b>${esc(counts[key] ?? 0)}</b></span>`).join('')}</p>
      ${errors.length ? `<ul class="collection-errors">${[...new Set(errors)].map((error) => `<li>${esc(String(error))}</li>`).join('')}</ul>` : ''}
      ${sample.length ? `<div class="collection-table overflow-x-auto"><table class="table table-sm"><caption>结果样本（最多 25 条）</caption><thead><tr><th scope="col">结果</th><th scope="col">记录或来源键</th><th scope="col">数据或说明</th></tr></thead><tbody>${sample.map((item) => `<tr><td>${label(item.status)}</td><td>${value(item.record_id || item.dedup_key)}</td><td>${value(item.error || item.data || '—')}${item.notify ? '<br>正式运行将通知接收人' : ''}</td></tr>`).join('')}</tbody></table></div>` : '<p class="settings-muted">本次没有可展示的记录样本。</p>'}
      ${result.id ? `<p class="settings-muted">运行编号：${esc(result.id)}</p>` : ''}</section>`;
  }

  function history(scriptId, result) {
    const items = result.items || [];
    const page = Number(result.page) || 1, pages = Math.max(1, Number(result.totalPages) || 1);
    return `<div class="collection-table overflow-x-auto"><table class="table table-sm"><caption>采集运行记录</caption><thead><tr><th scope="col">时间 / 版本</th><th scope="col">状态</th><th scope="col">写入 / 通知</th><th scope="col">回执</th></tr></thead><tbody>${items.map((item) => `<tr><td>${time(item.started_at || item.created)} · v${esc(item.version)}</td><td>${label(item.status)}</td><td>${esc(item.counts?.written ?? 0)} / ${esc(item.counts?.notifications ?? 0)}</td><td><button type="button" class="btn btn-ghost btn-sm" data-settings-action="view-script-run" data-id="${esc(scriptId)}" data-run="${esc(item.id)}">查看结果</button></td></tr>`).join('') || '<tr><td colspan="4">暂无运行记录。</td></tr>'}</tbody></table></div>
      <div class="settings-actions"><button type="button" class="btn btn-ghost btn-sm" data-settings-action="runs-script" data-id="${esc(scriptId)}" data-page="${page - 1}" ${page <= 1 ? 'disabled' : ''}>上一页</button><span>${page} / ${pages}</span><button type="button" class="btn btn-ghost btn-sm" data-settings-action="runs-script" data-id="${esc(scriptId)}" data-page="${page + 1}" ${page >= pages ? 'disabled' : ''}>下一页</button></div>`;
  }
  return { summary, run, history };
}
