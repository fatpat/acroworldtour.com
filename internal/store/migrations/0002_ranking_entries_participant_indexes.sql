-- Pilot and team pages look up every ranking entry of one participant.
CREATE INDEX ranking_entries_civlid ON ranking_entries (civlid);
CREATE INDEX ranking_entries_team_id ON ranking_entries (team_id);
