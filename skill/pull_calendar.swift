// pull_calendar.swift
// Reads calendar events for a given week using EventKit (macOS).
// Much faster than AppleScript (~0.4s vs ~65s).
//
// Usage:
//   swiftc pull_calendar.swift -o pull_calendar
//   ./pull_calendar YYYY-MM-DD YYYY-MM-DD
//
// Arguments:
//   $1 — week start date (inclusive), e.g. 2026-03-30
//   $2 — week end date   (exclusive), e.g. 2026-04-04
//
// Output (one line per event, pipe-delimited):
//   calendarName|eventTitle|startDatetime|endDatetime|isAllDay(0|1)
//
// To rebuild the binary:
//   swiftc ~/.claude/skills/time-entry/pull_calendar.swift \
//          -o ~/.claude/skills/time-entry/pull_calendar

import EventKit
import Foundation

// MARK: — Argument parsing

let args = CommandLine.arguments
guard args.count == 3 else {
    fputs("Usage: pull_calendar <start-date> <end-date>\n", stderr)
    fputs("Example: pull_calendar 2026-03-30 2026-04-04\n", stderr)
    exit(1)
}

let dateFormatter = DateFormatter()
dateFormatter.dateFormat = "yyyy-MM-dd"
dateFormatter.timeZone = TimeZone.current

guard
    let startDate = dateFormatter.date(from: args[1]),
    let endDate   = dateFormatter.date(from: args[2])
else {
    fputs("ERROR: Dates must be in YYYY-MM-DD format\n", stderr)
    exit(1)
}

// MARK: — EventKit access

let store = EKEventStore()
let sema  = DispatchSemaphore(value: 0)

if #available(macOS 14.0, *) {
    store.requestFullAccessToEvents { granted, _ in
        if !granted { fputs("ERROR: Calendar access denied\n", stderr); exit(1) }
        sema.signal()
    }
} else {
    store.requestAccess(to: .event) { granted, _ in
        if !granted { fputs("ERROR: Calendar access denied\n", stderr); exit(1) }
        sema.signal()
    }
}
sema.wait()

// MARK: — Fetch and print events

let outputFormatter = DateFormatter()
outputFormatter.dateFormat = "yyyy-MM-dd HH:mm"
outputFormatter.timeZone = TimeZone.current

let pred   = store.predicateForEvents(withStart: startDate, end: endDate, calendars: nil)
let events = store.events(matching: pred)

for event in events {
    let calName  = event.calendar?.title ?? "unknown"
    let title    = (event.title ?? "").replacingOccurrences(of: "|", with: "·")  // sanitize delimiter
    let start    = outputFormatter.string(from: event.startDate)
    let end      = outputFormatter.string(from: event.endDate)
    let allDay   = event.isAllDay ? "1" : "0"
    print("\(calName)|\(title)|\(start)|\(end)|\(allDay)")
}
