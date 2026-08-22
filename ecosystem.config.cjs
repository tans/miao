const path = require('node:path');

module.exports = {
  apps: [
    {
      name: 'miaozao',
      script: './src/server.js',
      cwd: __dirname,
      instances: 1,
      exec_mode: 'fork',
      autorestart: true,
      watch: false,
      max_restarts: 10,
      restart_delay: 3000,
      time: true,
      env: {
        NODE_ENV: 'production',
        PORT: process.env.PORT || '41874',
        HOST: process.env.HOST || '0.0.0.0',
        MONGODB_URI: process.env.MONGODB_URI || 'mongodb://127.0.0.1:27017',
        MONGODB_DB: process.env.MONGODB_DB || 'agent_native_runtime'
      },
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      out_file: path.resolve(__dirname, 'logs/miaozao-out.log'),
      error_file: path.resolve(__dirname, 'logs/miaozao-error.log'),
      merge_logs: true
    }
  ]
};
