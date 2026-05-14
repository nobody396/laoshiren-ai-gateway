ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS gpt_image_call_price NUMERIC(20, 8);
