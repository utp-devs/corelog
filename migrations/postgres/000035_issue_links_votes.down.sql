-- Votes are dropped with their counters reset; links of the kinds added in this migration are removed because the
-- previous kind set cannot represent them.
DROP TABLE IF EXISTS issue_votes;

UPDATE issues SET vote_count = 0 WHERE vote_count <> 0;

DELETE FROM issue_links WHERE kind NOT IN ('blocks', 'relates', 'duplicates');

ALTER TABLE issue_links DROP CONSTRAINT IF EXISTS issue_links_no_self_check;
ALTER TABLE issue_links DROP CONSTRAINT IF EXISTS issue_links_kind_check;
ALTER TABLE issue_links ADD CONSTRAINT issue_links_kind_check CHECK (kind IN ('blocks', 'relates', 'duplicates'));
