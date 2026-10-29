// Copyright 2021-2026 ALTESSA SOLUTIONS INC. All rights reserved.
// Use of this source code is governed by license that can be found in
// the LICENSE file.

package scheduler_test

import (
	"context"
	"testing"

	"github.com/altessa-s/go-atlas-core/scheduler"
)

func BenchmarkTaskPriority_String(b *testing.B) {
	p := scheduler.TaskPriorityNormal
	for b.Loop() {
		_ = p.String()
	}
}

func BenchmarkTaskConfig_Construction(b *testing.B) {
	for b.Loop() {
		cfg := scheduler.TaskConfig{
			ID:       "bench-task",
			Schedule: "@every 5m",
			Func:     func(ctx context.Context) error { return nil },
			Priority: scheduler.TaskPriorityNormal,
			Meta: map[string]string{
				"team":  "platform",
				"owner": "bench",
			},
		}
		_ = cfg
	}
}

func BenchmarkParseSchedule(b *testing.B) {
	for _, spec := range []string{"@every 5m", "0 */5 * * * MON-FRI", "CRON_TZ=Europe/Moscow 0 0 3 * * *"} {
		b.Run(spec, func(b *testing.B) {
			for b.Loop() {
				if _, err := scheduler.ParseSchedule(spec); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
