// Package cadence holds the pure functions that derive a feed's next-poll
// time from its velocity (entries/day), error history, and origin headers.
// Leaf package: imports only stdlib so internal/db and internal/poll can
// both depend on it without cycling.
package cadence
