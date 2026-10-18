# Catalog test data

- `2022.json` to `2025.json`: JSON arrays built by concatenating `legacy/legacy/import/<year>/tricks/*.json` (sorted by filename). For 2022, `synchro_spiral.json` and `synchro_asymmetric_spiral.json` were dropped: they use the direction "mirror", which the legacy code could not import either (no acronym configured).
- `2026.json`: production catalog (non-deleted tricks of the mongo export's `tricks.json`, `_id` and `deleted` removed) including each trick's stored `tricks` variants, used as the expected output of `catalog.Generate`.
