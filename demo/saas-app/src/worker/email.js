// Email sender using nodemailer pointed at Mailpit SMTP.
import { createTransport } from "nodemailer";

const transporter = createTransport({
  host: process.env.SMTP_HOST || "localhost",
  port: parseInt(process.env.SMTP_PORT || "1025", 10),
  secure: false,
  tls: { rejectUnauthorized: false },
});

/**
 * Send a welcome email.
 * @param {{ to: string, name: string }} params
 */
export async function sendWelcomeEmail({ to, name }) {
  const info = await transporter.sendMail({
    from: process.env.SMTP_FROM || "noreply@demo.localcloud.dev",
    to,
    subject: `Welcome to LocalCloud Demo, ${name}!`,
    text: `Hi ${name},\n\nThanks for signing up to the LocalCloud demo app.\n\nThis email was captured by Mailpit — it never left your machine.\n\n— The LocalCloud Demo`,
    html: `<h1>Welcome, ${name}!</h1>
<p>Thanks for signing up to the LocalCloud demo app.</p>
<p><em>This email was captured by Mailpit — it never left your machine.</em></p>`,
  });

  console.log(`[email] sent welcome email to ${to}, messageId: ${info.messageId}`);
  return info;
}
