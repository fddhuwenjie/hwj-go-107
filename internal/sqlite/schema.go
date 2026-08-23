package sqlite

// schema 全量建表语句，使用 IF NOT EXISTS 保证幂等。
const schema = `
CREATE TABLE IF NOT EXISTS preservation_levels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS storage_units (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK(kind IN ('warehouse','showcase')),
    location TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled')),
    version INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS artifacts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    category TEXT NOT NULL DEFAULT '',
    era TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN ('registered','stored','isolated','frozen','out','on_loan','returned_pending','retired')),
    level_id INTEGER NOT NULL REFERENCES preservation_levels(id),
    storage_unit_id INTEGER REFERENCES storage_units(id),
    note TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    retired INTEGER NOT NULL DEFAULT 0,
    retired_reason TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_artifacts_status ON artifacts(status);
CREATE INDEX IF NOT EXISTS idx_artifacts_unit ON artifacts(storage_unit_id);

CREATE TABLE IF NOT EXISTS attachments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    artifact_id INTEGER NOT NULL REFERENCES artifacts(id),
    name TEXT NOT NULL,
    spec TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_attachments_artifact ON attachments(artifact_id);

CREATE TABLE IF NOT EXISTS threshold_rule_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    level_id INTEGER NOT NULL REFERENCES preservation_levels(id),
    version_no INTEGER NOT NULL,
    temp_min REAL NOT NULL,
    temp_max REAL NOT NULL,
    humidity_min REAL NOT NULL,
    humidity_max REAL NOT NULL,
    consecutive_breach INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','active','retired')),
    created_at INTEGER NOT NULL,
    activated_at INTEGER,
    UNIQUE(level_id, version_no)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_rule_active ON threshold_rule_versions(level_id) WHERE status='active';

CREATE TABLE IF NOT EXISTS sensors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    storage_unit_id INTEGER NOT NULL REFERENCES storage_units(id),
    kind TEXT NOT NULL DEFAULT 'th',
    status TEXT NOT NULL DEFAULT 'active' CHECK(status IN ('active','disabled')),
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sensors_unit ON sensors(storage_unit_id);

CREATE TABLE IF NOT EXISTS env_samples (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sensor_id INTEGER NOT NULL REFERENCES sensors(id),
    storage_unit_id INTEGER NOT NULL REFERENCES storage_units(id),
    temperature REAL NOT NULL,
    humidity REAL NOT NULL,
    sampled_at INTEGER NOT NULL,
    received_at INTEGER NOT NULL,
    late INTEGER NOT NULL DEFAULT 0,
    processed INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_samples_unit_time ON env_samples(storage_unit_id, sampled_at);
CREATE INDEX IF NOT EXISTS idx_samples_sensor_time ON env_samples(sensor_id, sampled_at);
CREATE INDEX IF NOT EXISTS idx_samples_processed ON env_samples(processed);

CREATE TABLE IF NOT EXISTS anomaly_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    storage_unit_id INTEGER NOT NULL REFERENCES storage_units(id),
    rule_version_id INTEGER NOT NULL REFERENCES threshold_rule_versions(id),
    sample_id INTEGER NOT NULL REFERENCES env_samples(id),
    severity TEXT NOT NULL CHECK(severity IN ('minor','major','critical')),
    status TEXT NOT NULL CHECK(status IN ('open','isolating','disposing','reviewing','closed')),
    breach_count INTEGER NOT NULL DEFAULT 1,
    title TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL DEFAULT 1,
    opened_at INTEGER NOT NULL,
    closed_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_anomaly_unit_status ON anomaly_events(storage_unit_id, status);

CREATE TABLE IF NOT EXISTS protection_actions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id INTEGER NOT NULL REFERENCES anomaly_events(id),
    action_type TEXT NOT NULL,
    operator TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN ('pending','done','review_pass','review_reject')),
    reviewed_by TEXT NOT NULL DEFAULT '',
    reviewed_at INTEGER,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_actions_event ON protection_actions(event_id);

CREATE TABLE IF NOT EXISTS loan_applications (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL UNIQUE,
    borrower TEXT NOT NULL,
    venue TEXT NOT NULL DEFAULT '',
    purpose TEXT NOT NULL DEFAULT '',
    start_at INTEGER NOT NULL,
    end_at INTEGER NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('draft','submitted','approved','in_transit','exhibiting','returned','closed','rejected','cancelled')),
    rule_snapshot TEXT NOT NULL DEFAULT '',
    approved_by TEXT NOT NULL DEFAULT '',
    approved_at INTEGER,
    reject_reason TEXT NOT NULL DEFAULT '',
    overdue INTEGER NOT NULL DEFAULT 0,
    attention INTEGER NOT NULL DEFAULT 0,
    version INTEGER NOT NULL DEFAULT 1,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_loans_status ON loan_applications(status);

CREATE TABLE IF NOT EXISTS loan_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    loan_id INTEGER NOT NULL REFERENCES loan_applications(id),
    artifact_id INTEGER NOT NULL REFERENCES artifacts(id),
    frozen_status TEXT NOT NULL DEFAULT '',
    frozen_level_id INTEGER NOT NULL DEFAULT 0,
    frozen_unit_id INTEGER NOT NULL DEFAULT 0,
    packaging_snapshot TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    UNIQUE(loan_id, artifact_id)
);

CREATE TABLE IF NOT EXISTS inventory_checks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    loan_id INTEGER NOT NULL REFERENCES loan_applications(id),
    direction TEXT NOT NULL CHECK(direction IN ('out','in')),
    idempotency_key TEXT NOT NULL UNIQUE,
    operator TEXT NOT NULL,
    complete INTEGER NOT NULL DEFAULT 0,
    checked_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_checks_loan ON inventory_checks(loan_id);

CREATE TABLE IF NOT EXISTS inventory_check_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    check_id INTEGER NOT NULL REFERENCES inventory_checks(id),
    artifact_id INTEGER NOT NULL,
    attachment_id INTEGER NOT NULL DEFAULT 0,
    present INTEGER NOT NULL DEFAULT 0,
    note TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_check_items_check ON inventory_check_items(check_id);

CREATE TABLE IF NOT EXISTS package_handovers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    loan_id INTEGER NOT NULL REFERENCES loan_applications(id),
    seq INTEGER NOT NULL,
    from_person TEXT NOT NULL,
    to_person TEXT NOT NULL,
    handed_at INTEGER NOT NULL,
    location TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL,
    UNIQUE(loan_id, seq)
);

CREATE TABLE IF NOT EXISTS transport_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    loan_id INTEGER NOT NULL REFERENCES loan_applications(id),
    seq INTEGER NOT NULL,
    node_type TEXT NOT NULL CHECK(node_type IN ('departure','transit','arrival')),
    location TEXT NOT NULL DEFAULT '',
    occurred_at INTEGER NOT NULL,
    recorded_by TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    UNIQUE(loan_id, seq)
);

CREATE TABLE IF NOT EXISTS exhibition_confirms (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    loan_id INTEGER NOT NULL REFERENCES loan_applications(id),
    showcase_id INTEGER NOT NULL REFERENCES storage_units(id),
    confirmed_by TEXT NOT NULL,
    confirmed_at INTEGER NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS return_acceptances (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    loan_id INTEGER NOT NULL REFERENCES loan_applications(id),
    check_id INTEGER NOT NULL REFERENCES inventory_checks(id),
    result TEXT NOT NULL CHECK(result IN ('pass','pass_with_notes','rejected')),
    reviewer TEXT NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    reviewed_at INTEGER NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS artifact_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    artifact_id INTEGER NOT NULL REFERENCES artifacts(id),
    status TEXT NOT NULL,
    level_id INTEGER NOT NULL,
    storage_unit_id INTEGER,
    note TEXT NOT NULL DEFAULT '',
    version INTEGER NOT NULL,
    reason TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_snapshots_artifact ON artifact_snapshots(artifact_id);

CREATE TABLE IF NOT EXISTS audit_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    entity_type TEXT NOT NULL,
    entity_id INTEGER NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_logs(entity_type, entity_id);

CREATE TABLE IF NOT EXISTS jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    kind TEXT NOT NULL,
    payload TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK(status IN ('pending','running','done','failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 5,
    run_at INTEGER NOT NULL,
    last_error TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_jobs_status_run ON jobs(status, run_at);
`
