package memstore

// this function is only for testing purposes
func (m *Memstore) Count() int {
	count := 0
	m.sessions.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}
