package bars

const (
	HourMS int64 = 3_600_000
	BarMS  int64 = 10_800_000
)

// Candle is one closed 1h kline from the lake.
type Candle struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// Bar is one complete 3h UTC bar built from three 1h candles.
type Bar struct {
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	OpenTime  int64
	CloseTime int64
	HourClose [3]float64
}

// Series returns every complete 3h bar at a UTC boundary with open time
// at or before barOpen, ordered by open time. The requested bar must be
// complete. Earlier incomplete triples are left out.
func Series(candles []Candle, barOpen int64) ([]Bar, bool) {
	byOpen := make(map[int64]Candle, len(candles))
	for _, candle := range candles {
		byOpen[candle.OpenTime] = candle
	}

	opens := make([]int64, 0)
	seen := make(map[int64]struct{})
	consider := func(open int64) {
		if open%BarMS != 0 || open > barOpen {
			return
		}
		if _, ok := seen[open]; ok {
			return
		}
		seen[open] = struct{}{}
		opens = append(opens, open)
	}
	consider(barOpen)
	for open := range byOpen {
		consider(open)
	}
	sortInt64(opens)

	series := make([]Bar, 0, len(opens))
	for _, open := range opens {
		bar, ok := build(byOpen, open)
		if !ok {
			if open == barOpen {
				return nil, false
			}
			continue
		}
		series = append(series, bar)
	}
	if len(series) == 0 || series[len(series)-1].OpenTime != barOpen {
		return nil, false
	}
	return series, true
}

func build(byOpen map[int64]Candle, open int64) (Bar, bool) {
	first, ok0 := byOpen[open]
	second, ok1 := byOpen[open+HourMS]
	third, ok2 := byOpen[open+2*HourMS]
	if !ok0 || !ok1 || !ok2 {
		return Bar{}, false
	}
	return Bar{
		Open:      first.Open,
		High:      max3(first.High, second.High, third.High),
		Low:       min3(first.Low, second.Low, third.Low),
		Close:     third.Close,
		Volume:    first.Volume + second.Volume + third.Volume,
		OpenTime:  open,
		CloseTime: third.CloseTime,
		HourClose: [3]float64{first.Close, second.Close, third.Close},
	}, true
}

func max3(a, b, c float64) float64 {
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	return a
}

func min3(a, b, c float64) float64 {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

func sortInt64(values []int64) {
	for i := 1; i < len(values); i++ {
		v := values[i]
		j := i
		for j > 0 && values[j-1] > v {
			values[j] = values[j-1]
			j--
		}
		values[j] = v
	}
}
