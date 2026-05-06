package questdb

// ScanPageForTest exposes the internal scanPage function for black-box tests.
func ScanPageForTest(rows pgRows, gpuID string, page, pageSize int) (*TelemetryPage, error) {
	return scanPage(rows, gpuID, page, pageSize)
}

// GetCreateTableSQL returns the DDL for test assertions.
func GetCreateTableSQL() string {
	return createTableSQL
}
