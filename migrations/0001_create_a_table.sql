-- +goose Up
-- +goose StatementBegin
CREATE TABLE metrics (
    id VARCHAR(255) NOT NULL,
    m_type VARCHAR(50) NOT NULL CHECK (m_type IN ('counter', 'gauge')),
    delta BIGINT NULL,
    value DOUBLE PRECISION NULL,
    hash VARCHAR(255) NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (id)
);

CREATE INDEX idx_metrics_id ON metrics(id);
CREATE INDEX idx_metrics_type ON metrics(m_type);
CREATE INDEX idx_metrics_hash ON metrics(hash);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_metrics_hash;
DROP INDEX IF EXISTS idx_metrics_type;
DROP INDEX IF EXISTS idx_metrics_id;

DROP TABLE IF EXISTS metrics;
-- +goose StatementEnd