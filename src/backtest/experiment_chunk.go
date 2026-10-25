package backtest

// ChunkStrings splits ids into batches of at most size (size < 1 → one batch).
func ChunkStrings(ids []string, size int) [][]string {
	if len(ids) == 0 {
		return nil
	}
	if size < 1 {
		return [][]string{ids}
	}
	var out [][]string
	for i := 0; i < len(ids); i += size {
		end := i + size
		if end > len(ids) {
			end = len(ids)
		}
		out = append(out, ids[i:end])
	}
	return out
}
