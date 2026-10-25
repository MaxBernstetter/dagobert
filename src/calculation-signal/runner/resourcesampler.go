package runner

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/MaxBernstetter/dagobert/src/calculation-signal/internal/framework"
)

const resourceSampleInterval = 100 * time.Millisecond

// resourceSampler periodically records process CPU%, RSS, and heap alloc.
type resourceSampler struct {
	stop chan struct{}
	done chan struct{}

	mu       sync.Mutex
	cpu      []float64
	rss      []float64
	heap     []float64
	lastCPU  float64 // cumulative CPU seconds
	lastWall time.Time
	haveCPU  bool
}

func startResourceSampler(interval time.Duration) *resourceSampler {
	if interval <= 0 {
		interval = resourceSampleInterval
	}
	s := &resourceSampler{
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	s.sampleOnce()
	go func() {
		defer close(s.done)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-s.stop:
				return
			case <-t.C:
				s.sampleOnce()
			}
		}
	}()
	return s
}

func (s *resourceSampler) Stop() {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	<-s.done
	s.sampleOnce()
}

func (s *resourceSampler) Apply(report *framework.RunReport) {
	s.mu.Lock()
	defer s.mu.Unlock()
	report.CPUPercent = framework.StatsFromSamples(s.cpu)
	report.RSSBytes = framework.StatsFromSamples(s.rss)
	report.HeapAllocBytes = framework.StatsFromSamples(s.heap)
}

func (s *resourceSampler) sampleOnce() {
	now := time.Now()
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	heap := float64(mem.HeapAlloc)

	rss, rssOK := readRSSBytes()
	cpuSec, cpuOK := readCPUSeconds()

	s.mu.Lock()
	defer s.mu.Unlock()
	s.heap = append(s.heap, heap)
	if rssOK {
		s.rss = append(s.rss, float64(rss))
	}
	if cpuOK {
		if s.haveCPU {
			wall := now.Sub(s.lastWall).Seconds()
			if wall > 0 {
				delta := cpuSec - s.lastCPU
				if delta < 0 {
					delta = 0
				}
				s.cpu = append(s.cpu, 100*delta/wall)
			}
		}
		s.lastCPU = cpuSec
		s.lastWall = now
		s.haveCPU = true
	}
}

func readRSSBytes() (uint64, bool) {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return 0, false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "VmRSS:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0, false
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, false
		}
		return kb * 1024, true
	}
	return 0, false
}

// readCPUSeconds returns cumulative process user+system CPU time in seconds.
func readCPUSeconds() (float64, bool) {
	b, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, false
	}
	// Comm may contain spaces/parens; find trailing ")" then field-split the rest.
	s := string(b)
	idx := strings.LastIndex(s, ")")
	if idx < 0 || idx+2 >= len(s) {
		return 0, false
	}
	fields := strings.Fields(s[idx+2:])
	// After comm: state(1) ... utime(12) stime(13) relative to post-comm fields → indices 11,12
	if len(fields) < 13 {
		return 0, false
	}
	utime, err1 := strconv.ParseUint(fields[11], 10, 64)
	stime, err2 := strconv.ParseUint(fields[12], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	clk := float64(clockTicksPerSecond())
	if clk <= 0 {
		clk = 100
	}
	return float64(utime+stime) / clk, true
}

func clockTicksPerSecond() int64 {
	// Linux default; reading via sysconf would need cgo/x/sys — 100 is correct on glibc Linux.
	return 100
}
