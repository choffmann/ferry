package chaos

// FailWith takes the random draw as an argument rather than drawing itself, so
// the middleware stays deterministic under test.
func FailWith(s State, draw float64) (int, bool) {
	if s.HTTP.ErrorRate <= 0 || draw >= s.HTTP.ErrorRate {
		return 0, false
	}
	status := s.HTTP.Status
	if status < 400 || status > 599 {
		status = 500
	}
	return status, true
}
