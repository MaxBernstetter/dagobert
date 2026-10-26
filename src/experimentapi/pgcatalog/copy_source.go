package pgcatalog

import (
	"crypto/rand"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// pgUUID wraps a uuid for COPY. uuid.UUID implements driver.Valuer, which pgx would encode
// through the slow database/sql path; pgtype.UUID takes the binary codec directly.
func pgUUID(u uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: u, Valid: true} }

// idPool hands out random version 4 UUIDs from one bulk crypto/rand read instead of one
// read per row.
type idPool struct {
	buf [256 * 16]byte
	off int
}

func (p *idPool) next() pgtype.UUID {
	if p.off == 0 || p.off >= len(p.buf) {
		_, _ = rand.Read(p.buf[:])
		p.off = 0
	}
	var b [16]byte
	copy(b[:], p.buf[p.off:p.off+16])
	p.off += 16
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return pgtype.UUID{Bytes: b, Valid: true}
}

// fillSource streams the fill rows of all completed items into COPY without materialising
// them. It reuses one row slice; pgx encodes each row before asking for the next.
type fillSource struct {
	items []InstanceBatchItem
	ids   []uuid.UUID
	exp   pgtype.UUID
	cur   int // item index, -1 before the first Next
	fill  int // fill index within the current item
	side  string
	inst  pgtype.UUID
	pool  idPool
	row   [11]any
}

func (s *fillSource) Next() bool {
	for {
		if s.cur >= 0 && s.cur < len(s.items) && s.items[s.cur].Error == "" {
			fills := s.items[s.cur].Fills
			for s.fill < len(fills) {
				side := strings.ToLower(strings.TrimSpace(fills[s.fill].Side))
				s.fill++
				if side != "" {
					s.side = side
					return true
				}
			}
		}
		s.cur++
		s.fill = 0
		if s.cur >= len(s.items) {
			return false
		}
		s.inst = pgUUID(s.ids[s.cur])
	}
}

func (s *fillSource) Values() ([]any, error) {
	it := &s.items[s.cur]
	f := &it.Fills[s.fill-1]
	s.row = [11]any{s.pool.next(), s.exp, s.inst, nil, it.Symbol, f.TsMs, s.side, f.Qty, f.Price, 0.0, ""}
	return s.row[:], nil
}

func (s *fillSource) Err() error { return nil }

// outcomeSource streams the trade outcome rows of all completed items into COPY.
type outcomeSource struct {
	items []InstanceBatchItem
	ids   []uuid.UUID
	cur   int
	ord   int
	inst  pgtype.UUID
	row   [5]any
}

func (s *outcomeSource) Next() bool {
	for {
		if s.cur >= 0 && s.cur < len(s.items) && s.items[s.cur].Error == "" && s.ord < len(s.items[s.cur].Outcomes) {
			s.ord++
			return true
		}
		s.cur++
		s.ord = 0
		if s.cur >= len(s.items) {
			return false
		}
		s.inst = pgUUID(s.ids[s.cur])
	}
}

func (s *outcomeSource) Values() ([]any, error) {
	o := &s.items[s.cur].Outcomes[s.ord-1]
	var pct any
	if o.Percent != nil {
		pct = *o.Percent
	}
	s.row = [5]any{s.inst, int32(s.ord - 1), o.Side, o.Quote, pct}
	return s.row[:], nil
}

func (s *outcomeSource) Err() error { return nil }
