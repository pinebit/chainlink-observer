-- +goose Up
-- +goose StatementBegin
CREATE TABLE observer_specs (
    id              SERIAL PRIMARY KEY,
    addresses       bytea[] NOT NULL,
    events          text[] NOT NULL,
    interval        bigint NOT NULL,
    evm_chain_id    numeric(78,0) REFERENCES evm_chains (id),
    created_at      timestamp with time zone NOT NULL,
    updated_at      timestamp with time zone NOT NULL
);

ALTER TABLE jobs
    ADD COLUMN observer_spec_id INT REFERENCES observer_specs (id),
    DROP CONSTRAINT chk_only_one_spec,
    ADD CONSTRAINT chk_only_one_spec CHECK (
            num_nonnulls(
                    ocr_oracle_spec_id,
                    ocr2_oracle_spec_id,
                    direct_request_spec_id,
                    flux_monitor_spec_id,
                    keeper_spec_id,
                    cron_spec_id,
                    webhook_spec_id,
                    vrf_spec_id,
                    blockhash_store_spec_id,
                    block_header_feeder_spec_id,
                    bootstrap_spec_id,
                    observer_spec_id) = 1);
-- +goose StatementEnd

-- +goose Down

-- +goose StatementBegin
ALTER TABLE jobs
    DROP CONSTRAINT chk_only_one_spec,
    ADD CONSTRAINT chk_only_one_spec CHECK (
            num_nonnulls(
                    ocr_oracle_spec_id,
                    ocr2_oracle_spec_id,
                    direct_request_spec_id,
                    flux_monitor_spec_id,
                    keeper_spec_id,
                    cron_spec_id,
                    webhook_spec_id,
                    vrf_spec_id,
                    blockhash_store_spec_id,
                    block_header_feeder_spec_id,
                    bootstrap_spec_id) = 1);

ALTER TABLE jobs
    DROP COLUMN observer_spec_id;

DROP TABLE observer_specs;
-- +goose StatementEnd
