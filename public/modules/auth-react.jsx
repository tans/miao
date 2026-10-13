import React, { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { t, setLanguage } from './i18n.js';

// React entry for the authentication surface (Phase 2 of the daisyUI ->
// shadcn/ui migration). Owns login/register/verify-email/reset-password.
// app.js detects this flag and skips its tokenless auth wiring; login/logout
// reload across the boundary so the vanilla workspace keeps its session flow.
window.__MIAO_REACT_AUTH__ = true;

const TOKEN_KEY = 'miao_token';
const LAST_LOGIN_KEY = 'miao_last_login';
const pendingInvite = new URLSearchParams(location.search).get('invite');
const verifyingEmail = location.pathname === '/verify-email';
const resettingPassword = location.pathname === '/reset-password';

async function api(url, options = {}) {
  const headers = { ...(options.body ? { 'Content-Type': 'application/json' } : {}), ...(options.headers || {}) };
  const response = await fetch(url, { ...options, headers });
  const result = await response.json().catch(() => ({}));
  if (!response.ok) {
    const error = new Error(result.error || result.message || t('请求失败，请稍后重试'));
    error.status = response.status;
    throw error;
  }
  return result;
}

function toast(message, error = false) {
  const node = document.querySelector('#toast');
  if (!node) return;
  node.innerHTML = `<div class="alert ${error ? 'alert-error' : 'alert-success'}"><span>${String(message ?? '').replace(/[&<>"']/g, (char) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#039;' }[char]))}</span></div>`;
  setTimeout(() => { node.replaceChildren(); }, 3000);
}

const inputCls = 'h-[42px] w-full rounded-md border border-input bg-background px-3.5 text-[13px] leading-[20px] text-foreground placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:opacity-50';
const labelCls = 'block text-[13px] font-medium leading-[20px] text-[#454a50]';
const buttonCls = (variant) => {
  const base = 'inline-flex h-[42px] w-full items-center justify-center rounded-md px-4 text-[13px] font-medium leading-[20px] focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50';
  if (variant === 'ghost') return `${base} border border-transparent bg-transparent text-foreground hover:bg-accent`;
  return `${base} border border-transparent bg-primary text-primary-foreground hover:bg-[#303438]`;
};

function Field({ label, children }) {
  return <label className="grid gap-[6px]"><span className={labelCls}>{label}</span>{children}</label>;
}

function AuthForm({ registering }) {
  const [busy, setBusy] = useState(false);
  const submit = async (event) => {
    event.preventDefault();
    if (busy) return;
    const form = new FormData(event.currentTarget);
    const path = registering ? '/api/auth/register' : '/api/auth/login';
    const payload = Object.fromEntries(form);
    if (registering && pendingInvite) payload.invite_token = pendingInvite;
    setBusy(true);
    try {
      const result = await api(path, { method: 'POST', body: JSON.stringify(payload) });
      if (result.requires_verification) {
        toast(t('验证邮件已发送到 {email}', { email: result.email }));
        event.currentTarget.reset();
        return;
      }
      localStorage.setItem(TOKEN_KEY, result.token);
      try { localStorage.setItem(LAST_LOGIN_KEY, JSON.stringify({ email: payload.email, password: payload.password })); } catch {}
      location.reload();
    } catch (error) {
      toast(error.message, true);
    } finally {
      setBusy(false);
    }
  };
  return <form className="grid gap-4" onSubmit={submit} aria-busy={busy}>
    {registering && <Field label={t('姓名')}><input className={inputCls} name="name" autoComplete="name" required placeholder={t('怎么称呼你')} /></Field>}
    <Field label={t('邮箱')}><input className={inputCls} name="email" type="email" autoComplete="email" required placeholder="name@company.com" /></Field>
    <Field label={t('密码')}><input className={inputCls} name="password" type="password" autoComplete={registering ? 'new-password' : 'current-password'} minLength={8} required placeholder={t('至少 8 位')} /></Field>
    <button className={buttonCls('primary')} type="submit" disabled={busy}>{registering ? t('创建账号') : t('登录')}</button>
  </form>;
}

function LangSwitch() {
  return <div className="lang-switch" role="group" aria-label="Language">
    <button className="btn btn-link btn-xs" type="button" data-lang="zh-CN" onClick={() => setLanguage('zh-CN')}>中文</button>
    <button className="btn btn-link btn-xs" type="button" data-lang="en" onClick={() => setLanguage('en')}>English</button>
  </div>;
}

function AuthCard({ onMode, onForgot, mode }) {
  return <div className="auth-card">
    <a className="brand" href="/"><img src="/mascots/cat-logo-head.png" alt="" /><strong>miao.my</strong></a>
    <h1 className="text-foreground">{mode === 'register' ? t('创建工作区') : t('登录')}</h1>
    {pendingInvite && <p className="text-muted-foreground">{t('你收到了工作区邀请。使用受邀邮箱登录或注册，即可直接加入。')}</p>}
    <AuthForm registering={mode === 'register'} />
    <p className="auth-switch"><span>{mode === 'register' ? t('已有账号？') : t('还没有账号？')}</span><button className="btn btn-link" type="button" onClick={() => onMode(mode === 'register' ? 'login' : 'register')}>{mode === 'register' ? t('登录') : t('创建账号')}</button></p>
    {mode === 'login' && <button className="btn btn-link btn-sm" type="button" onClick={onForgot}>{t('忘记密码？')}</button>}
    <LangSwitch />
  </div>;
}

function ForgotForm({ onBack }) {
  const [busy, setBusy] = useState(false);
  const submit = async (event) => {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    try {
      const email = new FormData(event.currentTarget).get('email');
      const result = await api('/api/auth/password-reset/request', { method: 'POST', body: JSON.stringify({ email }) });
      toast(result.message || t('如果邮箱已登记，重置邮件将发送到邮箱'));
      onBack();
    } catch (error) {
      toast(error.message, true);
    } finally {
      setBusy(false);
    }
  };
  return <div className="auth-card">
    <a className="brand" href="/"><img src="/mascots/cat-logo-head.png" alt="" /><strong>miao.my</strong></a>
    <h1 className="text-foreground">{t('忘记密码？')}</h1>
    <form className="grid gap-4" onSubmit={submit} aria-busy={busy}>
      <Field label={t('邮箱')}><input className={inputCls} type="email" name="email" autoComplete="email" required /></Field>
      <div className="flex gap-2"><button className={`${buttonCls('ghost')} flex-1`} type="button" onClick={onBack} disabled={busy}>{t('返回登录')}</button><button className={`${buttonCls('primary')} flex-1`} type="submit" disabled={busy}>{t('发送重置邮件')}</button></div>
    </form>
    <LangSwitch />
  </div>;
}

function ResetForm() {
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState(false);
  const submit = async (event) => {
    event.preventDefault();
    if (busy) return;
    setBusy(true);
    const password = new FormData(event.currentTarget).get('password');
    const token = new URLSearchParams(location.search).get('token');
    try {
      await api('/api/auth/password-reset/confirm', { method: 'POST', body: JSON.stringify({ token, password }) });
      try {
        const saved = JSON.parse(localStorage.getItem(LAST_LOGIN_KEY) || 'null');
        if (saved?.email) localStorage.setItem(LAST_LOGIN_KEY, JSON.stringify({ email: saved.email, password }));
      } catch {}
      history.replaceState({}, '', '/');
      toast(t('密码已更新，请使用新密码登录'));
      setDone(true);
    } catch (error) {
      toast(error.message, true);
    } finally {
      setBusy(false);
    }
  };
  if (done) return <AuthApp />;
  return <div className="auth-card">
    <a className="brand" href="/"><img src="/mascots/cat-logo-head.png" alt="" /><strong>miao.my</strong></a>
    <h1 className="text-foreground">{t('重置密码')}</h1>
    <form className="grid gap-4" onSubmit={submit} aria-busy={busy}>
      <Field label={t('新密码')}><input className={inputCls} type="password" name="password" minLength={8} required /></Field>
      <button className={buttonCls('primary')} type="submit" disabled={busy}>{t('更新密码')}</button>
    </form>
  </div>;
}

function AuthApp() {
  const [mode, setMode] = useState('login');
  const [view, setView] = useState('form');
  useEffect(() => {
    if (view !== 'form') return;
    const email = document.querySelector('#auth-root [name="email"]');
    const password = document.querySelector('#auth-root [name="password"]');
    if (mode === 'register') {
      if (password) password.value = '';
      return;
    }
    let saved = null;
    try { saved = JSON.parse(localStorage.getItem(LAST_LOGIN_KEY) || 'null'); } catch {}
    if (saved?.email) {
      if (email && !email.value) email.value = saved.email || '';
      if (password && !password.value) password.value = saved.password || '';
    }
  }, [mode, view]);
  return <>
    {view === 'form' && <AuthCard mode={mode} onMode={setMode} onForgot={() => setView('forgot')} />}
    {view === 'forgot' && <ForgotForm onBack={() => setView('form')} />}
  </>;
}

function VerifyEmail() {
  const [busy, setBusy] = useState(true);
  useEffect(() => {
    let cancelled = false;
    api('/api/auth/verify-email', { method: 'POST', body: JSON.stringify({ token: new URLSearchParams(location.search).get('token') }) })
      .then(() => {
        const invite = new URLSearchParams(location.search).get('invite');
        history.replaceState({}, '', invite ? `/?invite=${encodeURIComponent(invite)}` : '/');
        toast(t('邮箱验证完成，可以登录了'));
      })
      .catch((error) => toast(error.message, true))
      .finally(() => { if (!cancelled) setBusy(false); });
    return () => { cancelled = true; };
  }, []);
  if (!busy) return <AuthApp />;
  return <section className="auth-screen" aria-busy={busy}><div className="session-loading" role="status" aria-live="polite"><span className="session-loading-spinner" aria-hidden="true"></span><span>{t('正在恢复登录状态…')}</span></div></section>;
}

if (!localStorage.getItem(TOKEN_KEY)) {
  const root = document.createElement('div');
  root.id = 'auth-root';
  // Let .auth-card stay the direct grid child of .auth-screen (width: min(380px, 100%)).
  root.style.display = 'contents';
  const section = document.querySelector('#auth');
  if (section) {
    section.classList.remove('session-checking');
    section.setAttribute('aria-busy', 'false');
    section.replaceChildren(root);
  }
  createRoot(root).render(verifyingEmail ? <VerifyEmail /> : resettingPassword ? <ResetForm /> : <AuthApp />);
} else {
  // Token present: the vanilla workspace owns the page. Keep the static #auth
  // markup hidden until app.js takes over the show() switch.
  document.querySelector('#auth')?.classList.add('hidden');
}
