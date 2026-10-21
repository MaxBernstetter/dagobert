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

// windows splits missing open times into contiguous pages of at most 1000 hours.
// The second value is the inclusive endTime: min(lastOpen+hour-1, to-1).
func windows(missing []int64, to int64) [][2]int64 {
	var out [][2]int64
	for i := 0; i < len(missing); {
		j := i
		n := 1
		for j+1 < len(missing) && missing[j+1] == missing[j]+hourMS && n < pageLimit {
			j++
			n++
		}
		start := missing[i]
		end := int64(math.MaxInt64)
		if missing[j] <= math.MaxInt64-hourMS+1 {
			end = missing[j] + hourMS - 1
		}
		if to-1 < end {
			end = to - 1
		}
		out = append(out, [2]int64{start, end})
		i = j + 1
	}
	return out
}
