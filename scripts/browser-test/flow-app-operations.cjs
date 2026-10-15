const{CDP,BASE}=require('./cdp-lib.cjs');
(async()=>{const c=new CDP();await c.ready;const t=await c.createTarget(BASE);await c.attach(t);await c.wait(900);
if(await c.visible('#auth')){await c.fill('input[name=email]',process.env.TEST_EMAIL||'tester-1792017639590@example.com');await c.fill('input[name=password]',process.env.TEST_PASSWORD||'MiaoTest-12345678');await c.eval(`document.querySelector('form').requestSubmit()`);await c.wait(1800)}
if(!(await c.visible('#workspace'))){console.log('auth remains',await c.eval('document.body.innerText'));c.close();return}
await c.click('.workspace-switcher-trigger').catch(()=>{}); await c.wait(250);
if(await c.eval(`!![...document.querySelectorAll('button')].find(e=>e.textContent.trim()==='浏览器验收工作区')`)){await c.clickText('浏览器验收工作区'); await c.wait(900)}
await c.shot('50-app-ops-dashboard');await c.clickText('成交作战台').catch(()=>{});await c.wait(900);await c.shot('51-runtime-before-actions');
for(const text of ['联系人','跟进记录']){if(await c.eval(`!![...document.querySelectorAll('button')].find(e=>e.textContent.trim()==='${text}')`)){await c.clickText(text);await c.wait(400);await c.shot('52-runtime-'+text)}}
const selector='.runtime-search-form input, input[placeholder*="搜索"]';if(await c.eval(`!!document.querySelector('${selector}')`)){await c.fill(selector,'test');await c.wait(500);await c.shot('53-runtime-search')}
await c.clickText('应用配置').catch(()=>{});await c.wait(700);await c.shot('54-app-settings-detail');await c.clickText('后台任务').catch(()=>{});await c.wait(500);if(await c.eval(`!![...document.querySelectorAll('button')].find(e=>e.textContent.trim()==='刷新')`)){await c.clickText('刷新');await c.wait(500)}await c.shot('55-tasks-refresh');
await c.click('[data-action="return-to-app"]').catch(()=>{});await c.wait(400);if(await c.eval(`!!document.querySelector('#agent-form textarea')`)){await c.fill('#agent-form textarea','请列出当前应用中的数据表，并说明每张表的用途');await c.shot('56-assistant-prompt-filled');await c.eval(`document.querySelector('#agent-form').requestSubmit()`);await c.wait(5000);await c.shot('57-assistant-response')}
c.close()})().catch(e=>{console.error('FAIL',e.stack||e);process.exit(1)})