// Package scoring implements the AcroWorldTour scoring engine.
//
// The package is pure: no I/O, no database, no clock, no globals, no logging.
// It must reproduce the legacy Python engine's floats exactly (see
// specifications/03-scoring-rules.md). Score follows spec 03 §5.0 step by
// step; the code carries the same "step N" numbers.
//
// # JSON shape
//
// Every exported type has snake_case JSON tags; the golden corpus
// (testdata/golden) stores Input and Result in this form. An Input:
//
//	{
//	  "type": "solo",                      // or "synchro"
//	  "config": {
//	    "warning": 0.5, "warning2": 1, "malus_repetition": 13, "warnings_to_dsq": 3,
//	    "judge_weights": {"senior": 100, "certified": 100, "trainee": 20},
//	    "mark_percentages": {
//	      "solo": {"technical": 40, "choreography": 40, "landing": 20},
//	      "synchro": {"technical": 20, "choreography": 20, "landing": 20, "synchro": 40}
//	    },
//	    "max_bonus_per_run": {"twist": 5, "reverse": 3, "flip": 2}
//	  },
//	  "rules": {
//	    "max_tricks_per_type": {"tumbling": 2, "to infinite": 1},
//	    "tip_touch_pct_per_touch": 2, "tip_touch_pct_max": 6, "default_max_bonus": 10
//	  },
//	  "run_index": 1,
//	  "mark_type": "",                     // "", "awt" or "awq"
//	  "is_awt_pilot": true, "has_awt_season": true, "has_awq_season": false,
//	  "repeatable_base_tricks": ["Stall"],
//	  "reset_policies": [0, 1],            // runs 0..run_index: 0 none, 1 awt, 2 awq, 3 all
//	  "previous": [                        // same pilot/team, runs < run_index, ascending
//	    {"run_index": 0, "tricks": [<flown trick>...], "warnings1": 1, "warnings2": 0}
//	  ],
//	  "judge_levels": {"<judge id>": "senior"},
//	  "position_rules": {                  // by base trick name
//	    "Tumbling": {"first_maneuver": 0, "no_first_maneuver": 0, "last_maneuver": 0, "no_last_maneuver": 2}
//	  },
//	  "legacy_last_maneuver": false,
//	  "flight": {
//	    "did_not_start": false,
//	    "tricks": [<flown trick>...],
//	    "marks": [
//	      {"judge": "<judge id>", "technical": 7, "choreography": 6.5, "landing": 8,
//	       "synchro": 7, "technical_per_trick": [8, null, 7]}   // absent key = no mark
//	    ],
//	    "warnings1": ["late at briefing"], "warnings2": [],
//	    "tip_touches": 0
//	  }
//	}
//
// A flown trick is a catalog.UniqueTrick (flattened: name, acronym,
// technical_coefficient, bonus, bonus_types, base_trick, uniqueness, bonuses,
// synchro, solo, solo_awt, types) plus "technical_mark" (number or null) and
// "technical_marks" (object judge id -> number, or null). With classic
// marking a stored technical_mark still scales the bonus, as in legacy.
//
// A Result:
//
//	{
//	  "final": {
//	    "judges_mark": {"judge": "", "technical": 7.1, "choreography": 6.5, "landing": 8.1, "synchro": 0},
//	    "technicity": 1.73, "bonus_percentage": 5.5, "technical": 4.9, "choreography": 2.6,
//	    "landing": 1.6, "synchro": 0, "bonus": 0.4, "score": 9.1, "malus": 0,
//	    "warnings1": [...], "warnings2": [...], "notes": [...], "mark_type": ""
//	  },
//	  "tricks": [<flown trick>...],        // after the steps 4-5 computations
//	  "marks": [<judge mark>...]           // per-trick nulls filled with the plain mean
//	}
//
// Final numbers are full precision; Round3 them when persisting (spec 03 §9).
package scoring
