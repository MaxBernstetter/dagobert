package candlefile

import (
	"errors"
	"fmt"
	"io"
	"strconv"

	"github.com/parquet-go/parquet-go"

	"github.com/MaxBernstetter/dagobert/src/internal/parquetmeta"
)

// SchemaVersion is the footer schema_version of files written by this package.
const SchemaVersion = 2

// Row is one candle with its values as plain decimal strings.
type Row struct {
	OpenTime  int64
	Open      string
	High      string
	Low       string
	Close     string
	Volume    string
	CloseTime int64
}

// FloatRow is one candle with its values as float64.
type FloatRow struct {
	OpenTime  int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
	CloseTime int64
}

// diskRow is the on-disk layout. Column order is the field order.
type diskRow struct {
	OpenTime  int64             `parquet:"open_time,delta,snappy"`
	Open      int64             `parquet:"open,decimal(8:18),delta,snappy"`
	High      int64             `parquet:"high,decimal(8:18),delta,snappy"`
	Low       int64             `parquet:"low,decimal(8:18),delta,snappy"`
	Close     int64             `parquet:"close,decimal(8:18),delta,snappy"`
	Volume    [VolumeBytes]byte `parquet:"volume,decimal(8:28),snappy"`
	CloseTime int64             `parquet:"close_time,delta,snappy"`
}

// Write encodes rows in the current schema. It fails with a *ValueError, before
// writing anything, when a value has more than Scale decimals or does not fit
// its precision.
func Write(w io.Writer, rows []Row, meta parquetmeta.Meta) error {
	disk := make([]diskRow, len(rows))
	for i, r := range rows {
		var err error
		d := diskRow{OpenTime: r.OpenTime, CloseTime: r.CloseTime}
		if d.Open, err = EncodePrice("open", r.Open); err != nil {
			return err
		}
		if d.High, err = EncodePrice("high", r.High); err != nil {
			return err
		}
		if d.Low, err = EncodePrice("low", r.Low); err != nil {
			return err
		}
		if d.Close, err = EncodePrice("close", r.Close); err != nil {
			return err
		}
		if d.Volume, err = EncodeVolume("volume", r.Volume); err != nil {
			return err
		}
		disk[i] = d
	}
	meta.SchemaVersion = SchemaVersion
	return parquet.Write(w, disk, parquetmeta.WriterOptions(meta)...)
}

// IsLegacy reports whether the file stores the value columns as strings.
func IsLegacy(pf *parquet.File) bool {
	leaf, ok := pf.Schema().Lookup("open")
	return ok && leaf.Node.Type().Kind() == parquet.ByteArray
}

// Read loads every row as decimal strings from either schema. Decimal columns
// render with exactly Scale decimals; legacy strings are returned as stored.
func Read(pf *parquet.File) ([]Row, error) {
	var out []Row
	err := scan(pf, func(t0 int64, v [5]decoded, t1 int64) {
		out = append(out, Row{t0, v[0].str, v[1].str, v[2].str, v[3].str, v[4].str, t1})
	}, false)
	return out, err
}

// ReadFloat loads every row as float64 from either schema (or from the plain
// DOUBLE columns some test fixtures use). Decimal values convert to the float64
// nearest the exact decimal, equal to parsing the legacy string.
func ReadFloat(pf *parquet.File) ([]FloatRow, error) {
	var out []FloatRow
	err := scan(pf, func(t0 int64, v [5]decoded, t1 int64) {
		out = append(out, FloatRow{t0, v[0].f, v[1].f, v[2].f, v[3].f, v[4].f, t1})
	}, true)
	return out, err
}

type decoded struct {
	str string
	f   float64
}

var valueColumns = [5]string{"open", "high", "low", "close", "volume"}

// scan walks the rows once. wantFloat selects which field of decoded is filled.
func scan(pf *parquet.File, emit func(openTime int64, v [5]decoded, closeTime int64), wantFloat bool) error {
	schema := pf.Schema()
	idx := func(name string) (int, parquet.Node, error) {
		leaf, ok := schema.Lookup(name)
		if !ok {
			return 0, nil, fmt.Errorf("candle file has no %q column", name)
		}
		return leaf.ColumnIndex, leaf.Node, nil
	}
	openIdx, _, err := idx("open_time")
	if err != nil {
		return err
	}
	closeIdx, _, err := idx("close_time")
	if err != nil {
		return err
	}
	var cols [5]int
	var dec [5]func(parquet.Value) (decoded, error)
	for i, name := range valueColumns {
		ci, node, err := idx(name)
		if err != nil {
			return err
		}
		cols[i] = ci
		if dec[i], err = decoderFor(name, node, wantFloat); err != nil {
			return err
		}
	}

	reader := parquet.NewReader(pf)
	defer reader.Close()
	buf := make([]parquet.Row, 512)
	for {
		n, readErr := reader.ReadRows(buf)
		for _, row := range buf[:n] {
			var v [5]decoded
			for i := range v {
				if v[i], err = dec[i](row[cols[i]]); err != nil {
					return err
				}
			}
			emit(row[openIdx].Int64(), v, row[closeIdx].Int64())
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

func decoderFor(name string, node parquet.Node, wantFloat bool) (func(parquet.Value) (decoded, error), error) {
	typ := node.Type()
	scale := 0
	if lt := typ.LogicalType(); lt != nil && lt.Decimal != nil {
		scale = int(lt.Decimal.Scale)
	}
	isDecimal := typ.LogicalType() != nil && typ.LogicalType().Decimal != nil
	switch typ.Kind() {
	case parquet.ByteArray:
		return func(v parquet.Value) (decoded, error) {
			s := string(v.ByteArray())
			if !wantFloat {
				return decoded{str: s}, nil
			}
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return decoded{}, fmt.Errorf("candle %s value %q: %w", name, s, err)
			}
			return decoded{f: f}, nil
		}, nil
	case parquet.Double:
		return func(v parquet.Value) (decoded, error) {
			if wantFloat {
				return decoded{f: v.Double()}, nil
			}
			return decoded{str: strconv.FormatFloat(v.Double(), 'f', -1, 64)}, nil
		}, nil
	case parquet.Int64:
		if !isDecimal {
			break
		}
		return func(v parquet.Value) (decoded, error) {
			u := v.Int64()
			if wantFloat {
				return decoded{f: floatFromUnscaled(u, scale)}, nil
			}
			return decoded{str: formatScaled(strconv.FormatInt(u, 10), scale)}, nil
		}, nil
	case parquet.FixedLenByteArray:
		if !isDecimal {
			break
		}
		return func(v parquet.Value) (decoded, error) {
			b := v.ByteArray()
			u := bigFromVolume(b)
			if wantFloat && u.IsInt64() {
				return decoded{f: floatFromUnscaled(u.Int64(), scale)}, nil
			}
			s := formatScaled(u.String(), scale)
			if !wantFloat {
				return decoded{str: s}, nil
			}
			g, err := strconv.ParseFloat(s, 64)
			return decoded{f: g}, err
		}, nil
	}
	return nil, fmt.Errorf("candle column %q has unsupported type %s", name, typ)
}
