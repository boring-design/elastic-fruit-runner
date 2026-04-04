//go:build !darwin

package hostmetrics

// Collect returns zero-value Vitals on non-darwin platforms.
// Host metrics collection is only supported on macOS.
func Collect() Vitals {
	return Vitals{}
}
