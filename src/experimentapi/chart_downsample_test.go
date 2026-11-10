package experimentapi

import (
	"math"
	"testing"

	v1 "github.com/MaxBernstetter/dagobert/src/gen/tradingcopilot/v1"
)

func TestCandleBucketsCoverAndBoundTheCount(t *testing.T) {
	for n := 1; n <= 200; n++ {
		for maxPoints := 1; maxPoints <= 50; maxPoints++ {
			b := candleBuckets(n, maxPoints)
			if len(b) > maxPoints {
				t.Fatalf("n=%d max=%d: %d buckets", n, maxPoints, len(b))
			}
			next := 0
			for _, r := range b {
				if r[0] != next || r[1] <= r[0] {
					t.Fatalf("n=%d max=%d: bucket %v after %d is not contiguous", n, maxPoints, r, next)
				}
				next = r[1]
			}
			if next != n {
				t.Fatalf("n=%d max=%d: buckets end at %d", n, maxPoints, next)
			}
		}
	}
}

func TestCandleBucketsEdgeCases(t *testing.T) {
	if got := candleBuckets(0, 4); got != nil {
		t.Fatalf("empty: %v", got)
	}
	if got := candleBuckets(3, 5); len(got) != 3 {
		t.Fatalf("fewer candles than budget keeps each candle: %v", got)
	}
	if got := candleBuckets(4, 0); len(got) != 4 {
		t.Fatalf("maxPoints 0 keeps every candle: %v", got)
	}
	// Uneven last bucket: 10 candles into 4 buckets of 3 leaves one candle for the last.
	want := [][2]int{{0, 3}, {3, 6}, {6, 9}, {9, 10}}
	got := candleBuckets(10, 4)
	if len(got) != len(want) {
		t.Fatalf("10 into 4: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("10 into 4: %v want %v", got, want)
		}
	}
	if got := candleBuckets(28000, 2000); len(got) != 2000 || got[1999] != [2]int{27986, 28000} {
		t.Fatalf("28000 into 2000: %d buckets, last %v", len(got), got[len(got)-1])
	}
}

// bucketFixture is a 10 candle series: candle i has open i, high i+0.5, low i-0.5, close i+0.25, and an
// indicator whose value is 10*i (NaN at candle 4).
func bucketFixture() *v1.ChartSeries {
	s := &v1.ChartSeries{}
	var sma []float64
	for i := 0; i < 10; i++ {
		s.TimeSec = append(s.TimeSec, int64(1000+i*3600))
		s.Open = append(s.Open, float64(i))
		s.High = append(s.High, float64(i)+0.5)
		s.Low = append(s.Low, float64(i)-0.5)
		s.Close = append(s.Close, float64(i)+0.25)
		v := 10 * float64(i)
		if i == 4 {
			v = math.NaN()
		}
		sma = append(sma, v)
	}
	s.Indicators = []*v1.ChartIndicator{{Id: "sma-3h", Label: "SMA 3h", Pane: v1.ChartPane_CHART_PANE_PRICE, Style: v1.ChartStyle_CHART_STYLE_LINE, Value: sma}}
	s.Portfolio = []*v1.ChartPoint{{TimeSec: 1, Value: 100}}
	s.Markers = []*v1.ChartMarker{{TimeSec: 5, Side: v1.ChartSide_CHART_SIDE_BUY}}
	return s
}

func TestDownsampleSeriesAggregatesOHLCAndIndicators(t *testing.T) {
	in := bucketFixture()
	out := downsampleSeries(in, 4) // buckets [0,3) [3,6) [6,9) [9,10)
	if len(out.TimeSec) != 4 {
		t.Fatalf("buckets %d", len(out.TimeSec))
	}
	wantTime := []int64{1000, 1000 + 3*3600, 1000 + 6*3600, 1000 + 9*3600}
	wantOpen := []float64{0, 3, 6, 9}
	wantHigh := []float64{2.5, 5.5, 8.5, 9.5}
	wantLow := []float64{-0.5, 2.5, 5.5, 8.5}
	wantClose := []float64{2.25, 5.25, 8.25, 9.25}
	for k := range wantTime {
		if out.TimeSec[k] != wantTime[k] || out.Open[k] != wantOpen[k] || out.High[k] != wantHigh[k] ||
			out.Low[k] != wantLow[k] || out.Close[k] != wantClose[k] {
			t.Fatalf("bucket %d: t=%d o=%v h=%v l=%v c=%v", k, out.TimeSec[k], out.Open[k], out.High[k], out.Low[k], out.Close[k])
		}
	}
	// Indicator at each bucket's last candle: 20, 50, 80, 90. Candle 4 is NaN but is not a last candle.
	wantSMA := []float64{20, 50, 80, 90}
	got := out.Indicators[0].Value
	for k := range wantSMA {
		if got[k] != wantSMA[k] {
			t.Fatalf("sma bucket %d = %v want %v", k, got[k], wantSMA[k])
		}
	}
	if out.Indicators[0].Id != "sma-3h" || out.Indicators[0].Style != v1.ChartStyle_CHART_STYLE_LINE {
		t.Fatalf("indicator metadata lost: %+v", out.Indicators[0])
	}
	if out.Portfolio != nil || out.Markers != nil {
		t.Fatal("portfolio and markers are not candles and must be left to the caller")
	}
}

func TestDownsampleSeriesNaNAtLastCandleStaysNaN(t *testing.T) {
	// Two buckets of five: [0,5) ends on candle 4, which is NaN, so that bucket's value stays NaN.
	out := downsampleSeries(bucketFixture(), 2)
	if v := out.Indicators[0].Value[0]; !math.IsNaN(v) {
		t.Fatalf("NaN last candle became %v", v)
	}
	if v := out.Indicators[0].Value[1]; v != 90 {
		t.Fatalf("second bucket = %v want 90", v)
	}
}

func TestDownsampleSeriesIdentityAndInputUnchanged(t *testing.T) {
	in := bucketFixture()
	if got := downsampleSeries(in, 0); got != in {
		t.Fatal("maxPoints 0 must return the series itself")
	}
	if got := downsampleSeries(in, 10); got != in {
		t.Fatal("fewer candles than budget must return the series itself")
	}
	before := len(in.TimeSec)
	downsampleSeries(in, 3)
	if len(in.TimeSec) != before || in.Indicators[0].Value[9] != 90 {
		t.Fatal("downsampling modified its input")
	}
}
