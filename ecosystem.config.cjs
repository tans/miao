const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');

function defaultInstallDir() {
  if (process.platform === 'darwin') return path.join(os.homedir(), 'Library', 'Application Support', 'Miao');
  return path.join(process.env.XDG_DATA_HOME || path.join(os.homedir(), '.local', 'share'), 'miao');
}

const installDir = process.env.MIAO_INSTALL_DIR || defaultInstallDir();
const configFile = process.env.MIAO_CONFIG_FILE || path.join(installDir, 'miao.env');

function readConfig(file) {
  if (!fs.existsSync(file)) return {};
  const result = {};
  for (const sourceLine of fs.readFileSync(file, 'utf8').split(/\r?\n/)) {
    const line = sourceLine.trim().replace(/^export\s+/, '');
    if (!line || line.startsWith('#')) continue;
    const separator = line.indexOf('=');
    if (separator < 1) continue;
    const key = line.slice(0, separator).trim();
    let value = line.slice(separator + 1).trim();
    if ((value.startsWith('"') && value.endsWith('"')) || (value.startsWith("'") && value.endsWith("'"))) {
      value = value.slice(1, -1);
    }
    result[key] = value;
  }
  return result;
}

const fileEnv = readConfig(configFile);
const setting = (name, fallback) => process.env[name] || fileEnv[name] || fallback;
const dataDir = setting('MIAO_DATA_DIR', process.platform === 'darwin'
  ? path.join(os.homedir(), 'Library', 'Application Support', 'Miao', 'data')
  : path.join(process.env.XDG_DATA_HOME || path.join(os.homedir(), '.local', 'share'), 'miao'));
const miaoPort = setting('MIAO_PORT', '41874');
const host = setting('HOST', '0.0.0.0');
const root = process.env.MIAO_ROOT || __dirname;
const miaoBinary = process.env.MIAO_BIN || path.join(installDir, 'bin', 'miao');

const commonEnv = {
  MIAO_DATA_DIR: dataDir,
};

module.exports = {
  apps: [
    {
      name: 'miao-platform',
      kill_timeout: 30000,
      script: path.join(root, 'scripts', 'run-miao.sh'),
      interpreter: 'none',
      cwd: root,
      args: [],
      env: {
        ...commonEnv,
        MIAO_BIN: miaoBinary,
        HOST: host,
        PORT: miaoPort,
        AI_GATEWAY_API_KEY: setting('AI_GATEWAY_API_KEY', ''),
        MIAO_JEV_API_KEY: setting('MIAO_JEV_API_KEY', ''),
        MIAO_JEV_MODEL: setting('MIAO_JEV_MODEL', 'typesafe-ai/jev'),
        MIAO_AI_PROVIDER: setting('MIAO_AI_PROVIDER', 'vercel'),
        MIAO_AI_BASE_URL: setting('MIAO_AI_BASE_URL', 'http://127.0.0.1:3210/api/v1'),
        MIAO_AI_MODEL: setting('MIAO_AI_MODEL', 'gpt-5.2'),
        MIAO_PUBLIC_URL: setting('MIAO_PUBLIC_URL', ''),
        MIAO_MAIL_FROM: setting('MIAO_MAIL_FROM', ''),
        RESEND_API_KEY: setting('RESEND_API_KEY', ''),
        MIAO_REQUIRE_EMAIL_VERIFICATION: setting('MIAO_REQUIRE_EMAIL_VERIFICATION', 'false'),
        MIAO_REGISTRATION_MODE: setting('MIAO_REGISTRATION_MODE', 'open'),
        MIAO_ALLOWED_EMAIL_DOMAINS: setting('MIAO_ALLOWED_EMAIL_DOMAINS', ''),
        MIAO_ADMIN_EMAILS: setting('MIAO_ADMIN_EMAILS', ''),
        MIAO_SETTINGS_ENCRYPTION_KEY: setting('MIAO_SETTINGS_ENCRYPTION_KEY', ''),
      },
      out_file: path.join(dataDir, 'logs', 'miao-out.log'),
      error_file: path.join(dataDir, 'logs', 'miao-error.log'),
      watch: false,
      autorestart: true,
      restart_delay: 1000,
      max_restarts: 20,
    },
  ],
};
