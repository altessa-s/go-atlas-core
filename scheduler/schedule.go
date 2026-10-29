// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package scheduler

import (
	"errors"
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"

	coreerrs "github.com/altessa-s/go-atlas-core/errors"
)

// ErrInvalidSchedule reports a [TaskConfig.Schedule] spec that does not parse.
// It wraps [coreerrs.ErrInvalidArgument], so errors.Is matches either.
var ErrInvalidSchedule = fmt.Errorf("%w: invalid schedule", coreerrs.ErrInvalidArgument)

// scheduleParser is the one cron grammar of go-atlas: six fields with leading
// seconds plus descriptors ("@every 5m", "@hourly", ...). A "CRON_TZ=" or
// "TZ=" prefix selects the time zone of the spec. A cron.Parser is a
// read-only value, safe for concurrent use.
var scheduleParser = cron.NewParser(
	cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// ParseSchedule parses spec with the grammar every go-atlas scheduler uses
// (service/scheduler included): six space-separated fields with leading
// seconds ("0 */5 * * * *"), or a descriptor ("@every 5m", "@hourly",
// "@daily", ...), optionally prefixed with "CRON_TZ=<zone> " or
// "TZ=<zone> ". A spec that does not parse, the empty spec included, returns
// an error matching [ErrInvalidSchedule] that quotes spec.
func ParseSchedule(spec string) (schedule cron.Schedule, err error) {
	if perr := checkZonePrefix(spec); perr != nil {
		return nil, fmt.Errorf("%w %q: %w", ErrInvalidSchedule, spec, perr)
	}
	defer func() {
		// The parser is third-party code fed with configuration: a spec it cannot handle must be an invalid spec, not a crash.
		if r := recover(); r != nil {
			schedule, err = nil, fmt.Errorf("%w %q: the cron parser failed: %v", ErrInvalidSchedule, spec, r)
		}
	}()
	schedule, err = scheduleParser.Parse(spec)
	if err != nil {
		return nil, fmt.Errorf("%w %q: %w", ErrInvalidSchedule, spec, err)
	}
	return schedule, nil
}

// errZoneWithoutSchedule rejects a time zone prefix that no space separates from a schedule; robfig/cron v3 slices such a spec
// out of range and panics.
var errZoneWithoutSchedule = errors.New(`a "CRON_TZ=" or "TZ=" prefix needs a space and a schedule after the zone`)

// checkZonePrefix checks that a "CRON_TZ=" or "TZ=" prefix of spec is followed by a space, as the parser requires.
func checkZonePrefix(spec string) error {
	if (strings.HasPrefix(spec, "TZ=") || strings.HasPrefix(spec, "CRON_TZ=")) && !strings.Contains(spec, " ") {
		return errZoneWithoutSchedule
	}
	return nil
}

// ValidateSchedule reports whether spec parses with [ParseSchedule]; it
// returns nil or an error matching [ErrInvalidSchedule].
func ValidateSchedule(spec string) error {
	_, err := ParseSchedule(spec)
	return err
}
