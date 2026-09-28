const apiKey = () => process.env.RESEND_API_KEY;
const sender = () => process.env.MIAO_MAIL_FROM;
export const isMailConfigured = () => Boolean(apiKey() && sender());

export const sendMail = async ({ to, subject, html }) => {
  if (!isMailConfigured()) return false;
  const response = await fetch('https://api.resend.com/emails', {
    method: 'POST',
    headers: { Authorization: `Bearer ${apiKey()}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ from: sender(), to: [to], subject, html })
  });
  if (!response.ok) throw new Error(`邮件服务返回 ${response.status}`);
  return true;
};

export const publicUrl = (path) => new URL(path, process.env.MIAO_PUBLIC_URL || 'http://localhost:41874').toString();
