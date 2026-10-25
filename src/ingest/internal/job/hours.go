package job

import "math"

// hourOpens returns hour-aligned open times t with from <= t < to.
func hourOpens(from, to int64) ([]int64, bool) {
	t := from
	if rem := t % hourMS; rem != 0 {
		if t >= 0 {
			delta := hourMS - rem
			if t > math.MaxInt64-delta {
				return nil, false
			}
			t += delta
		} else {
			t -= rem
		}
	}
	if t < from {
		return nil, false
	}
	var out []int64
	for t < to {
		out = append(out, t)
		if t > math.MaxInt64-hourMS {
			break
		}
		t += hourMS
	}
	return out, true
}
