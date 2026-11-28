//go:build race

package main

// raceEnabled is true when the tests are built with the race detector, which slows code down by an order of
// magnitude: timing budgets are not meaningful then.
const raceEnabled = true
