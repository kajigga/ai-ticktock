#!/usr/bin/env python3
"""
tt.py — time tracker CLI
Usage: python3 tt.py <command> [subcommand] [args]

Commands:
  add DATE PROJECT WORK_TYPE HOURS NOTES
  batch JSON_ARRAY
  amend show <last|first|N|ID>
  amend update ID field=value [field=value ...]
  show [WEEK_START]
  list [--project P] [--type T] [--from D] [--to D] [--limit N]
  backup
  backup status
  config add-account NAME
  config set-spreadsheet PATH
  config set-boss-email EMAIL
  config detect-mail-app
"""

import json, sys, os, uuid, zipfile
from datetime import datetime, timedelta

SKILLS_DIR  = os.path.dirname(os.path.abspath(__file__))
CONFIG_PATH = os.path.expanduser('~/.config/timetracker.json')


# ── Helpers ───────────────────────────────────────────────────────────────

def load_config():
    with open(CONFIG_PATH) as f:
        return json.load(f)

def save_config(config):
    with open(CONFIG_PATH, 'w') as f:
        json.dump(config, f, indent=2)

def load_entries(config):
    with open(config['data_file']) as f:
        return json.load(f)

def save_entries(config, entries):
    with open(config['data_file'], 'w') as f:
        json.dump(entries, f, indent=2)

def make_entry(date_str, project, work_type, hours, notes):
    d = datetime.strptime(date_str, '%Y-%m-%d')
    week_start = (d - timedelta(days=d.weekday())).strftime('%Y-%m-%d')
    return {
        'id':         uuid.uuid4().hex,
        'date':       date_str,
        'week_start': week_start,
        'project':    project,
        'work_type':  work_type,
        'hours':      float(hours),
        'notes':      notes,
        'created_at': datetime.now().isoformat(timespec='seconds'),
    }

def fmt_entry(e):
    notes = ' -- ' + e['notes'] if e.get('notes') else ''
    return '{} | {} | {} | {}h{}'.format(
        e['date'], e['project'], e['work_type'], e['hours'], notes)

def current_week_start():
    today = datetime.today()
    return (today - timedelta(days=today.weekday())).strftime('%Y-%m-%d')

def die(msg):
    print('ERROR: ' + msg, file=sys.stderr)
    sys.exit(1)


# ── Commands ──────────────────────────────────────────────────────────────

def cmd_add(args):
    """add DATE PROJECT WORK_TYPE HOURS NOTES"""
    if len(args) < 5:
        die('Usage: tt.py add DATE PROJECT WORK_TYPE HOURS NOTES')
    date_str, project, work_type, hours, notes = args[0], args[1], args[2], args[3], args[4]
    config  = load_config()
    entries = load_entries(config)
    entry   = make_entry(date_str, project, work_type, hours, notes)
    entries.append(entry)
    save_entries(config, entries)
    print('Added: ' + fmt_entry(entry))


def cmd_batch(args):
    """batch JSON_ARRAY  — add multiple entries from a JSON array"""
    if not args:
        die('Usage: tt.py batch \'[{"date":...,"project":...,"work_type":...,"hours":N,"notes":...}, ...]\'')
    raw    = ' '.join(args)
    parsed = json.loads(raw)
    items  = parsed if isinstance(parsed, list) else [parsed]
    config  = load_config()
    entries = load_entries(config)
    new = [make_entry(i['date'], i['project'], i.get('work_type', 'Billable'),
                      i['hours'], i.get('notes', '')) for i in items]
    entries.extend(new)
    save_entries(config, entries)
    for e in new:
        print('Added: ' + fmt_entry(e))


def cmd_amend(args):
    """amend show <last|first|N|ID>  |  amend update ID field=value ..."""
    if not args:
        die('Usage: tt.py amend show <last|first|N|ID>\n       tt.py amend update ID field=value ...')
    sub = args[0]

    config  = load_config()
    entries = load_entries(config)

    def find(selector):
        sorted_e = sorted(entries, key=lambda e: e.get('created_at', e['date']))
        if selector == 'last':  return sorted_e[-1]
        if selector == 'first': return sorted_e[0]
        try:
            n = int(selector)
            return sorted_e[-n]
        except ValueError:
            pass
        matches = [e for e in entries if e['id'] == selector]
        if matches: return matches[0]
        die('Entry not found: ' + selector)

    if sub == 'show':
        if len(args) < 2:
            die('Usage: tt.py amend show <last|first|N|ID>')
        e = find(args[1])
        updated = '  updated_at: ' + e['updated_at'] if 'updated_at' in e else ''
        print('Entry:')
        print('  id:         ' + e['id'])
        print('  date:       ' + e['date'])
        print('  project:    ' + e['project'])
        print('  work_type:  ' + e['work_type'])
        print('  hours:      ' + str(e['hours']))
        print('  notes:      ' + e.get('notes', ''))
        print('  created_at: ' + e.get('created_at', 'unknown') + updated)

    elif sub == 'update':
        if len(args) < 3:
            die('Usage: tt.py amend update ID field=value ...')
        entry_id = args[1]
        entry = next((e for e in entries if e['id'] == entry_id), None)
        if not entry:
            die('No entry with id: ' + entry_id)
        for pair in args[2:]:
            key, _, val = pair.partition('=')
            if key == 'hours':
                entry['hours'] = float(val)
            elif key == 'date':
                entry['date'] = val
                d = datetime.strptime(val, '%Y-%m-%d')
                entry['week_start'] = (d - timedelta(days=d.weekday())).strftime('%Y-%m-%d')
            else:
                entry[key] = val
        entry['updated_at'] = datetime.now().isoformat(timespec='seconds')
        save_entries(config, entries)
        print('Updated: ' + fmt_entry(entry))
        print('updated_at: ' + entry['updated_at'])
    else:
        die('Unknown amend subcommand: ' + sub)


def cmd_show(args):
    """show [WEEK_START]  — weekly summary, defaults to current week"""
    config  = load_config()
    entries = load_entries(config)
    week_start = args[0] if args else current_week_start()
    week_entries = [e for e in entries if e.get('week_start') == week_start]

    total   = sum(e['hours'] for e in week_entries)
    by_type = {}
    by_proj = {}
    by_day  = {}
    for e in week_entries:
        by_type[e['work_type']] = by_type.get(e['work_type'], 0) + e['hours']
        by_proj[e['project']]   = by_proj.get(e['project'], 0)  + e['hours']
        by_day[e['date']]       = by_day.get(e['date'], 0)      + e['hours']

    print('Week of ' + week_start)
    print('Total: {}h  |  Billable: {}  Non-Billable: {}  PTO: {}'.format(
        total,
        by_type.get('Billable', 0),
        by_type.get('Non-Billable', 0),
        by_type.get('PTO', 0),
    ))
    print('\nBy project:')
    for proj, hrs in sorted(by_proj.items(), key=lambda x: -x[1]):
        print('  {:<22} {}h'.format(proj, hrs))
    print('\nBy day:')
    ws = datetime.strptime(week_start, '%Y-%m-%d')
    for i in range(5):
        d = (ws + timedelta(days=i))
        hrs = by_day.get(d.strftime('%Y-%m-%d'), 0)
        bar = '#' * int(hrs * 2)
        print('  {} {:5.1f}h  {}'.format(d.strftime('%a %b %-d'), hrs, bar))


def cmd_list(args):
    """list [--project P] [--type T] [--from D] [--to D] [--limit N]"""
    config  = load_config()
    entries = load_entries(config)

    filter_project = filter_type = filter_from = filter_to = None
    limit = 20
    i = 0
    while i < len(args):
        if args[i] == '--project' and i + 1 < len(args):
            filter_project = args[i+1]; i += 2
        elif args[i] == '--type' and i + 1 < len(args):
            filter_type = args[i+1]; i += 2
        elif args[i] == '--from' and i + 1 < len(args):
            filter_from = args[i+1]; i += 2
        elif args[i] == '--to' and i + 1 < len(args):
            filter_to = args[i+1]; i += 2
        elif args[i] == '--limit' and i + 1 < len(args):
            limit = int(args[i+1]); i += 2
        else:
            i += 1

    filtered = entries
    if filter_project: filtered = [e for e in filtered if e['project']   == filter_project]
    if filter_type:    filtered = [e for e in filtered if e['work_type'] == filter_type]
    if filter_from:    filtered = [e for e in filtered if e['date']      >= filter_from]
    if filter_to:      filtered = [e for e in filtered if e['date']      <= filter_to]

    filtered = sorted(filtered, key=lambda e: (e['date'], e.get('created_at', '')), reverse=True)[:limit]

    current_day = None
    for e in filtered:
        if e['date'] != current_day:
            current_day = e['date']
            d = datetime.strptime(current_day, '%Y-%m-%d')
            print('\n' + d.strftime('%A, %b %-d'))
        notes = ' -- ' + e['notes'] if e.get('notes') else ''
        print('  {:<22} {:5.1f}h  {}{}'.format(e['project'], e['hours'], e['work_type'], notes))


def cmd_backup(args):
    """backup         — run daily backup (no-op if already done today)
    backup status  — show backup info"""
    config = load_config()
    sub = args[0] if args else None

    if sub == 'status':
        backup_file = config.get('backup_file', 'not configured')
        last_date   = config.get('last_backup_date', 'never')
        if os.path.exists(backup_file):
            size = os.path.getsize(backup_file)
            entry_count = '(unknown)'
            try:
                with zipfile.ZipFile(backup_file) as zf:
                    with zf.open(zf.infolist()[0]) as f2:
                        entry_count = str(len(json.load(f2))) + ' entries'
            except Exception:
                pass
            print('Backup file:     ' + backup_file)
            print('Last backup:     ' + last_date)
            print('Backup size:     {:,} bytes'.format(size))
            print('Backup contains: ' + entry_count)
        else:
            print('No backup found at: ' + backup_file)
            print('Last backup date in config: ' + last_date)
    else:
        today = datetime.today().strftime('%Y-%m-%d')
        if config.get('last_backup_date') == today:
            print('Backup already current ({}), skipping.'.format(today))
        else:
            data_file   = config['data_file']
            backup_file = config['backup_file']
            os.makedirs(os.path.dirname(backup_file), exist_ok=True)
            with zipfile.ZipFile(backup_file, 'w', zipfile.ZIP_DEFLATED) as zf:
                zf.write(data_file, os.path.basename(data_file))
            config['last_backup_date'] = today
            save_config(config)
            print('Backup saved -> ' + backup_file)


def cmd_config(args):
    """config add-account NAME
    config set-spreadsheet PATH
    config set-boss-email EMAIL
    config detect-mail-app"""
    if not args:
        die('Usage: tt.py config <add-account|set-spreadsheet|set-boss-email|detect-mail-app> [value]')
    sub = args[0]
    config = load_config()

    if sub == 'add-account':
        if len(args) < 2:
            die('Usage: tt.py config add-account NAME')
        name = args[1]
        if name not in config['accounts']:
            config['accounts'].append(name)
            save_config(config)
            print('Added account: ' + name)
        else:
            print('Account already exists: ' + name)

    elif sub == 'set-spreadsheet':
        if len(args) < 2:
            die('Usage: tt.py config set-spreadsheet PATH')
        path = os.path.expanduser(args[1])
        config['spreadsheet_file'] = path
        save_config(config)
        print('Saved spreadsheet path: ' + path)

    elif sub == 'set-boss-email':
        if len(args) < 2:
            die('Usage: tt.py config set-boss-email EMAIL')
        config['boss_email'] = args[1]
        save_config(config)
        print('Saved boss_email: ' + args[1])

    elif sub == 'detect-mail-app':
        import ctypes, ctypes.util
        cf = ctypes.cdll.LoadLibrary(ctypes.util.find_library('CoreFoundation'))
        ls = ctypes.cdll.LoadLibrary(ctypes.util.find_library('CoreServices'))
        cf.CFStringCreateWithCString.restype  = ctypes.c_void_p
        cf.CFStringCreateWithCString.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_uint32]
        ls.LSCopyDefaultHandlerForURLScheme.restype  = ctypes.c_void_p
        ls.LSCopyDefaultHandlerForURLScheme.argtypes = [ctypes.c_void_p]
        cf.CFStringGetCString.restype  = ctypes.c_bool
        cf.CFStringGetCString.argtypes = [ctypes.c_void_p, ctypes.c_char_p, ctypes.c_long, ctypes.c_uint32]

        scheme_ref  = cf.CFStringCreateWithCString(None, b'mailto', 0x08000100)
        handler_ref = ls.LSCopyDefaultHandlerForURLScheme(scheme_ref)
        bundle_id   = ''
        if handler_ref:
            buf = ctypes.create_string_buffer(256)
            cf.CFStringGetCString(handler_ref, buf, 256, 0x08000100)
            bundle_id = buf.value.decode('utf-8').lower()

        outlook_installed = os.path.isdir('/Applications/Microsoft Outlook.app')
        mail_installed    = os.path.isdir('/System/Applications/Mail.app')

        if 'outlook' in bundle_id:       detected = 'Microsoft Outlook'
        elif 'apple.mail' in bundle_id:  detected = 'Mail'
        elif outlook_installed:          detected = 'Microsoft Outlook'
        elif mail_installed:             detected = 'Mail'
        else:                            detected = 'UNKNOWN'

        print('bundle_id=' + bundle_id)
        print('outlook_installed=' + str(outlook_installed))
        print('detected=' + detected)
    else:
        die('Unknown config subcommand: ' + sub)


# ── Dispatch ──────────────────────────────────────────────────────────────

COMMANDS = {
    'add':    cmd_add,
    'batch':  cmd_batch,
    'amend':  cmd_amend,
    'show':   cmd_show,
    'list':   cmd_list,
    'backup': cmd_backup,
    'config': cmd_config,
}

if __name__ == '__main__':
    if len(sys.argv) < 2 or sys.argv[1] in ('-h', '--help'):
        print(__doc__)
        sys.exit(0)
    cmd = sys.argv[1]
    if cmd not in COMMANDS:
        die('Unknown command: {}. Run with --help for usage.'.format(cmd))
    COMMANDS[cmd](sys.argv[2:])
