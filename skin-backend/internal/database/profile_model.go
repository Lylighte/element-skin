package database

func NormalizeProfileModel(m string) string {
	if m == "slim" {
		return "slim"
	}
	return "default"
}
