#!/usr/bin/env python3
# export.py — export a week's entries to the Excel timesheet
# Usage: uv run --with openpyxl python3 export.py WEEK_START
# WEEK_START must be a Monday in YYYY-MM-DD format. Defaults to current week.
# Example: uv run --with openpyxl python3 ~/.claude/skills/time-entry/export.py 2026-03-30

import json, os, sys
from datetime import datetime, timedelta
from collections import defaultdict
import openpyxl
from openpyxl.styles import Font, PatternFill, Alignment, Border, Side
from openpyxl.utils import get_column_letter

# ── Config ───────────────────────────────────────────────────────────────
config_path = os.path.expanduser('~/.config/timetracker.json')
with open(config_path) as f:
    config = json.load(f)

data_file = config['data_file']
xlsx_path = os.path.expanduser(config['spreadsheet_file'])
target_hours = 40

if len(sys.argv) >= 2:
    week_start_str = sys.argv[1]
else:
    today = datetime.today()
    week_start_str = (today - timedelta(days=today.weekday())).strftime('%Y-%m-%d')

# ── Load entries ──────────────────────────────────────────────────────────
with open(data_file) as f:
    all_entries = json.load(f)

week_entries = [e for e in all_entries if e.get('week_start') == week_start_str]
week_start   = datetime.strptime(week_start_str, '%Y-%m-%d')
week_end     = week_start + timedelta(days=6)

# ── Helpers ───────────────────────────────────────────────────────────────
BLUE_DARK  = '1F3864'
BLUE_MID   = '2E74B5'
BLUE_LIGHT = 'DEEAF1'
GREY_LIGHT = 'F5F5F5'
GREY_MID   = 'D9D9D9'
WHITE      = 'FFFFFF'
BLACK      = '000000'

def hdr_font(bold=True, color=WHITE, size=11):
    return Font(bold=bold, color=color, size=size, name='Calibri')

def body_font(bold=False, color=BLACK, size=10):
    return Font(bold=bold, color=color, size=size, name='Calibri')

def fill(hex_color):
    return PatternFill('solid', fgColor=hex_color)

def center():
    return Alignment(horizontal='center', vertical='center')

def left():
    return Alignment(horizontal='left', vertical='center')

def thin_border():
    s = Side(style='thin', color=GREY_MID)
    return Border(top=s, bottom=s, left=s, right=s)

def progress_bar(hours, target, width=20):
    filled = min(int((hours / target) * width), width)
    return chr(9608) * filled + chr(9617) * (width - filled)

# ── Compute summary ───────────────────────────────────────────────────────
by_type    = defaultdict(float)
by_project = defaultdict(float)
by_day     = defaultdict(lambda: defaultdict(float))

for e in week_entries:
    t = e.get('work_type', 'Billable')
    by_type[t]             += e['hours']
    by_project[e['project']] += e['hours']
    by_day[e['date']][t]   += e['hours']

total_hours = sum(by_type.values())
billable    = by_type.get('Billable', 0)
non_bill    = by_type.get('Non-Billable', 0)
pto         = by_type.get('PTO', 0)
utilization = billable / target_hours if target_hours else 0

days          = [(week_start + timedelta(days=i)) for i in range(5)]
day_names     = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday']
projects_sorted = sorted(by_project.items(), key=lambda x: -x[1])

# ── Open or create workbook ───────────────────────────────────────────────
if os.path.dirname(xlsx_path):
    os.makedirs(os.path.dirname(xlsx_path), exist_ok=True)

if os.path.exists(xlsx_path):
    wb = openpyxl.load_workbook(xlsx_path)
else:
    wb = openpyxl.Workbook()
    if 'Sheet' in wb.sheetnames:
        del wb['Sheet']

# ── Dashboard sheet ───────────────────────────────────────────────────────
if 'Dashboard' in wb.sheetnames:
    del wb['Dashboard']
ws = wb.create_sheet('Dashboard', 0)
ws.sheet_view.showGridLines = False
for col, w in zip('ABCDEF', [18, 14, 14, 14, 14, 14]):
    ws.column_dimensions[col].width = w

r = 1
ws.merge_cells('A{}:F{}'.format(r, r))
c = ws.cell(r, 1, 'Timesheet Dashboard')
c.font = Font(bold=True, size=16, color=WHITE, name='Calibri')
c.fill = fill(BLUE_DARK); c.alignment = left()
ws.row_dimensions[r].height = 36

r += 1
week_label = "Week of {} - {}".format(week_start.strftime('%b %-d'), week_end.strftime('%b %-d, %Y'))
generated  = "Generated: " + datetime.today().strftime('%b %-d, %Y')
ws.merge_cells('A{}:D{}'.format(r, r))
c = ws.cell(r, 1, week_label)
c.font = Font(bold=True, size=11, color=WHITE, name='Calibri')
c.fill = fill(BLUE_MID); c.alignment = left()
ws.cell(r, 5, generated).font = body_font(color='888888')
ws.row_dimensions[r].height = 22

r += 2
ws.merge_cells('A{}:F{}'.format(r, r))
c = ws.cell(r, 1, 'WEEK SUMMARY')
c.font = hdr_font(size=10); c.fill = fill(BLUE_MID); c.alignment = left()
ws.row_dimensions[r].height = 18
r += 1

summary_rows = [
    ('Total Hours',          '{:.1f}h'.format(total_hours),
     '{}  {:.0%} of {}h target'.format(progress_bar(total_hours, target_hours), total_hours/target_hours if target_hours else 0, target_hours)),
    ('Billable',             '{:.1f}h'.format(billable),    ''),
    ('Non-Billable',         '{:.1f}h'.format(non_bill),    ''),
    ('PTO',                  '{:.1f}h'.format(pto),         ''),
    ('Billable Utilization', '{:.0%}'.format(utilization),  ''),
]
for i, (label, value, note) in enumerate(summary_rows):
    bg   = GREY_LIGHT if i % 2 == 0 else WHITE
    bold = (i == 0)
    ws.cell(r, 1, label).font = body_font(bold=bold)
    ws.cell(r, 1).fill = fill(bg); ws.cell(r, 1).alignment = left()
    ws.cell(r, 2, value).font = body_font(bold=bold)
    ws.cell(r, 2).fill = fill(bg); ws.cell(r, 2).alignment = center()
    if note:
        ws.merge_cells('C{}:F{}'.format(r, r))
        c = ws.cell(r, 3, note)
        c.font = Font(name='Courier New', size=9, color='2E74B5')
        c.fill = fill(bg)
    for col in range(1, 7):
        ws.cell(r, col).border = thin_border()
    ws.row_dimensions[r].height = 18
    r += 1

r += 1
ws.merge_cells('A{}:F{}'.format(r, r))
c = ws.cell(r, 1, 'DAILY BREAKDOWN')
c.font = hdr_font(size=10); c.fill = fill(BLUE_MID); c.alignment = left()
ws.row_dimensions[r].height = 18
r += 1

for col, h in enumerate(['Day', 'Date', 'Billable', 'Non-Billable', 'PTO', 'Total'], 1):
    c = ws.cell(r, col, h)
    c.font = hdr_font(size=10, color=WHITE); c.fill = fill(BLUE_DARK)
    c.alignment = center(); c.border = thin_border()
ws.row_dimensions[r].height = 18
r += 1

for day, name in zip(days, day_names):
    d_str = day.strftime('%Y-%m-%d')
    b = by_day[d_str].get('Billable', 0)
    n = by_day[d_str].get('Non-Billable', 0)
    p = by_day[d_str].get('PTO', 0)
    t = b + n + p
    has = t > 0
    bg = BLUE_LIGHT if has else WHITE
    tc = BLACK if has else 'BBBBBB'
    vals = [name, day.strftime('%b %-d'),
            '{:.1f}h'.format(b) if b else '-',
            '{:.1f}h'.format(n) if n else '-',
            '{:.1f}h'.format(p) if p else '-',
            '{:.1f}h'.format(t) if t else '-']
    for col, val in enumerate(vals, 1):
        c = ws.cell(r, col, val)
        c.font = body_font(bold=(col == 6 and has), color=tc)
        c.fill = fill(bg)
        c.alignment = center() if col > 1 else left()
        c.border = thin_border()
    ws.row_dimensions[r].height = 17
    r += 1

total_b = sum(by_day[d.strftime('%Y-%m-%d')].get('Billable', 0) for d in days)
total_n = sum(by_day[d.strftime('%Y-%m-%d')].get('Non-Billable', 0) for d in days)
total_p = sum(by_day[d.strftime('%Y-%m-%d')].get('PTO', 0) for d in days)
for col, val in enumerate(['Total', '', '{:.1f}h'.format(total_b), '{:.1f}h'.format(total_n), '{:.1f}h'.format(total_p), '{:.1f}h'.format(total_b+total_n+total_p)], 1):
    c = ws.cell(r, col, val)
    c.font = body_font(bold=True); c.fill = fill(GREY_MID)
    c.alignment = center() if col > 1 else left(); c.border = thin_border()
ws.row_dimensions[r].height = 18
r += 2

ws.merge_cells('A{}:F{}'.format(r, r))
c = ws.cell(r, 1, 'PROJECT SUMMARY')
c.font = hdr_font(size=10); c.fill = fill(BLUE_MID); c.alignment = left()
ws.row_dimensions[r].height = 18
r += 1

for col, h in enumerate(['Project', 'Hours', '% of Total', 'Billable', 'Non-Billable', 'PTO'], 1):
    c = ws.cell(r, col, h)
    c.font = hdr_font(size=10, color=WHITE); c.fill = fill(BLUE_DARK)
    c.alignment = center() if col > 1 else left(); c.border = thin_border()
ws.row_dimensions[r].height = 18
r += 1

for i, (project, hours) in enumerate(projects_sorted):
    bg  = GREY_LIGHT if i % 2 == 0 else WHITE
    pct = hours / total_hours if total_hours else 0
    pe  = [e for e in week_entries if e['project'] == project]
    pb  = sum(e['hours'] for e in pe if e.get('work_type') == 'Billable')
    pn  = sum(e['hours'] for e in pe if e.get('work_type') == 'Non-Billable')
    pp  = sum(e['hours'] for e in pe if e.get('work_type') == 'PTO')
    for col, val in enumerate([project, '{:.1f}h'.format(hours), '{:.0%}'.format(pct),
                                '{:.1f}h'.format(pb) if pb else '-',
                                '{:.1f}h'.format(pn) if pn else '-',
                                '{:.1f}h'.format(pp) if pp else '-'], 1):
        c = ws.cell(r, col, val)
        c.font = body_font(); c.fill = fill(bg)
        c.alignment = center() if col > 1 else left(); c.border = thin_border()
    ws.row_dimensions[r].height = 17
    r += 1

for col, val in enumerate(['Total', '{:.1f}h'.format(total_hours), '100%', '', '', ''], 1):
    c = ws.cell(r, col, val)
    c.font = body_font(bold=True); c.fill = fill(GREY_MID)
    c.alignment = center() if col > 1 else left(); c.border = thin_border()
ws.row_dimensions[r].height = 18
r += 2

ws.merge_cells('A{}:F{}'.format(r, r))
c = ws.cell(r, 1, 'ENTRY DETAIL')
c.font = hdr_font(size=10); c.fill = fill(BLUE_MID); c.alignment = left()
ws.row_dimensions[r].height = 18
r += 1

for col, h in enumerate(['Date', 'Day', 'Project', 'Type', 'Hours', 'Notes'], 1):
    c = ws.cell(r, col, h)
    c.font = hdr_font(size=10, color=WHITE); c.fill = fill(BLUE_DARK)
    c.alignment = center() if col < 3 else left(); c.border = thin_border()
ws.row_dimensions[r].height = 18
r += 1

for i, e in enumerate(sorted(week_entries, key=lambda e: (e['date'], e['project']))):
    bg = GREY_LIGHT if i % 2 == 0 else WHITE
    d  = datetime.strptime(e['date'], '%Y-%m-%d')
    for col, val in enumerate([d.strftime('%b %-d'), d.strftime('%A'), e['project'],
                                e.get('work_type', 'Billable'),
                                '{:.1f}h'.format(e['hours']),
                                e.get('notes', '')], 1):
        c = ws.cell(r, col, val)
        c.font = body_font(); c.fill = fill(bg)
        c.alignment = center() if col <= 2 else left(); c.border = thin_border()
    ws.row_dimensions[r].height = 17
    r += 1

# ── Time Entries sheet ────────────────────────────────────────────────────
if 'Time Entries' in wb.sheetnames:
    del wb['Time Entries']
te = wb.create_sheet('Time Entries')
for col, w in zip('ABCDEF', [14, 14, 22, 14, 8, 50]):
    te.column_dimensions[get_column_letter(ord(col)-64)].width = w

te.cell(1, 1, 'Time Entries').font = Font(bold=True, size=13, name='Calibri')
for col, h in enumerate(['Date', 'Week Start', 'Account / Project', 'Work Type', 'Hours', 'Notes'], 1):
    c = te.cell(2, col, h)
    c.font = Font(bold=True, color=WHITE, name='Calibri')
    c.fill = fill(BLUE_DARK)
    c.alignment = Alignment(horizontal='center')

for i, e in enumerate(sorted(all_entries, key=lambda e: (e['date'], e['project'])), 3):
    te.cell(i, 1, e['date'])
    te.cell(i, 2, e.get('week_start', ''))
    te.cell(i, 3, e['project'])
    te.cell(i, 4, e.get('work_type', 'Billable'))
    te.cell(i, 5, e['hours'])
    te.cell(i, 6, e.get('notes', ''))

# ── Save ──────────────────────────────────────────────────────────────────
wb.save(xlsx_path)
print("Exported {} entries (week of {}) to {}".format(len(week_entries), week_start_str, xlsx_path))
