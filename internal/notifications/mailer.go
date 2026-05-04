// Package notifications provides email sending via SMTP (gomail).
package notifications

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"go.uber.org/zap"
	gomail "gopkg.in/gomail.v2"

	"github.com/inheritance-choir/backend/internal/config"
)

// Mailer sends HTML emails using gomail + SMTP.
type Mailer struct {
	dialer *gomail.Dialer
	from   string
	name   string
	cfg    *config.Config
	log    *zap.Logger
}

// New creates a Mailer. In dev mode, emails are only logged.
func New(cfg *config.Config, log *zap.Logger) *Mailer {
	d := gomail.NewDialer(cfg.MailHost, cfg.MailPort, cfg.MailUser, cfg.MailPassword)
	return &Mailer{
		dialer: d,
		from:   cfg.MailFrom,
		name:   cfg.MailFromName,
		cfg:    cfg,
		log:    log,
	}
}

func (m *Mailer) send(to, subject, htmlBody string) {
	if !m.cfg.EnableNotifications {
		m.log.Info("[DEV EMAIL - NOTIFICATIONS DISABLED]",
			zap.String("to", to),
			zap.String("subject", subject),
		)
		return
	}

	msg := gomail.NewMessage()
	msg.SetAddressHeader("From", m.from, m.name)
	msg.SetHeader("To", to)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/html", htmlBody)

	// Retry loop for transient network errors (up to 3 attempts)
	var err error
	for i := 0; i < 3; i++ {
		if err = m.dialer.DialAndSend(msg); err == nil {
			m.log.Info("Email sent", zap.String("to", to), zap.String("subject", subject), zap.Int("attempt", i+1))
			return
		}
		m.log.Warn("Email attempt failed", zap.Int("attempt", i+1), zap.Error(err))
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	m.log.Error("Email send failed after retries", zap.String("to", to), zap.Error(err))
}

// Gold-themed base HTML template
const baseHTML = `<!DOCTYPE html><html><head><meta charset="UTF-8">
<title>{{.Subject}}</title></head>
<body style="margin:0;padding:0;background:#0F172A;font-family:Arial,sans-serif;">
<div style="max-width:560px;margin:0 auto;padding:32px 16px;">
  <div style="background:linear-gradient(135deg,#141E33,#1E2D4A);border-radius:16px 16px 0 0;
              padding:24px 28px;border-bottom:2px solid #C9A84C;">
    <h1 style="margin:0;font-size:20px;color:#C9A84C;">🎵 INHERITANCE CHOIR</h1>
    <p style="margin:6px 0 0;color:#94A3B8;font-size:12px;font-style:italic;">
      Voices united in worship and excellence
    </p>
  </div>
  <div style="background:#141E33;padding:28px;border-radius:0 0 16px 16px;border:1px solid #1E2D4A;border-top:none;">
    {{.Body}}
    <div style="margin-top:32px;padding-top:20px;border-top:1px solid #1E2D4A;font-size:11px;color:#475569;text-align:center;">
      <p style="margin:0;">INHERITANCE CHOIR · Kigali, Rwanda</p>
    </div>
  </div>
</div></body></html>`

func (m *Mailer) render(subject, bodyHTML string) string {
	t := template.Must(template.New("email").Parse(baseHTML))
	var buf bytes.Buffer
	t.Execute(&buf, map[string]template.HTML{
		"Subject": template.HTML(subject),
		"Body":    template.HTML(bodyHTML),
	})
	return buf.String()
}

// SendOTP sends a 6-digit OTP verification email.
func (m *Mailer) SendOTP(to, code, purpose string) {
	subject := "Inheritance Choir — Verification Code"
	if purpose == "reset" { subject = "Inheritance Choir — Password Reset Code" }

	body := fmt.Sprintf(`
<h2 style="color:#F0F4FF;margin:0 0 12px;">Email Verification</h2>
<p style="color:#94A3B8;margin:0 0 20px;">Your one-time code:</p>
<div style="background:#0F172A;border:1px solid rgba(201,168,76,0.4);border-radius:12px;
            padding:24px;text-align:center;margin:0 0 24px;">
  <span style="font-size:36px;font-weight:bold;letter-spacing:12px;color:#C9A84C;">%s</span>
</div>
<p style="color:#64748B;font-size:13px;">Expires in 5 minutes.</p>`, code)

	m.send(to, subject, m.render(subject, body))
}

// SendWelcomeEmail sends a welcome email to a newly registered member.
func (m *Mailer) SendWelcomeEmail(to, fullName string) {
	subject := "Welcome to Inheritance Choir! 🎵"
	firstName := firstWord(fullName)
	body := fmt.Sprintf(`
<h2 style="color:#C9A84C;margin:0 0 16px;">Welcome, %s! 🎉</h2>
<p style="color:#CBD5E1;line-height:1.6;">
  Your registration is pending admin approval. You'll be notified once activated.
</p>`, firstName)
	m.send(to, subject, m.render(subject, body))
}

// SendApprovalEmail notifies a member that their account was approved.
func (m *Mailer) SendApprovalEmail(to, fullName string) {
	subject := "Your Inheritance Choir Account is Approved! 🎉"
	firstName := firstWord(fullName)
	body := fmt.Sprintf(`
<h2 style="color:#22C55E;margin:0 0 16px;">Account Approved! ✅</h2>
<p style="color:#CBD5E1;line-height:1.6;">
  Dear <strong style="color:#F0F4FF;">%s</strong>,<br><br>
  Your account has been approved. You can now log in and join the choir family.
</p>`, firstName)
	m.send(to, subject, m.render(subject, body))
}

// SendMonthlyTitheReminder sends a contribution reminder.
func (m *Mailer) SendMonthlyTitheReminder(to, fullName, month string) {
	subject := "💰 Monthly Contribution Reminder — " + m.cfg.AppName
	firstName := firstWord(fullName)
	body := fmt.Sprintf(`
<h2 style="color:#C9A84C;margin:0 0 16px;">Monthly Contribution Reminder</h2>
<p style="color:#CBD5E1;line-height:1.6;">
  Dear <strong style="color:#F0F4FF;">%s</strong>,<br><br>
  Your contribution for <strong style="color:#F0F4FF;">%s</strong> has not been recorded yet.
  Please contribute via the choir portal or contact the Treasurer.
</p>`, firstName, month)
	m.send(to, subject, m.render(subject, body))
}

// SendAttendanceAlert sends a personal at-risk attendance alert.
func (m *Mailer) SendAttendanceAlert(to, fullName string, rate float64) {
	subject := "⚠️ Attendance Alert — " + m.cfg.AppName
	firstName := firstWord(fullName)
	body := fmt.Sprintf(`
<h2 style="color:#F59E0B;margin:0 0 16px;">⚠️ Attendance Alert</h2>
<p style="color:#CBD5E1;line-height:1.6;">
  Dear <strong style="color:#F0F4FF;">%s</strong>,<br><br>
  Your attendance is <strong style="color:#EF4444;font-size:18px;">%.1f%%</strong> —
  below the target of <strong style="color:#C9A84C;">%.0f%%</strong>.
  Please make every effort to attend rehearsals and services.
</p>`, firstName, rate, m.cfg.AttendanceThreshold)
	m.send(to, subject, m.render(subject, body))
}

// SendEventReminder sends a 24h-before mandatory event reminder.
func (m *Mailer) SendEventReminder(to, fullName, eventTitle, eventDate, eventTime, location string) {
	subject := "📅 Reminder: " + eventTitle + " Tomorrow"
	firstName := firstWord(fullName)
	body := fmt.Sprintf(`
<h2 style="color:#3B82F6;margin:0 0 16px;">📅 Event Reminder</h2>
<p style="color:#CBD5E1;margin:0 0 20px;">
  Dear <strong style="color:#F0F4FF;">%s</strong>, you have a mandatory event tomorrow:
</p>
<table style="width:100%%;border-collapse:collapse;background:#0F172A;border-radius:12px;overflow:hidden;">
  <tr><td style="padding:10px 16px;color:#64748B;width:100px;">Event</td>
      <td style="padding:10px 16px;color:#F0F4FF;font-weight:bold;">%s</td></tr>
  <tr><td style="padding:10px 16px;color:#64748B;">Date</td>
      <td style="padding:10px 16px;color:#F0F4FF;">%s</td></tr>
  <tr><td style="padding:10px 16px;color:#64748B;">Time</td>
      <td style="padding:10px 16px;color:#C9A84C;font-weight:bold;">%s</td></tr>
  <tr><td style="padding:10px 16px;color:#64748B;">Location</td>
      <td style="padding:10px 16px;color:#F0F4FF;">%s</td></tr>
</table>`, firstName, eventTitle, eventDate, eventTime, location)
	m.send(to, subject, m.render(subject, body))
}

// SendBirthdayGreeting sends a birthday email.
func (m *Mailer) SendBirthdayGreeting(to, fullName string) {
	subject := "🎂 Happy Birthday from Inheritance Choir!"
	firstName := firstWord(fullName)
	body := fmt.Sprintf(`
<div style="text-align:center;padding:20px 0;">
  <div style="font-size:48px;">🎂🎉🎵</div>
  <h2 style="color:#C9A84C;">Happy Birthday, %s!</h2>
  <p style="color:#CBD5E1;line-height:1.6;">
    The entire Inheritance Choir family wishes you a wonderful birthday<br>
    filled with joy, love, and beautiful music! 🎶
  </p>
</div>`, firstName)
	m.send(to, subject, m.render(subject, body))
}

// SendChoirDigest sends the weekly digest to admins.
func (m *Mailer) SendChoirDigest(to, fullName string, stats map[string]interface{}) {
	subject := fmt.Sprintf("📊 %s — Weekly Choir Digest", m.cfg.AppName)
	body := fmt.Sprintf(`
<h2 style="color:#C9A84C;margin:0 0 16px;">📊 Weekly Choir Digest</h2>
<p style="color:#64748B;font-size:12px;margin:0 0 20px;">%s</p>
<table style="width:100%%;border-collapse:collapse;">
  <tr style="background:#0F172A;">
    <td style="padding:10px;color:#64748B;border:1px solid #1E2D4A;">Active Members</td>
    <td style="padding:10px;color:#C9A84C;font-weight:bold;border:1px solid #1E2D4A;">%v</td>
  </tr>
  <tr>
    <td style="padding:10px;color:#64748B;border:1px solid #1E2D4A;">Upcoming Events</td>
    <td style="padding:10px;color:#F0F4FF;border:1px solid #1E2D4A;">%v</td>
  </tr>
  <tr style="background:#0F172A;">
    <td style="padding:10px;color:#64748B;border:1px solid #1E2D4A;">At-Risk Members</td>
    <td style="padding:10px;color:#EF4444;border:1px solid #1E2D4A;">%v</td>
  </tr>
</table>`,
		time.Now().Format("January 02, 2006"),
		stats["activeMembers"], stats["upcomingEvents"], stats["atRiskCount"],
	)
	m.send(to, subject, m.render(subject, body))
}

// SendFinanceReport notifies admins that the monthly report is available.
func (m *Mailer) SendFinanceReport(to, fullName, month string, count int) {
	subject := fmt.Sprintf("📊 Finance Report — %s", month)
	body := fmt.Sprintf(`
<h2 style="color:#C9A84C;margin:0 0 16px;">Monthly Finance Report</h2>
<p style="color:#CBD5E1;">The finance report for <strong style="color:#F0F4FF;">%s</strong>
is ready. <strong>%d</strong> contribution records were processed this month.
Log in to the choir portal to view the full analytics dashboard.</p>`, month, count)
	m.send(to, subject, m.render(subject, body))
}

func firstWord(s string) string {
	for i, r := range s {
		if r == ' ' { return s[:i] }
	}
	return s
}

// SendGeneric sends a fully custom email with any subject and HTML body.
func (m *Mailer) SendGeneric(to, fullName, subject, bodyHTML string) {
	m.send(to, subject, m.render(subject, bodyHTML))
}

