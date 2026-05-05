package reader

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
)

type TelemetryRow struct {
	Timestamp  string
	MetricName string
	GPUID      string
	Device     string
	UUID       string
	ModelName  string
	Hostname   string
	Container  string
	Pod        string
	Namespace  string
	Value      string
	LabelsRaw  string
}

type CSVReader struct {
	reader *csv.Reader
	file   *os.File
}

func NewCSVReader(filePath string) (*CSVReader, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	r := csv.NewReader(f)
	r.ReuseRecord = true

	// Skip header
	if _, err := r.Read(); err != nil {
		f.Close()
		return nil, err
	}

	return &CSVReader{
		reader: r,
		file:   f,
	}, nil
}

func (r *CSVReader) Read() (*TelemetryRow, error) {
	record, err := r.reader.Read()
	if err != nil {
		return nil, err
	}

	if len(record) < 12 {
		return nil, fmt.Errorf("invalid record length: %d", len(record))
	}

	return &TelemetryRow{
		Timestamp:  record[0],
		MetricName: record[1],
		GPUID:      record[2],
		Device:     record[3],
		UUID:       record[4],
		ModelName:  record[5],
		Hostname:   record[6],
		Container:  record[7],
		Pod:        record[8],
		Namespace:  record[9],
		Value:      record[10],
		LabelsRaw:  record[11],
	}, nil
}

func (r *CSVReader) Close() error {
	return r.file.Close()
}
