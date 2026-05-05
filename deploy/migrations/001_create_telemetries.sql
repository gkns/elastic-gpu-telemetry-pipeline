CREATE TABLE telemetries (
    timestamp TIMESTAMP,
    metric_name SYMBOL,
    gpu_id SYMBOL,
    device SYMBOL,
    uuid SYMBOL,
    model_name SYMBOL,
    hostname SYMBOL,
    value DOUBLE,
    labels_raw STRING
) timestamp(timestamp) PARTITION BY DAY WAL;
