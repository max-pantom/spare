package backup

// Restore is an explicit name for callers that prefer lifecycle terminology.
func Restore(source, destination string) (Manifest, error) {
	return Import(source, destination)
}
