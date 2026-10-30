// Copyright 2021-2026 ALTESSA SOLUTIONS INC.
// Use of this source code is governed by the MIT license that can be found in
// the LICENSE file.

// Package civil provides [Date], a calendar date without a time zone.
//
// Due dates, service periods, effective dates and deadlines are calendar
// dates: converting them between time zones must never move them to another
// day. Store instants as UTC timestamps and calendar dates as [Date].
//
// # Zero value
//
// The zero Date means "no date". It formats, marshals and is stored as the
// empty string, and the empty string parses back to the zero Date, so an
// optional date needs no pointer.
//
// # Text and JSON
//
// A Date marshals as its "YYYY-MM-DD" string ([Date.MarshalText]); that
// form sorts in date order. [Date.UnmarshalJSONFrom] also accepts the object
// form {"Year":2026,"Month":10,"Day":9} that encoding/json produced for
// date structs without text methods, so documents written before a service
// adopted this type still decode.
//
// JSON is implemented by [Date.MarshalJSONTo] and [Date.UnmarshalJSONFrom],
// the encoding/json/v2 interfaces, which encoding/json also calls; the two
// codecs emit identical bytes for every date. The legacy object form is
// decoded with the encoding/json (v1) rules whichever codec reads it.
//
// # Usage
//
//	due := civil.MustNew(2026, time.January, 31)
//	next := due.AddMonthsClamped(1, 31) // 2026-02-28
//	today := civil.Today(loc)
//	if today.After(due) { ... }
//
// [TodayFrom] reads the instant from an injected
// [github.com/altessa-s/go-atlas-core/time/clock.Clock], so tests can pin
// today's date with a fake clock; [Today] uses the real clock.
//
// Date does not implement BSON hooks itself; a BSON codec for it must be
// registered by the caller's Mongo registry.
package civil
