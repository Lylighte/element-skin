package permission

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
