// SPDX-License-Identifier: Apache-2.0
package adaptor

import "time"

// RealClock implements port.Clock using the system clock.
type RealClock struct{}

func (c *RealClock) Now() time.Time {
	return time.Now().UTC()
}
