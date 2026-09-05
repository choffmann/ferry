package chaos

func Ready(s State) bool {
	return s.Readiness != "fail"
}
