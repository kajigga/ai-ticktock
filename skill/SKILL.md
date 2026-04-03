---
name: time-entry
description: Add and view time entries using the timetracker CLI
---

## Invocation

Use `/time-entry` to explicitly invoke this skill. You can also trigger it with natural language, but using `/time-entry` is preferred — it guarantees the right workflow is used and you can pass your request directly as an argument:

```
/time-entry log 4h GXO today — Oracle work
/time-entry show this week
/time-entry email this week's report to my boss
/time-entry fix my last entry — it was 3h not 2
```

## Purpose
Add, view, report on, and export time entries via the `timetracker` Go binary at `~/.claude/skills/time-entry/timetracker`. No Python or jq required for core operations. Excel export uses a separate `export.py` (requires `uv`).

## Config & Data Files
- **Config**: `~/.config/timetracker.json`
  - `data_file`: path to the entries JSON file — **must be inside OneDrive**
  - `backup_file`: path to the daily .zip backup — **must also be inside OneDrive**
  - `accounts`: list of known project accounts
  - `spreadsheet_file`: path to the Excel timesheet (optional, set on first export)
  - `last_backup_date`: tracks when the last backup ran (managed automatically)
- **Entries file**: JSON array at the path in `data_file`

---

## Step 0: Verify Prerequisites & Config (ALWAYS do this first)

### Check binary

```bash
~/.claude/skills/time-entry/timetracker --version 2>&1 && echo "ok" || echo "missing"
```

If missing, tell the user:
> "The timetracker binary is missing. Rebuild it from source:
> `cd ~/.claude/skills/time-entry/go-src && go build -o ~/.claude/skills/time-entry/timetracker .`
> Requires Go 1.21+. See BUILD.md in that directory."

Do not proceed if the binary is unavailable.

### Check config

```bash
test -f ~/.config/timetracker.json && echo "exists" || echo "missing"
```

**If missing**, detect OneDrive and ask for paths:

```bash
ls -d ~/Library/CloudStorage/OneDrive-*/ 2>/dev/null | head -1
```

Use the detected path to suggest defaults, then ask:
> "Where should your time entries file be stored?
> Recommended (OneDrive): [DETECTED_ONEDRIVE_PATH]timetracker/entries.json
> Enter path or press enter to use recommended:"

> "Where should daily backups be saved?
> Recommended: [DETECTED_ONEDRIVE_PATH]timetracker/backups/entries-backup.zip"

**Important:** Both paths should be inside OneDrive so data is cloud-synced and protected.

Then create config and data file:

```bash
~/.claude/skills/time-entry/timetracker config init \
  --data-file "DATA_FILE_PATH" \
  --backup-file "BACKUP_FILE_PATH"
```

---

## Batch Entry Mode

If the user wants to log multiple entries at once (e.g. "log my whole week", "enter several entries", or provides a list), enter **batch mode**:

1. Parse all entries the user provided upfront (project, hours, date, notes)
2. For any entry with a missing or ambiguous field, resolve it before writing — ask once per ambiguity, not per entry
3. If any project names aren't in the accounts list, collect them all first and ask once: `"These projects aren't in your list: X, Y. Add them all?"`
4. Show a summary table of all pending entries and ask for confirmation before writing:

```
Ready to log:
  2026-03-31 | GXO        | Billable     | 4.0h — Oracle work
  2026-03-31 | FCB        | Billable     | 2.0h
  2026-04-01 | Arrow      | Billable     | 6.0h — infra review
Log all 3 entries? (yes / edit / cancel)
```

5. On confirmation, write all entries with a single `timetracker batch` call:
6. Print a confirmation line for each entry added
7. Run daily backup (see **Backup & Recovery** section)

### Batch Write (multiple entries at once)

Build the `new_entries` list from all confirmed entries, then run:

Pass a JSON array of entry objects:

```bash
~/.claude/skills/time-entry/timetracker batch '[
  {"date":"DATE1","project":"PROJECT1","work_type":"WORK_TYPE1","hours":HOURS1,"notes":"NOTES1"},
  {"date":"DATE2","project":"PROJECT2","work_type":"WORK_TYPE2","hours":HOURS2,"notes":"NOTES2"}
]'
```

---

## Adding a Single Time Entry

### 1. Quick-Entry Parsing
If the user provides partial info like "log 4h QTS today" or "8h GXO - bug fixes":
- Extract: hours, account, date (default: today), notes
- Ask for any missing required fields

### 2. Account/Project Selection (IMPORTANT)

Read the current accounts list with:

```bash
~/.claude/skills/time-entry/timetracker accounts list
```

**Present the list to the user and ask them to pick one.** Always include "Other (new project)" as the last option.

Example prompt:
> Which project?
> 1. GXO
> 2. FCB
> 3. IHG
> 4. Other (new project)

**If the user picks "Other" or types a name not in the list:**
1. Ask: `"Add '[name]' to your accounts list for future use?"`
2. If yes, add it to the config:

```bash
~/.claude/skills/time-entry/timetracker config add-account "NEW_ACCOUNT_NAME"
```

**If the accounts list is empty** (first use), skip the selection and just ask:
> "What project is this for?"
Then offer to save it: `"Save '[name]' to your accounts list?"`

### 3. Work Type
- Default: **Billable** — skip asking unless user said "non-billable" or "pto"
- Valid values: `Billable`, `Non-Billable`, `PTO`

### 4. Date
- Default: today
- Accept: YYYY-MM-DD, "today", natural language ("April 1")

### 5. Hours
Ask `"Hours:"` and let the user type directly (e.g. 4, 8, 1.5). Required, numeric.

### 6. Notes
Optional. Ask `"Notes: (optional)"` and allow the user to skip with enter.

### 7. Write the Entry

Use the pre-written script — replace DATE, PROJECT, WORK_TYPE, HOURS, NOTES with actual values:

```bash
~/.claude/skills/time-entry/timetracker add DATE PROJECT WORK_TYPE HOURS "NOTES"
```

Example:
```bash
~/.claude/skills/time-entry/timetracker add 2026-04-03 FCB Billable 0.5 "Planning"
```

### 8. Confirm
Show the output line to the user: `Added: [date] | [project] | [type] | [hours]h`

### 9. Run daily backup (after every write)

```bash
~/.claude/skills/time-entry/timetracker backup
```

---

## Amending / Editing Entries

Trigger phrases: "edit last entry", "fix my last entry", "change the last one", "edit first entry", "edit entry 2", "fix today's GXO entry", "amend", "update entry"

### Finding the entry to edit

Support these selectors:
- `last` — the most recently created entry (highest `created_at`, or last in array if missing)
- `first` — the oldest entry (lowest `created_at`, or first in array)
- `N` — Nth from the end (e.g. "edit entry 2" = second-to-last)
- project + date — e.g. "fix today's GXO entry" → filter by project and date, pick the most recent if multiple

**Step 1: Show the candidate entry and ask what to change.**

```bash
~/.claude/skills/time-entry/timetracker amend show last
# or: show first | show 2 | show ENTRY_ID
```

Show this to the user and ask: `"What would you like to change? (date / project / hours / notes / work_type)"`

**Step 2: Apply the change and write back.**

```bash
~/.claude/skills/time-entry/timetracker amend update ENTRY_ID field=value [field=value ...]
# Examples:
#   timetracker amend update abc123 hours=3.0
#   timetracker amend update abc123 notes="Oracle 9 schema work"
#   timetracker amend update abc123 hours=3.0 notes="Updated" project=GXO
```

**Step 3:** Confirm the change to the user, then run the daily backup.

### Example interactions

```
User: fix my last entry — it was 3 hours not 2
→ Updated: 2026-04-03 | Acrisure | Billable | 3.0h — Working on Palantir integration
  updated_at: 2026-04-03T15:42:10

User: edit today's GXO entry, notes should be "Oracle 9 schema work"
→ Updated: 2026-04-03 | GXO | Billable | 1.0h — Oracle 9 schema work
  updated_at: 2026-04-03T15:43:55
```

---

## Calendar & Email Import

Trigger phrases: "pull from calendar", "pull from calendar and email", "sync from calendar", "import from calendar", "check calendar for entries", "pull this week from calendar", "pull from calendar and email"

> **⚠️ Desktop requirement:** This feature requires **Calendar.app** and **Mail.app** installed and configured on macOS. Work accounts that will be queried are scoped to **veza.com** and **servicenow.com** domains. Personal calendars (rosariokevin.com, US Holidays, etc.) are automatically excluded.

---

### Step 1: Determine the week

Default: current week. Accept "this week", "last week", or a specific date. Compute `week_start` (Monday) and `week_end` (Friday) as YYYY-MM-DD strings.

---

### Step 2: Load existing entries for deduplication

```bash
~/.claude/skills/time-entry/timetracker list --week WEEK_START
```

Keep this list in mind for Step 4 deduplication.

---

### Step 3: Pull from Calendar.app

> **Performance note:** Uses a pre-compiled Swift + EventKit binary stored at `~/.claude/skills/time-entry/pull_calendar`. EventKit does a single bulk fetch from the CalendarAgent daemon (~0.4s for a full week) vs AppleScript (~65s). The source lives at `~/.claude/skills/time-entry/pull_calendar.swift` — edit and recompile there if you need changes.

**Check binary exists, compile if missing:**

```bash
test -x ~/.claude/skills/time-entry/pull_calendar && echo "binary_ok" || (
  echo "Compiling calendar binary..." &&
  swiftc ~/.claude/skills/time-entry/pull_calendar.swift \
         -o ~/.claude/skills/time-entry/pull_calendar &&
  echo "compiled_ok"
)
```

**Run it** — replace WEEK_START (Monday, e.g. 2026-03-30) and WEEK_END (Saturday, e.g. 2026-04-04):

```bash
~/.claude/skills/time-entry/pull_calendar WEEK_START WEEK_END 2>&1
```

Output format (one line per event): `calendarName|title|start|end|isAllDay`

On first run macOS will prompt for Calendar permission — grant it once and it persists.

**To rebuild the binary** (e.g. after a macOS update breaks it):
```bash
swiftc ~/.claude/skills/time-entry/pull_calendar.swift \
       -o ~/.claude/skills/time-entry/pull_calendar
```

**Filter rules — include only events that:**
- Come from a work calendar (calendar name contains "ServiceNow", "Veza", or account is veza.com / servicenow.com)
- Are NOT all-day events (start time is not midnight-to-midnight)
- Start between 06:00 and 20:00
- Have a non-zero duration
- Are NOT canceled (title does not start with "Canceled")
- Are NOT status-only/no-meeting (title contains "Status only" or "no meeting")
- Are NOT test events (title contains "[TEST]")

**Classify as customer events if the title:**
- Contains a known account name from the accounts list
- Matches pattern `CustomerName | Veza`, `CustomerName/Veza`, `CustomerName<>Veza`, `CustomerName – Veza`
- Contains "Veza" alongside another identifiable company name

**Exclude as internal/non-billable if the title matches:**
- Standup, sync, CS meeting, CSE/CSA, Expert Services, Architect Sync
- Email and Messaging Catchup, OAA dev time / OAA time (generic dev blocks)
- Lunch, personal events

**For each customer event**, identify the most likely account from the known accounts list. If the account can't be determined with confidence, flag it with `?` and ask the user.

---

### Step 4: Pull from Mail.app (if "and email" was requested)

> **Note:** Email-based entries are lower confidence than calendar entries. Always flag them separately and let the user decide. Emails show *communication* happened, not necessarily billable work time.

Use AppleScript to read emails from work accounts for the week:

```bash
osascript << 'APPLESCRIPT'
set startDate to date "WEEK_START_DATE"
set output to ""
tell application "Mail"
    repeat with acct in accounts
        set acctEmail to email addresses of acct
        -- Only query veza.com and servicenow.com accounts
        repeat with addr in acctEmail
            if addr contains "veza.com" or addr contains "servicenow.com" then
                set msgs to (messages of inbox of acct whose date received >= startDate)
                repeat with m in msgs
                    set output to output & (addr as string) & "|" & (subject of m) & "|" & ((date received of m) as string) & "|" & (sender of m) & "
"
                end repeat
            end if
        end repeat
    end repeat
end tell
return output
APPLESCRIPT
```

**From the email results:**
- Group emails by apparent customer/project (subject line keywords, sender domain)
- Estimate time only for substantive threads (multiple back-and-forth, detailed technical content)
- Suggest 0.5h per significant email thread as a floor
- Round up to nearest half-hour
- Flag all email-derived entries clearly as `[from email]` in the proposal table

---

### Step 5: Deduplicate and round hours

For each proposed entry:
1. Check if the same project already has an entry on that date in the existing entries
2. If yes → flag as `⚠️ possible duplicate` and include in the proposal but visually distinct
3. Round actual duration up to the nearest 0.5h: `ceil(minutes / 30) * 0.5`

---

### Step 6: Present proposals table and ask for approval

Show a numbered table:

```
Proposed entries from calendar (and email) — Week of WEEK_START:

#   Date        Project         Hours  Source        Notes
1   2026-03-31  QTS             1.0h   📅 calendar   QTS weekly working session
2   2026-03-31  Arrow           0.5h   📅 calendar   OAA for ServiceNow discussion
3   2026-04-01  Acrisure        2.5h   📅 calendar   Acrisure dev time
4   2026-04-01  Snowflake       1.0h   📅 calendar   Custom app integration  🆕 new account
5   2026-04-02  IHG             0.5h   📧 email      [from email] Customer email thread
...

Which entries should I log? (all / skip N / merge N and M / cancel)
```

- Label calendar entries with 📅 and email entries with 📧
- Flag new accounts with 🆕
- Flag possible duplicates with ⚠️
- Accept responses like: "all", "1 2 3", "skip 4", "all except 4", "merge 3 and 6"

---

### Step 7: Write approved entries

Use batch write (see **Batch Entry Mode** section). For any 🆕 new accounts, add them to the accounts list first (see **Accounts Management**).

---

### Step 8: Run daily backup

See **Backup & Recovery** section.

---

## Viewing & Reporting

### Weekly Summary

```bash
~/.claude/skills/time-entry/timetracker weekly              # current week
~/.claude/skills/time-entry/timetracker weekly --week 2026-03-30  # specific week (Monday date)
```

### Daily Summary

```bash
~/.claude/skills/time-entry/timetracker day                 # today
~/.claude/skills/time-entry/timetracker day 2026-04-01      # specific date
```

### List Entries

```bash
~/.claude/skills/time-entry/timetracker list                        # recent entries
~/.claude/skills/time-entry/timetracker list --week 2026-03-30      # filter by week
~/.claude/skills/time-entry/timetracker list --date 2026-04-01      # filter by date
~/.claude/skills/time-entry/timetracker list --project GXO          # filter by project
```

---

## Accounts Management

### List Accounts

```bash
~/.claude/skills/time-entry/timetracker accounts list
```

### Add Account
See the "Add account" snippet in the Account/Project Selection section above.

---

## Example Interactions

### Quick entry, existing account:
```
User: log 4h QTS today - OAuth integration
Assistant: [shows account list, QTS selected]
→ Added: 2026-04-02 | QTS | Billable | 4.0h
```

### New account:
```
User: log 2h on Acme Corp
Assistant: "Acme Corp isn't in your accounts list. Add it?"
User: yes
→ Added account: Acme Corp
→ Added: 2026-04-02 | Acme Corp | Billable | 2.0h
```

### First-time setup:
```
User: log 4h QTS
Assistant: "No config found. Where should your time entries be stored?"
User: ~/Documents/timetracker/entries.json
→ Config created. [proceeds to add entry]
```

### Batch entry:
```
User: log this week — Monday 4h GXO Oracle work, 2h FCB. Tuesday 6h Arrow infra review
Assistant: [parses 3 entries, shows summary table, asks to confirm]
→ Added: 2026-03-30 | GXO | Billable | 4.0h — Oracle work
→ Added: 2026-03-30 | FCB | Billable | 2.0h
→ Added: 2026-03-31 | Arrow | Billable | 6.0h — infra review
```

### Weekly summary:
```
User: show this week's hours
→ Week of 2026-03-30
  Total: 33.0h  Billable: 33.0 | Non-Billable: 0 | PTO: 0
  GXO: 9.0 | Arrow: 6.0 | FCB: 6.0 ...
```

### Export to spreadsheet:
```
User: export this week to spreadsheet
→ Exported 19 entries to ~/Library/CloudStorage/.../Timesheet.xlsx
```

---

## Backup & Recovery

### Daily Backup (run after every write operation)

Checks if a backup has already run today — no-op if so, otherwise writes the zip. Safe to call after every write.

```bash
~/.claude/skills/time-entry/timetracker backup
```

### Check Backup Status

```bash
~/.claude/skills/time-entry/timetracker backup status
```

### Restore from Backup

**Always show the user what's in the backup before restoring.** Then confirm before overwriting the live data.

```bash
~/.claude/skills/time-entry/timetracker backup restore
```

Ask the user to confirm, then restore:

```bash
~/.claude/skills/time-entry/timetracker backup restore --confirm
```

---

## Export to Spreadsheet

### Step 0: Ensure uv is installed

`uv` is only needed for Excel export (it pulls in `openpyxl` on demand).

```bash
which uv > /dev/null 2>&1 && echo "ok" || echo "missing"
```

If missing:
```bash
curl -LsSf https://astral.sh/uv/install.sh | sh && source ~/.zprofile
```

### Step 1: Resolve spreadsheet path

Read `~/.config/timetracker.json`. If `spreadsheet_file` key is missing or empty, ask:
> "Where should the timesheet spreadsheet be saved? (e.g. ~/Documents/Timesheet.xlsx)"

Then save it to config:
```bash
~/.claude/skills/time-entry/timetracker config set-spreadsheet "USER_PROVIDED_PATH"
```

### Step 2: Determine week to export

Default: current week. Accept "this week", "last week", or a specific date (YYYY-MM-DD).
Compute `week_start` as the Monday of that week.

### Step 3: Run the export

> No duplicate entries: the script fully replaces the Time Entries and Dashboard sheets on every export. Re-running is always safe.


```bash
uv run --with openpyxl python3 ~/.claude/skills/time-entry/export.py WEEK_START
# WEEK_START is the Monday date, e.g. 2026-03-30. Omit for current week.
```

<!-- legacy heredoc removed — see export.py -->
```bash
# kept for reference only — do not use
uv run --with openpyxl python3 << 'EOF_LEGACY'
import json, os
from datetime import datetime, timedelta
from collections import defaultdict
import openpyxl
from openpyxl.styles import Font, PatternFill, Alignment, Border, Side, numbers
from openpyxl.utils import get_column_letter

# ── Config ──────────────────────────────────────────────────────────────
config_path = os.path.expanduser('~/.config/timetracker.json')
with open(config_path) as f:
    config = json.load(f)

data_file = config['data_file']
xlsx_path = os.path.expanduser(config['spreadsheet_file'])
week_start_str = 'WEEK_START'
target_hours = 40

# ── Load entries ─────────────────────────────────────────────────────────
with open(data_file) as f:
    all_entries = json.load(f)

week_entries = [e for e in all_entries if e.get('week_start') == week_start_str]
week_start = datetime.strptime(week_start_str, '%Y-%m-%d')
week_end = week_start + timedelta(days=6)

# ── Helpers ──────────────────────────────────────────────────────────────
BLUE_DARK   = '1F3864'
BLUE_MID    = '2E74B5'
BLUE_LIGHT  = 'DEEAF1'
GREY_LIGHT  = 'F5F5F5'
GREY_MID    = 'D9D9D9'
GREEN       = 'E2EFDA'
WHITE       = 'FFFFFF'
BLACK       = '000000'

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

def thin_border(sides='all'):
    s = Side(style='thin', color=GREY_MID)
    n = Side(style=None)
    t = s if 'all' in sides or 'top' in sides else n
    b = s if 'all' in sides or 'bottom' in sides else n
    l = s if 'all' in sides or 'left' in sides else n
    r = s if 'all' in sides or 'right' in sides else n
    return Border(top=t, bottom=b, left=l, right=r)

def set_row(ws, row, values, bg=None, font=None, align=None, border=None, height=None):
    for col, val in enumerate(values, 1):
        c = ws.cell(row=row, column=col, value=val)
        if bg:    c.fill = fill(bg)
        if font:  c.font = font
        if align: c.alignment = align
        if border: c.border = border
    if height:
        ws.row_dimensions[row].height = height

def progress_bar(hours, target, width=20):
    filled = min(int((hours / target) * width), width)
    return '█' * filled + '░' * (width - filled)

# ── Compute summary data ──────────────────────────────────────────────────
by_type = defaultdict(float)
by_project = defaultdict(float)
by_day = defaultdict(lambda: defaultdict(float))

for e in week_entries:
    t = e.get('work_type', 'Billable')
    by_type[t] += e['hours']
    by_project[e['project']] += e['hours']
    by_day[e['date']][t] += e['hours']

total_hours = sum(by_type.values())
billable    = by_type.get('Billable', 0)
non_bill    = by_type.get('Non-Billable', 0)
pto         = by_type.get('PTO', 0)
utilization = billable / target_hours if target_hours else 0

days = [(week_start + timedelta(days=i)) for i in range(5)]  # Mon–Fri
day_names = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday']

projects_sorted = sorted(by_project.items(), key=lambda x: -x[1])

# ── Open or create workbook ───────────────────────────────────────────────
os.makedirs(os.path.dirname(xlsx_path), exist_ok=True) if os.path.dirname(xlsx_path) else None

if os.path.exists(xlsx_path):
    wb = openpyxl.load_workbook(xlsx_path)
else:
    wb = openpyxl.Workbook()
    # Remove default sheet
    if 'Sheet' in wb.sheetnames:
        del wb['Sheet']

# ── Rebuild Dashboard sheet ───────────────────────────────────────────────
if 'Dashboard' in wb.sheetnames:
    del wb['Dashboard']
ws = wb.create_sheet('Dashboard', 0)

ws.sheet_view.showGridLines = False
ws.column_dimensions['A'].width = 18
ws.column_dimensions['B'].width = 14
ws.column_dimensions['C'].width = 14
ws.column_dimensions['D'].width = 14
ws.column_dimensions['E'].width = 14
ws.column_dimensions['F'].width = 14

r = 1
# Title
ws.merge_cells(f'A{r}:F{r}')
c = ws.cell(r, 1, '📊 Timesheet Dashboard')
c.font = Font(bold=True, size=16, color=WHITE, name='Calibri')
c.fill = fill(BLUE_DARK)
c.alignment = left()
ws.row_dimensions[r].height = 36

r += 1
# Subtitle: week range + generated date
week_label = f"Week of {week_start.strftime('%b %-d')} – {week_end.strftime('%b %-d, %Y')}"
generated  = f"Generated: {datetime.today().strftime('%b %-d, %Y')}"
ws.merge_cells(f'A{r}:D{r}')
c = ws.cell(r, 1, week_label)
c.font = Font(bold=True, size=11, color=WHITE, name='Calibri')
c.fill = fill(BLUE_MID)
c.alignment = left()
ws.cell(r, 5, generated).font = body_font(color='888888')
ws.row_dimensions[r].height = 22

r += 2
# ── Section: Week Summary ─────────────────────────────────────────────────
ws.merge_cells(f'A{r}:F{r}')
c = ws.cell(r, 1, 'WEEK SUMMARY')
c.font = hdr_font(size=10)
c.fill = fill(BLUE_MID)
c.alignment = left()
ws.row_dimensions[r].height = 18
r += 1

summary_rows = [
    ('Total Hours',          f'{total_hours:.1f}h',  f'{progress_bar(total_hours, target_hours)}  {total_hours/target_hours:.0%} of {target_hours}h target'),
    ('Billable',             f'{billable:.1f}h',     ''),
    ('Non-Billable',         f'{non_bill:.1f}h',     ''),
    ('PTO',                  f'{pto:.1f}h',          ''),
    ('Billable Utilization', f'{utilization:.0%}',   ''),
]
for i, (label, value, note) in enumerate(summary_rows):
    bg = GREY_LIGHT if i % 2 == 0 else WHITE
    bold = (i == 0)
    ws.cell(r, 1, label).font  = body_font(bold=bold)
    ws.cell(r, 1).fill         = fill(bg)
    ws.cell(r, 1).alignment    = left()
    ws.cell(r, 2, value).font  = body_font(bold=bold)
    ws.cell(r, 2).fill         = fill(bg)
    ws.cell(r, 2).alignment    = center()
    if note:
        ws.merge_cells(f'C{r}:F{r}')
        c = ws.cell(r, 3, note)
        c.font = Font(name='Courier New', size=9, color='2E74B5')
        c.fill = fill(bg)
    for col in range(1, 7):
        ws.cell(r, col).border = thin_border()
    ws.row_dimensions[r].height = 18
    r += 1

r += 1
# ── Section: Daily Breakdown ──────────────────────────────────────────────
ws.merge_cells(f'A{r}:F{r}')
c = ws.cell(r, 1, 'DAILY BREAKDOWN')
c.font = hdr_font(size=10)
c.fill = fill(BLUE_MID)
c.alignment = left()
ws.row_dimensions[r].height = 18
r += 1

headers = ['Day', 'Date', 'Billable', 'Non-Billable', 'PTO', 'Total']
for col, h in enumerate(headers, 1):
    c = ws.cell(r, col, h)
    c.font  = hdr_font(size=10, color=WHITE)
    c.fill  = fill(BLUE_DARK)
    c.alignment = center()
    c.border = thin_border()
ws.row_dimensions[r].height = 18
r += 1

for i, (day, name) in enumerate(zip(days, day_names)):
    d_str = day.strftime('%Y-%m-%d')
    b = by_day[d_str].get('Billable', 0)
    n = by_day[d_str].get('Non-Billable', 0)
    p = by_day[d_str].get('PTO', 0)
    t = b + n + p
    has_entries = t > 0
    bg = BLUE_LIGHT if has_entries else WHITE
    text_color = BLACK if has_entries else 'BBBBBB'
    row_vals = [name, day.strftime('%b %-d'), f'{b:.1f}h' if b else '—', f'{n:.1f}h' if n else '—', f'{p:.1f}h' if p else '—', f'{t:.1f}h' if t else '—']
    for col, val in enumerate(row_vals, 1):
        c = ws.cell(r, col, val)
        c.font      = body_font(bold=(col == 6 and has_entries), color=text_color)
        c.fill      = fill(bg)
        c.alignment = center() if col > 1 else left()
        c.border    = thin_border()
    ws.row_dimensions[r].height = 17
    r += 1

# Totals row
total_b = sum(by_day[d.strftime('%Y-%m-%d')].get('Billable', 0) for d in days)
total_n = sum(by_day[d.strftime('%Y-%m-%d')].get('Non-Billable', 0) for d in days)
total_p = sum(by_day[d.strftime('%Y-%m-%d')].get('PTO', 0) for d in days)
for col, val in enumerate(['Total', '', f'{total_b:.1f}h', f'{total_n:.1f}h', f'{total_p:.1f}h', f'{total_b+total_n+total_p:.1f}h'], 1):
    c = ws.cell(r, col, val)
    c.font   = body_font(bold=True)
    c.fill   = fill(GREY_MID)
    c.alignment = center() if col > 1 else left()
    c.border = thin_border()
ws.row_dimensions[r].height = 18
r += 2

# ── Section: Project Summary ──────────────────────────────────────────────
ws.merge_cells(f'A{r}:F{r}')
c = ws.cell(r, 1, 'PROJECT SUMMARY')
c.font = hdr_font(size=10)
c.fill = fill(BLUE_MID)
c.alignment = left()
ws.row_dimensions[r].height = 18
r += 1

for col, h in enumerate(['Project', 'Hours', '% of Total', 'Billable', 'Non-Billable', 'PTO'], 1):
    c = ws.cell(r, col, h)
    c.font = hdr_font(size=10, color=WHITE)
    c.fill = fill(BLUE_DARK)
    c.alignment = center() if col > 1 else left()
    c.border = thin_border()
ws.row_dimensions[r].height = 18
r += 1

for i, (project, hours) in enumerate(projects_sorted):
    bg = GREY_LIGHT if i % 2 == 0 else WHITE
    pct = hours / total_hours if total_hours else 0
    proj_entries = [e for e in week_entries if e['project'] == project]
    pb = sum(e['hours'] for e in proj_entries if e.get('work_type') == 'Billable')
    pn = sum(e['hours'] for e in proj_entries if e.get('work_type') == 'Non-Billable')
    pp = sum(e['hours'] for e in proj_entries if e.get('work_type') == 'PTO')
    for col, val in enumerate([project, f'{hours:.1f}h', f'{pct:.0%}', f'{pb:.1f}h' if pb else '—', f'{pn:.1f}h' if pn else '—', f'{pp:.1f}h' if pp else '—'], 1):
        c = ws.cell(r, col, val)
        c.font = body_font()
        c.fill = fill(bg)
        c.alignment = center() if col > 1 else left()
        c.border = thin_border()
    ws.row_dimensions[r].height = 17
    r += 1

# Grand total
for col, val in enumerate(['Total', f'{total_hours:.1f}h', '100%', '', '', ''], 1):
    c = ws.cell(r, col, val)
    c.font = body_font(bold=True)
    c.fill = fill(GREY_MID)
    c.alignment = center() if col > 1 else left()
    c.border = thin_border()
ws.row_dimensions[r].height = 18
r += 2

# ── Section: Entry Detail ─────────────────────────────────────────────────
ws.merge_cells(f'A{r}:F{r}')
c = ws.cell(r, 1, 'ENTRY DETAIL')
c.font = hdr_font(size=10)
c.fill = fill(BLUE_MID)
c.alignment = left()
ws.row_dimensions[r].height = 18
r += 1

for col, h in enumerate(['Date', 'Day', 'Project', 'Type', 'Hours', 'Notes'], 1):
    c = ws.cell(r, col, h)
    c.font = hdr_font(size=10, color=WHITE)
    c.fill = fill(BLUE_DARK)
    c.alignment = center() if col < 3 else left()
    c.border = thin_border()
ws.row_dimensions[r].height = 18
r += 1

sorted_entries = sorted(week_entries, key=lambda e: (e['date'], e['project']))
for i, e in enumerate(sorted_entries):
    bg = GREY_LIGHT if i % 2 == 0 else WHITE
    d = datetime.strptime(e['date'], '%Y-%m-%d')
    row_vals = [d.strftime('%b %-d'), d.strftime('%A'), e['project'], e.get('work_type', 'Billable'), f"{e['hours']:.1f}h", e.get('notes', '')]
    for col, val in enumerate(row_vals, 1):
        c = ws.cell(r, col, val)
        c.font = body_font()
        c.fill = fill(bg)
        c.alignment = center() if col <= 2 else left()
        c.border = thin_border()
    ws.row_dimensions[r].height = 17
    r += 1

# ── Update Time Entries sheet ─────────────────────────────────────────────
if 'Time Entries' in wb.sheetnames:
    del wb['Time Entries']
te = wb.create_sheet('Time Entries')
te.column_dimensions['A'].width = 14
te.column_dimensions['B'].width = 14
te.column_dimensions['C'].width = 22
te.column_dimensions['D'].width = 14
te.column_dimensions['E'].width = 8
te.column_dimensions['F'].width = 50

# Header
te.cell(1, 1, '🕒 Time Entries').font = Font(bold=True, size=13, name='Calibri')
headers = ['Date', 'Week Start', 'Account / Project', 'Work Type', 'Hours', 'Notes']
for col, h in enumerate(headers, 1):
    c = te.cell(2, col, h)
    c.font = Font(bold=True, color=WHITE, name='Calibri')
    c.fill = fill(BLUE_DARK)
    c.alignment = Alignment(horizontal='center')

sorted_all = sorted(all_entries, key=lambda e: (e['date'], e['project']))
for i, e in enumerate(sorted_all, 3):
    te.cell(i, 1, e['date'])
    te.cell(i, 2, e.get('week_start', ''))
    te.cell(i, 3, e['project'])
    te.cell(i, 4, e.get('work_type', 'Billable'))
    te.cell(i, 5, e['hours'])
    te.cell(i, 6, e.get('notes', ''))

# ── Save ──────────────────────────────────────────────────────────────────
wb.save(xlsx_path)
print(f"Exported {len(week_entries)} entries (week of {week_start_str}) to {xlsx_path}")
EOF_LEGACY
```

### Step 4: Confirm
Show the output line and offer to open the file:
> "Exported N entries to [path]. Open it? (yes/no)"

If yes:
```bash
open "SPREADSHEET_PATH"
```

---

## Email Weekly Report

Trigger phrases: "email report", "send weekly report", "draft report email", "email this week", "email week of [date]"

### Step 0: Resolve config — boss_email and mail_app

Read `~/.config/timetracker.json`. If `boss_email` is missing or empty, ask:
> "What is your manager's email address?"

Then save it:
```bash
~/.claude/skills/time-entry/timetracker config set-boss-email "BOSS_EMAIL"
```

If `mail_app` is missing, detect the default mail client and installed apps:

```bash
~/.claude/skills/time-entry/timetracker config detect-mail-app
```

- If a clear match is detected, use it and save to config as `mail_app`
- If ambiguous (e.g. default is Chrome/Gmail but both apps installed), ask:
  > "Which app do you use for email? 1. Microsoft Outlook  2. Apple Mail"
- Save the choice to config so it's not asked again
- Valid values: `"Microsoft Outlook"` or `"Mail"`

### Step 1: Determine week to export

Default: current week. Accept "this week", "last week", or a specific date.
Compute `week_start` as the Monday of that week (YYYY-MM-DD).

### Step 2: Generate the work summary paragraph

Read all entries for the week including their `notes` fields. Write a single natural-language paragraph summarizing the work done — mention each project by name, its hours, and the nature of the work based on the notes. Keep it concise and professional (3–6 sentences). This is written by you (Claude) directly from the data — do not use a template.

### Step 3: Build and open the draft

Replace all caps placeholders (WEEK_START, BOSS_EMAIL, SUMMARY, MAIL_APP) with real values:

```bash
python3 << 'EOF'
import json, os, subprocess, tempfile
from datetime import datetime, timedelta
from email.mime.multipart import MIMEMultipart
from email.mime.text import MIMEText

config_path = os.path.expanduser('~/.config/timetracker.json')
with open(config_path) as f:
    config = json.load(f)
with open(config['data_file']) as f:
    entries = json.load(f)

week_start_str = 'WEEK_START'
week_entries   = [e for e in entries if e.get('week_start') == week_start_str]

by_type = {}; by_project = {}; by_day = {}
for e in week_entries:
    by_type[e['work_type']] = by_type.get(e['work_type'], 0) + e['hours']
    by_project[e['project']] = by_project.get(e['project'], 0) + e['hours']
    by_day.setdefault(e['date'], {})
    by_day[e['date']][e['project']] = by_day[e['date']].get(e['project'], 0) + e['hours']

total    = sum(by_type.values())
billable = by_type.get('Billable', 0)
nonbill  = by_type.get('Non-Billable', 0)
pto      = by_type.get('PTO', 0)

week_start_dt = datetime.strptime(week_start_str, '%Y-%m-%d')
week_end_dt   = week_start_dt + timedelta(days=4)
days      = [(week_start_dt + timedelta(days=i)) for i in range(5)]
day_names = ['Monday','Tuesday','Wednesday','Thursday','Friday']

SN_DARK   = '#293E40'
SN_GREEN  = '#62D84E'
SN_LIGHT  = '#EDF5EE'
SN_BORDER = '#C8DCC9'

def fmt(h):
    return f'{h:.1f}' if h != int(h) else str(int(h))

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

day_rows = ''
for d, name in zip(days, day_names):
    d_str    = d.strftime('%Y-%m-%d')
    projects = by_day.get(d_str, {})
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

week_label = f"{week_start_dt.strftime('%b %-d')} – {week_end_dt.strftime('%b %-d, %Y')}"
subject    = f"Weekly Time Report — Week of {week_start_dt.strftime('%b %-d, %Y')}"
summary    = 'SUMMARY'   # replaced by Claude-generated paragraph before running

html = f"""<html><body style="font-family:Calibri,Arial,sans-serif;font-size:14px;color:#222;max-width:600px">
<p>Hi,</p>
<p>Here\'s my time report for the week of <b>{week_label}</b>.</p>
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
<p style="font-size:10px;font-weight:bold;letter-spacing:1.5px;color:{SN_DARK};margin-bottom:6px">SUMMARY</p>
<div style="background:{SN_LIGHT};border-left:4px solid {SN_GREEN};border-radius:0 4px 4px 0;padding:12px 16px;color:#2a3a2b;line-height:1.6;font-size:13px;margin-bottom:24px">
  {summary}
</div>
<p>Thanks,<br>Kevin</p>
</body></html>"""

mail_app  = config.get('mail_app', 'Mail')
boss_email = 'BOSS_EMAIL'

html_path = '/tmp/weekly_report_draft_body.html'
with open(html_path, 'w', encoding='utf-8') as f:
    f.write(html)

if mail_app == 'Microsoft Outlook':
    # Outlook on Mac: read HTML from file via do shell script to avoid escaping issues.
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
    # Mail.app: write .eml with X-Unsent header so it opens as a new draft
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
EOF
```

### Step 4: Confirm
Tell the user the draft is open and ready to review before sending.
