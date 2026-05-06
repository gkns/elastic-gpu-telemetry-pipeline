package telemetry

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"
)

// CSVParser reads TelemetryRecords from a DCGM metrics CSV file.
// The CSV timestamp column is ignored; ProcessedAt is set to time.Now().UTC()
// at the moment each row is read (per spec US3 acceptance criterion).
type CSVParser struct {
	f       *os.File
	r       *csv.Reader
	headers map[string]int
}

// NewCSVParser opens path and reads the header row.
func NewCSVParser(path string) (*CSVParser, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	r := csv.NewReader(f)
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	hdr, err := r.Read()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("read header: %w", err)
	}

	headers := make(map[string]int, len(hdr))
	for i, h := range hdr {
		headers[h] = i
	}

	return &CSVParser{f: f, r: r, headers: headers}, nil
}

// Next returns the next TelemetryRecord or io.EOF when done.
func (p *CSVParser) Next() (*TelemetryRecord, error) {
	row, err := p.r.Read()
	if err == io.EOF {
		return nil, io.EOF
	}
	if err != nil {
		return nil, fmt.Errorf("read row: %w", err)
	}

	get := func(col string) string {
		if idx, ok := p.headers[col]; ok && idx < len(row) {
			return row[idx]
		}
		return ""
	}

	valStr := get("value")
	val, _ := strconv.ParseFloat(valStr, 64)

	return &TelemetryRecord{
		ProcessedAt: time.Now().UTC(), // system time, not CSV timestamp (US3)
		MetricName:  get("metric_name"),
		GPUID:       get("gpu_id"),
		Device:      get("device"),
		UUID:        get("uuid"),
		ModelName:   get("modelName"),
		Hostname:    get("Hostname"),
		Container:   get("container"),
		Pod:         get("pod"),
		Namespace:   get("namespace"),
		Value:       val,
		LabelsRaw:   get("labels_raw"),
	}, nil
}

// Close closes the underlying file.
func (p *CSVParser) Close() error {
	return p.f.Close()
}
