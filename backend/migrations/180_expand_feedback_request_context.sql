-- Request ID input now also accepts pasted error details.
DROP INDEX IF EXISTS feedbacks_request_id;
ALTER TABLE feedbacks ALTER COLUMN request_id TYPE TEXT;
