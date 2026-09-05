package chaos

import "time"

func Delay(s State) time.Duration {
	if s.HTTP.LatencyMS <= 0 {
		return 0
	}
	return time.Duration(s.HTTP.LatencyMS) * time.Millisecond
}
