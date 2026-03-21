package store

const sqliteReadSliceInitialCapMax = 128

func boundedReadSliceInitialCapacity(limit int) int {
	if limit <= 0 {
		return 0
	}
	if limit > sqliteReadSliceInitialCapMax {
		return sqliteReadSliceInitialCapMax
	}
	return limit
}