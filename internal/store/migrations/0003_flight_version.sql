-- Optimistic concurrency of the judging screen: version counts the saves of
-- a flight's input (rescoring does not change it). A save carries the
-- version it was edited from and is refused when it no longer matches.
ALTER TABLE flights ADD COLUMN version INTEGER NOT NULL DEFAULT 1;
