package decision

func numberAllowed(number string, allowlist []string) bool {
	for _, n := range allowlist {
		if n == number {
			return true
		}
	}
	return false
}
