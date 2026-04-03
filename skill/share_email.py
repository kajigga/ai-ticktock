#!/usr/bin/env python3
# share_email.py — Create a draft email sharing the time-entry skill with coworkers.
#
# Usage: python3 share_email.py [SCREENSHOT_PATH]
#   SCREENSHOT_PATH  Optional path to a PNG image to embed as a preview
#                    (e.g. a screenshot of what the weekly report email looks like).
#
# Saves directly to the Drafts folder of the ServiceNow (Exchange) mailbox in
# Mail.app via AppleScript, with sender set to kevin.hansen@servicenow.com.

import base64, os, subprocess, sys
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText

SENDER = 'kevin.hansen@servicenow.com'

# ── Color palette (ServiceNow brand) ─────────────────────────────────────────
SN_DARK   = '#293E40'
SN_GREEN  = '#62D84E'
SN_LIGHT  = '#EDF5EE'
SN_BORDER = '#C8DCC9'

# ── Optional screenshot embed ─────────────────────────────────────────────────
screenshot_path = sys.argv[1] if len(sys.argv) > 1 else None
img_section = ''
if screenshot_path and os.path.exists(screenshot_path):
    with open(screenshot_path, 'rb') as f:
        img_b64 = base64.b64encode(f.read()).decode()
    img_section = f"""
<p style="font-size:10px;font-weight:bold;letter-spacing:1.5px;color:{SN_DARK};margin:24px 0 8px">EXAMPLE: WEEKLY REPORT EMAIL</p>
<p style="color:#555;font-size:13px;margin:0 0 10px">The <code>/time-entry email this week</code> command drafts a formatted report like this:</p>
<img src="data:image/png;base64,{img_b64}" style="width:100%;max-width:600px;border:1px solid {SN_BORDER};border-radius:4px;display:block" alt="Example weekly report email">"""

# ── Feature table ─────────────────────────────────────────────────────────────
features = [
    ("Log time with natural language",               "log 4h GXO today — Oracle schema work"),
    ("View weekly summaries &amp; daily breakdowns", "show this week"),
    ("Export to a formatted Excel timesheet",        "export this week to spreadsheet"),
    ("Draft your weekly time report email",          "email this week"),
    ("Pull entries from your work calendar",         "pull from calendar"),
]

feature_rows = ''
for i, (label, cmd) in enumerate(features):
    bg = SN_LIGHT if i % 2 == 0 else '#ffffff'
    feature_rows += (
        f'<tr style="background:{bg}">'
        f'<td style="padding:8px 14px;border-bottom:1px solid {SN_BORDER};color:{SN_DARK}">{label}</td>'
        f'<td style="padding:8px 14px;border-bottom:1px solid {SN_BORDER};font-family:Courier New,monospace;font-size:12px;color:#555;white-space:nowrap">/time-entry {cmd}</td>'
        f'</tr>'
    )

# ── HTML body ─────────────────────────────────────────────────────────────────
html = f"""<html><body style="font-family:Calibri,Arial,sans-serif;font-size:14px;color:#222;max-width:620px">
<p>Hi team,</p>
<p>I've been using a time-tracking skill for <strong>Claude Code</strong> (and opencode) that might be useful for others on the team. It lets you log, view, and report on time entries directly in your AI coding sessions — no switching apps, no manual spreadsheet work.</p>

<table cellpadding="0" cellspacing="0" style="border-collapse:collapse;background:{SN_DARK};margin:20px 0;width:100%">
  <tr>
    <td style="padding:14px 20px;color:white">
      <div style="font-size:10px;letter-spacing:1.5px;opacity:.65;margin-bottom:4px">WHAT IT DOES</div>
      <div style="font-size:15px;font-weight:bold;color:{SN_GREEN}">Time tracking inside your AI assistant</div>
    </td>
  </tr>
</table>

<table cellpadding="0" cellspacing="0" style="border-collapse:collapse;width:100%;border:1px solid {SN_BORDER};margin-bottom:24px">
  <tr style="background:{SN_DARK};color:white">
    <th style="padding:8px 14px;text-align:left;font-size:12px;font-weight:600;letter-spacing:.5px">Feature</th>
    <th style="padding:8px 14px;text-align:left;font-size:12px;font-weight:600;letter-spacing:.5px">Command</th>
  </tr>
  {feature_rows}
</table>

<p style="font-size:10px;font-weight:bold;letter-spacing:1.5px;color:{SN_DARK};margin-bottom:6px">INSTALL</p>
<div style="background:{SN_LIGHT};border-left:4px solid {SN_GREEN};border-radius:0 4px 4px 0;padding:12px 16px;margin-bottom:20px">
  <div style="font-family:Courier New,monospace;font-size:13px;color:{SN_DARK}">curl -fsSL https://raw.githubusercontent.com/kajigga/ai-ticktock/main/install.sh | bash</div>
</div>

<p>Works with <strong>Claude Code</strong> and <strong>opencode</strong>. After installing, open either app and type <span style="font-family:Courier New,monospace;font-size:13px;background:{SN_LIGHT};padding:1px 6px;border-radius:3px">/time-entry</span> to get started. First-time setup takes about a minute.</p>
{img_section}
<p style="margin-top:20px">Happy to help if you run into any issues getting it set up.</p>
<p>Kevin</p>
</body></html>"""

# ── Write HTML to temp file (avoids shell-escaping the full body) ─────────────
html_path = '/tmp/share_email_body.html'
with open(html_path, 'w', encoding='utf-8') as f:
    f.write(html)

# ── Build .eml and open as compose window ────────────────────────────────────
# Opening a .eml with X-Unsent:1 renders HTML properly in Mail's compose window.
# Mail matches the From address to the Exchange account, so pressing ⌘S will
# save the draft to the ServiceNow mailbox's Drafts folder.
subject = 'AI-powered time tracking skill for Claude Code / opencode'

msg = MIMEMultipart('alternative')
msg['Subject']  = subject
msg['From']     = SENDER
msg['To']       = SENDER
msg['X-Unsent'] = '1'
msg.attach(MIMEText(html, 'html', 'utf-8'))

eml_path = '/tmp/share_email_draft.eml'
with open(eml_path, 'w', encoding='utf-8') as f:
    f.write(msg.as_string())

subprocess.run(['open', '-a', 'Mail', eml_path])
print(f'Draft opened in Mail — add recipients and press ⌘S to save to {SENDER} Drafts.')
