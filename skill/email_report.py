#!/usr/bin/env python3
# email_report.py — Build and open the weekly time report email draft.
#
# Usage: python3 email_report.py WEEK_START BOSS_EMAIL SUMMARY
#   WEEK_START  Monday date of the target week, e.g. 2026-03-30
#   BOSS_EMAIL  Recipient address
#   SUMMARY     Claude-generated narrative paragraph (quoted string)
#
# Reads ~/.config/timetracker.json for data_file, mail_app, and sender_email.
# Opens a draft in Microsoft Outlook or Mail.app depending on mail_app config.

import json, os, subprocess, sys
from datetime import datetime, timedelta
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText

# ── Args ─────────────────────────────────────────────────────────────────────
if len(sys.argv) < 4:
    print(f"Usage: {sys.argv[0]} WEEK_START BOSS_EMAIL SUMMARY", file=sys.stderr)
    sys.exit(1)

week_start_str = sys.argv[1]
boss_email     = sys.argv[2]
summary        = sys.argv[3]

# ── Config & data ─────────────────────────────────────────────────────────────
config_path = os.path.expanduser('~/.config/timetracker.json')
with open(config_path) as f:
    config = json.load(f)
with open(config['data_file']) as f:
    entries = json.load(f)

week_entries = [e for e in entries if e.get('week_start') == week_start_str]

# ── Aggregate ─────────────────────────────────────────────────────────────────
by_type    = {}
by_project = {}
by_day     = {}
for e in week_entries:
    by_type[e['work_type']] = by_type.get(e['work_type'], 0) + e['hours']
    by_project[e['project']] = by_project.get(e['project'], 0) + e['hours']
    by_day.setdefault(e['date'], {})
    by_day[e['date']][e['project']] = by_day[e['date']].get(e['project'], 0) + e['hours']

total   = sum(by_type.values())
billable = by_type.get('Billable', 0)
nonbill  = by_type.get('Non-Billable', 0)
pto      = by_type.get('PTO', 0)

week_start_dt = datetime.strptime(week_start_str, '%Y-%m-%d')
week_end_dt   = week_start_dt + timedelta(days=4)
days      = [week_start_dt + timedelta(days=i) for i in range(5)]
day_names = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday']

# ── Color palette (ServiceNow brand) ─────────────────────────────────────────
SN_DARK   = '#293E40'
SN_GREEN  = '#62D84E'
SN_LIGHT  = '#EDF5EE'
SN_BORDER = '#C8DCC9'

def fmt(h):
    # Format hours: drop trailing .0 for whole numbers
    return f'{h:.1f}' if h != int(h) else str(int(h))

# ── Project table rows ────────────────────────────────────────────────────────
proj_rows = ''
for i, (proj, hrs) in enumerate(sorted(by_project.items(), key=lambda x: -x[1])):
    bg = SN_LIGHT if i % 2 == 0 else '#ffffff'
    proj_rows += (
        f'<tr style="background:{bg}">'
        f'<td style="padding:7px 14px;border-bottom:1px solid {SN_BORDER}">{proj}</td>'
        f'<td style="padding:7px 14px;border-bottom:1px solid {SN_BORDER};text-align:right">'
        f'<b style="color:{SN_DARK}">{fmt(hrs)}</b></td></tr>'
    )

def project_pills(projects):
    # Small colored badges showing project + hours for a single day cell
    rows = ''
    for p, h in sorted(projects.items(), key=lambda x: -x[1]):
        rows += (
            f'<div style="display:flex;justify-content:space-between;align-items:center;'
            f'padding:2px 7px;margin-bottom:3px;background:{SN_LIGHT};'
            f'border-left:3px solid {SN_GREEN};border-radius:0 4px 4px 0;font-size:12px">'
            f'<span style="color:#2a3a2b">{p}</span>'
            f'<span style="font-weight:bold;color:{SN_DARK};margin-left:12px">{fmt(h)}</span>'
            f'</div>'
        )
    return rows

# ── Daily breakdown rows ──────────────────────────────────────────────────────
day_rows = ''
for d, name in zip(days, day_names):
    d_str     = d.strftime('%Y-%m-%d')
    projects  = by_day.get(d_str, {})
    day_total = sum(projects.values())
    if projects:
        day_rows += (
            f'<tr><td style="padding:8px 14px;border-bottom:1px solid {SN_BORDER};vertical-align:top">'
            f'<b>{name}</b><br><span style="color:#888;font-size:12px">{d.strftime("%b %-d")}</span></td>'
            f'<td style="padding:8px 14px;border-bottom:1px solid {SN_BORDER};vertical-align:top;'
            f'text-align:right;white-space:nowrap"><b style="color:{SN_DARK}">{fmt(day_total)}</b></td>'
            f'<td style="padding:8px 14px;border-bottom:1px solid {SN_BORDER};vertical-align:top">'
            f'{project_pills(projects)}</td></tr>'
        )
    else:
        day_rows += (
            f'<tr style="color:#bbb"><td style="padding:8px 14px;border-bottom:1px solid {SN_BORDER}">'
            f'{name}<br><span style="font-size:12px">{d.strftime("%b %-d")}</span></td>'
            f'<td style="padding:8px 14px;border-bottom:1px solid {SN_BORDER};text-align:right">—</td>'
            f'<td></td></tr>'
        )

# ── HTML body ─────────────────────────────────────────────────────────────────
week_label = f"{week_start_dt.strftime('%b %-d')} \u2013 {week_end_dt.strftime('%b %-d, %Y')}"
subject    = f"Weekly Time Report \u2014 Week of {week_start_dt.strftime('%b %-d, %Y')}"

html = f"""<html><body style="font-family:Calibri,Arial,sans-serif;font-size:14px;color:#222;max-width:600px">
<p>Hi,</p>
<p>Here's my time report for the week of <b>{week_label}</b>.</p>
<div style="background:{SN_LIGHT};border-left:4px solid {SN_GREEN};border-radius:0 4px 4px 0;padding:12px 16px;color:#2a3a2b;line-height:1.6;font-size:13px;margin-bottom:24px">
  {summary}
</div>
<table cellpadding="0" cellspacing="0" style="border-collapse:collapse;background:{SN_DARK};margin-bottom:20px;width:100%">
  <tr>
    <td style="padding:14px 20px;color:white;text-align:center"><div style="font-size:10px;letter-spacing:1.5px;opacity:.65">TOTAL</div><div style="font-size:26px;font-weight:bold;color:{SN_GREEN}">{fmt(total)}</div></td>
    <td style="padding:14px 20px;color:white;text-align:center;border-left:1px solid rgba(255,255,255,.15)"><div style="font-size:10px;letter-spacing:1.5px;opacity:.65">BILLABLE</div><div style="font-size:26px;font-weight:bold">{fmt(billable)}</div></td>
    <td style="padding:14px 20px;color:white;text-align:center;border-left:1px solid rgba(255,255,255,.15)"><div style="font-size:10px;letter-spacing:1.5px;opacity:.65">NON-BILLABLE</div><div style="font-size:26px;font-weight:bold">{fmt(nonbill)}</div></td>
    <td style="padding:14px 20px;color:white;text-align:center;border-left:1px solid rgba(255,255,255,.15)"><div style="font-size:10px;letter-spacing:1.5px;opacity:.65">PTO</div><div style="font-size:26px;font-weight:bold">{fmt(pto)}</div></td>
  </tr>
</table>
<p style="font-size:10px;font-weight:bold;letter-spacing:1.5px;color:{SN_DARK};margin-bottom:4px">BY PROJECT</p>
<table cellpadding="0" cellspacing="0" style="border-collapse:collapse;width:100%;border:1px solid {SN_BORDER};margin-bottom:20px">
  <tr style="background:{SN_DARK};color:white">
    <th style="padding:8px 14px;text-align:left;font-size:12px;font-weight:600;letter-spacing:.5px">Project</th>
    <th style="padding:8px 14px;text-align:right;font-size:12px;font-weight:600;letter-spacing:.5px">Hours</th>
  </tr>{proj_rows}
</table>
<p style="font-size:10px;font-weight:bold;letter-spacing:1.5px;color:{SN_DARK};margin-bottom:4px">DAILY BREAKDOWN</p>
<table cellpadding="0" cellspacing="0" style="border-collapse:collapse;width:100%;border:1px solid {SN_BORDER};margin-bottom:24px">
  <tr style="background:{SN_DARK};color:white">
    <th style="padding:8px 14px;text-align:left;font-size:12px;font-weight:600;letter-spacing:.5px;width:105px">Day</th>
    <th style="padding:8px 14px;text-align:right;font-size:12px;font-weight:600;letter-spacing:.5px;width:65px">Hours</th>
    <th style="padding:8px 14px;text-align:left;font-size:12px;font-weight:600;letter-spacing:.5px">Projects</th>
  </tr>{day_rows}
</table>
<p>Thanks,<br>Kevin</p>
</body></html>"""

# ── Open draft in mail app ────────────────────────────────────────────────────
mail_app  = config.get('mail_app', 'Mail')
html_path = '/tmp/weekly_report_draft_body.html'
with open(html_path, 'w', encoding='utf-8') as f:
    f.write(html)

if mail_app == 'Microsoft Outlook':
    # Outlook AppleScript: read HTML from file to avoid shell-escaping issues.
    # 'content' is Outlook's HTML body property per its AppleScript dictionary.
    safe_subject = subject.replace('\\', '\\\\').replace('"', '\\"')
    safe_email   = boss_email.replace('"', '\\"')
    script = f'''set htmlContent to do shell script "cat {html_path}"
tell application "Microsoft Outlook"
    activate
    set theMsg to make new outgoing message with properties {{subject:"{safe_subject}", content:htmlContent}}
    make new to recipient at theMsg with properties {{email address:{{address:"{safe_email}"}}}}
    open theMsg
end tell
'''
    script_path = '/tmp/weekly_report_draft.applescript'
    with open(script_path, 'w') as f:
        f.write(script)
    subprocess.run(['osascript', script_path])
else:
    # Mail.app: write .eml with X-Unsent so it opens as a new compose window
    msg = MIMEMultipart('alternative')
    msg['Subject']  = subject
    msg['From']     = config.get('sender_email', '')
    msg['To']       = boss_email
    msg['X-Unsent'] = '1'
    msg.attach(MIMEText(html, 'html', 'utf-8'))
    eml_path = '/tmp/weekly_report_draft.eml'
    with open(eml_path, 'w') as f:
        f.write(msg.as_string())
    subprocess.run(['open', '-a', 'Mail', eml_path])

print(f'Draft opened in {mail_app}.')
