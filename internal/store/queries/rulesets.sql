-- name: GetRuleset :one
SELECT * FROM rulesets WHERE year = ?;

-- name: ListRulesets :many
SELECT * FROM rulesets ORDER BY year;

-- name: UpsertRuleset :exec
INSERT INTO rulesets (year, config, max_tricks_per_type, tip_touch_pct_per_touch, tip_touch_pct_max, default_max_bonus)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (year) DO UPDATE SET
    config = excluded.config,
    max_tricks_per_type = excluded.max_tricks_per_type,
    tip_touch_pct_per_touch = excluded.tip_touch_pct_per_touch,
    tip_touch_pct_max = excluded.tip_touch_pct_max,
    default_max_bonus = excluded.default_max_bonus;
