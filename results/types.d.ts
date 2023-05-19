export interface components {
  schemas: {
    /** Body_login_auth_login_post */
    Body_login_auth_login_post: {
      /** Grant Type */
      grant_type?: string;
      /** Username */
      username: string;
      /** Password */
      password: string;
      /**
       * Scope
       * @default
       */
      scope?: string;
      /** Client Id */
      client_id?: string;
      /** Client Secret */
      client_secret?: string;
    };
    /** Body_post_file_files_new_post */
    Body_post_file_files_new_post: {
      /**
       * File
       * Format: binary
       */
      file: string;
    };
    /**
     * Bonus
     * @example {
     *   "name": "twisted",
     *   "bonus": 2.5
     * }
     */
    Bonus: {
      /** Name */
      name: string;
      /** Bonus */
      bonus: number;
    };
    /**
     * CompetitionConfig
     * @example {
     *   "warning": 0.5,
     *   "malus_repetition": 13,
     *   "warnings_to_dsq": 3,
     *   "judges_weight": {
     *     "senior": 100,
     *     "certified": 100,
     *     "trainee": 20
     *   },
     *   "mark_percentages": {
     *     "solo": {
     *       "technical": 40,
     *       "choreography": 40,
     *       "landing": 20
     *     },
     *     "synchro": {
     *       "technical": 25,
     *       "choreography": 25,
     *       "landing": 25,
     *       "synchro": 25
     *     }
     *   },
     *   "max_bonus_per_run": {
     *     "twist": 5,
     *     "reverse": 3,
     *     "flip": 1
     *   }
     * }
     */
    CompetitionConfig: {
      /**
       * Warning
       * @description The point deduction for a warning
       * @default 0.5
       */
      warning?: number;
      /**
       * Malus Repetition
       * @description % reduction malus of choreography for repetition
       * @default 13
       */
      malus_repetition?: number;
      /**
       * Warnings To Dsq
       * @description number of warnings in a comp that lead to DSQ
       * @default 3
       */
      warnings_to_dsq?: number;
      /**
       * Judge Weights
       * @default {
       *   "senior": 100,
       *   "certified": 100,
       *   "trainee": 20
       * }
       */
      judge_weights?: components["schemas"]["JudgeWeights"];
      /**
       * Mark Percentages
       * @default {
       *   "solo": {
       *     "technical": 40,
       *     "choreography": 40,
       *     "landing": 20
       *   },
       *   "synchro": {
       *     "technical": 20,
       *     "choreography": 20,
       *     "landing": 20,
       *     "synchro": 40
       *   }
       * }
       */
      mark_percentages?: components["schemas"]["MarkPercentages"];
      /**
       * Max Bonus Per Run
       * @default {
       *   "twist": 5,
       *   "reverse": 3,
       *   "flip": 2
       * }
       */
      max_bonus_per_run?: components["schemas"]["MaxBonusPerRun"];
    };
    /** CompetitionExport */
    CompetitionExport: {
      /** Id */
      _id: string;
      /** Name */
      name: string;
      /** Code */
      code: string;
      /**
       * Start Date
       * Format: date
       */
      start_date: string;
      /**
       * End Date
       * Format: date
       */
      end_date: string;
      /** Location */
      location: string;
      /** Published */
      published: boolean;
      type: components["schemas"]["CompetitionType"];
      /** Pilots */
      pilots: components["schemas"]["Pilot"][];
      /** Teams */
      teams: components["schemas"]["TeamExport"][];
      /** Judges */
      judges: components["schemas"]["Judge"][];
      /** Repeatable Tricks */
      repeatable_tricks: components["schemas"]["Trick"][];
      state: components["schemas"]["CompetitionState"];
      config: components["schemas"]["CompetitionConfig"];
      /** Runs */
      runs: components["schemas"]["RunExport"][];
      /**
       * Image
       * Format: uri
       */
      image?: string;
      /** Seasons */
      seasons: string[];
    };
    /** CompetitionNew */
    CompetitionNew: {
      /** Name */
      name: string;
      /** Code */
      code?: string;
      /**
       * Start Date
       * Format: date
       */
      start_date: string;
      /**
       * End Date
       * Format: date
       */
      end_date: string;
      /** Location */
      location: string;
      /** Published */
      published: boolean;
      type: components["schemas"]["CompetitionType"];
      /** Image */
      image?: string;
      /**
       * Seasons
       * @default []
       */
      seasons?: string[];
    };
    /** CompetitionPilotResultsExport */
    CompetitionPilotResultsExport: {
      pilot?: components["schemas"]["Pilot"];
      team?: components["schemas"]["TeamExport"];
      /** Result Per Run */
      result_per_run: components["schemas"]["RunResultSummary"][];
      /** Score */
      score: number;
    };
    /** CompetitionPublicExport */
    CompetitionPublicExport: {
      /** Id */
      _id: string;
      /** Name */
      name: string;
      /** Code */
      code: string;
      /**
       * Start Date
       * Format: date
       */
      start_date: string;
      /**
       * End Date
       * Format: date
       */
      end_date: string;
      /** Location */
      location: string;
      /** Published */
      published: boolean;
      type: components["schemas"]["CompetitionType"];
      state: components["schemas"]["CompetitionState"];
      /** Number Of Pilots */
      number_of_pilots: number;
      /** Number Of Teams */
      number_of_teams: number;
      /** Number Of Judges */
      number_of_judges: number;
      /** Number Of Runs */
      number_of_runs: number;
      /**
       * Image
       * Format: uri
       */
      image?: string;
      /** Seasons */
      seasons: string[];
    };
    /** CompetitionPublicExportWithResults */
    CompetitionPublicExportWithResults: {
      /** Id */
      _id: string;
      /** Name */
      name: string;
      /** Code */
      code: string;
      /**
       * Start Date
       * Format: date
       */
      start_date: string;
      /**
       * End Date
       * Format: date
       */
      end_date: string;
      /** Location */
      location: string;
      /** Published */
      published: boolean;
      type: components["schemas"]["CompetitionType"];
      state: components["schemas"]["CompetitionState"];
      /** Number Of Pilots */
      number_of_pilots: number;
      /** Number Of Teams */
      number_of_teams: number;
      /** Number Of Judges */
      number_of_judges: number;
      /** Number Of Runs */
      number_of_runs: number;
      /**
       * Image
       * Format: uri
       */
      image?: string;
      /** Seasons */
      seasons: string[];
      results: components["schemas"]["CompetitionResultsExport"];
      /** Pilots */
      pilots: components["schemas"]["Pilot"][];
      /** Teams */
      teams: components["schemas"]["TeamExport"][];
      /** Judges */
      judges: components["schemas"]["Judge"][];
    };
    /** CompetitionResult */
    CompetitionResult: {
      competition: components["schemas"]["CompetitionPublicExport"];
      /** Rank */
      rank: number;
    };
    /** CompetitionResultsExport */
    CompetitionResultsExport: {
      /** Final */
      final: boolean;
      /** Type */
      type: string;
      /** Overall Results */
      overall_results: components["schemas"]["CompetitionPilotResultsExport"][];
      /** Runs Results */
      runs_results: components["schemas"]["RunResultsExport"][];
    };
    /**
     * CompetitionState
     * @description An enumeration.
     * @enum {string}
     */
    CompetitionState: "init" | "open" | "closed";
    /**
     * CompetitionType
     * @description An enumeration.
     * @enum {string}
     */
    CompetitionType: "solo" | "synchro";
    /** FileID */
    FileID: {
      /** Id */
      id: string;
    };
    /**
     * FinalMark
     * @example {
     *   "judges_mark": {
     *     "judge": "Average of the judges marks",
     *     "technical": 2.5,
     *     "choreography": 7,
     *     "landing": 7,
     *     "synchro": 7
     *   },
     *   "technicity": 1.87,
     *   "bonus_percentage": 23,
     *   "technical": 7,
     *   "choreography": 6,
     *   "landing": 7,
     *   "synchro": 7,
     *   "bonus": 1.23,
     *   "score": 9.244,
     *   "warnings": [
     *     "box",
     *     "late at briefing"
     *   ],
     *   "malus": 13,
     *   "notes": [
     *     "Yellow card: big ear to start the run"
     *   ]
     * }
     */
    FinalMark: {
      judges_mark: components["schemas"]["JudgeMark"];
      /** Technicity */
      technicity: number;
      /** Bonus Percentage */
      bonus_percentage: number;
      /** Technical */
      technical: number;
      /** Choreography */
      choreography: number;
      /** Landing */
      landing: number;
      /** Synchro */
      synchro: number;
      /** Bonus */
      bonus: number;
      /** Score */
      score: number;
      /** Warnings */
      warnings: string[];
      /** Malus */
      malus: number;
      /**
       * Notes
       * @default []
       */
      notes?: string[];
    };
    /** FinalMarkExport */
    FinalMarkExport: {
      judges_mark: components["schemas"]["JudgeMarkExport"];
      /** Technicity */
      technicity: number;
      /** Bonus Percentage */
      bonus_percentage: number;
      /** Technical */
      technical: number;
      /** Choreography */
      choreography: number;
      /** Landing */
      landing: number;
      /** Synchro */
      synchro: number;
      /** Bonus */
      bonus: number;
      /** Score */
      score: number;
      /** Warnings */
      warnings: string[];
      /** Malus */
      malus: number;
      /** Notes */
      notes: string[];
    };
    /**
     * Flight
     * @example {
     *   "pilot": 1234,
     *   "tricks": [],
     *   "marks": [],
     *   "did_not_start": false,
     *   "final_marks": {},
     *   "published": false,
     *   "warnings": []
     * }
     */
    Flight: {
      /** Pilot */
      pilot: number;
      /** Team */
      team?: string;
      /** Tricks */
      tricks: components["schemas"]["UniqueTrick"][];
      /** Marks */
      marks: components["schemas"]["JudgeMark"][];
      /**
       * Did Not Start
       * @default false
       */
      did_not_start?: boolean;
      final_marks?: components["schemas"]["FinalMark"];
      /**
       * Published
       * @default false
       */
      published?: boolean;
      /** Warnings */
      warnings: string[];
    };
    /** FlightExport */
    FlightExport: {
      pilot?: components["schemas"]["Pilot"];
      team?: components["schemas"]["TeamExport"];
      /** Tricks */
      tricks: components["schemas"]["UniqueTrick"][];
      /** Marks */
      marks: components["schemas"]["JudgeMarkExport"][];
      /**
       * Did Not Start
       * @default false
       */
      did_not_start?: boolean;
      final_marks?: components["schemas"]["FinalMarkExport"];
      /**
       * Published
       * @default false
       */
      published?: boolean;
      /** Warnings */
      warnings: string[];
    };
    /**
     * FlightNew
     * @example {
     *   "tricks": [
     *     "LM",
     *     "Right Misty Flip"
     *   ],
     *   "marks": [],
     *   "did_not_start": false,
     *   "warnings": []
     * }
     */
    FlightNew: {
      /** Tricks */
      tricks: string[];
      /** Marks */
      marks: components["schemas"]["JudgeMark"][];
      /**
       * Did Not Start
       * @default false
       */
      did_not_start?: boolean;
      /**
       * Warnings
       * @default []
       */
      warnings?: string[];
    };
    /**
     * GenderEnum
     * @description An enumeration.
     * @enum {string}
     */
    GenderEnum: "man" | "woman" | "none";
    /** HTTPValidationError */
    HTTPValidationError: {
      /** Detail */
      detail?: components["schemas"]["ValidationError"][];
    };
    /**
     * Judge
     * @example {
     *   "name": "Jerry The Judge",
     *   "country": "fra",
     *   "level": "certified",
     *   "civlid": 1234
     * }
     */
    Judge: {
      /** Id */
      _id?: string;
      /**
       * Name
       * @description The full name of the judge
       */
      name: string;
      /**
       * Country
       * @description The country of the judge using the 3 letter acronym of the country
       */
      country: string;
      /** @description The level of the judge */
      level: components["schemas"]["JudgeLevel"];
      /**
       * Civlid
       * @description The CIVL ID if any (must be registered in the pilot database
       */
      civlid?: number;
      /**
       * Deleted
       * Format: date-time
       */
      deleted?: string;
    };
    /**
     * JudgeLevel
     * @description An enumeration.
     * @enum {unknown}
     */
    JudgeLevel: "trainee" | "certified" | "senior";
    /**
     * JudgeMark
     * @example {
     *   "judge": "Jerry The Judge",
     *   "technical": 2.5,
     *   "choreography": 7,
     *   "landing": 7,
     *   "synchro": 7
     * }
     */
    JudgeMark: {
      /** Judge */
      judge: string;
      /** Technical */
      technical?: number;
      /** Choreography */
      choreography?: number;
      /** Landing */
      landing?: number;
      /** Synchro */
      synchro?: number;
    };
    /** JudgeMarkExport */
    JudgeMarkExport: {
      judge?: components["schemas"]["Judge"];
      /** Technical */
      technical?: number;
      /** Choreography */
      choreography?: number;
      /** Landing */
      landing?: number;
      /** Synchro */
      synchro?: number;
    };
    /**
     * JudgeWeights
     * @example {
     *   "senior": 100,
     *   "certified": 100,
     *   "trainee": 20
     * }
     */
    JudgeWeights: {
      /**
       * Senior
       * @description weight of a senior judge's mark
       * @default 100
       */
      senior?: number;
      /**
       * Certified
       * @description weight of a certified judge's mark
       * @default 100
       */
      certified?: number;
      /**
       * Trainee
       * @description weight of a trainee judge's mark
       * @default 20
       */
      trainee?: number;
    };
    /** Link */
    Link: {
      /** Name */
      name: string;
      /**
       * Link
       * Format: uri
       */
      link: string;
    };
    /**
     * MarkPercentageSolo
     * @example {
     *   "technical": 40,
     *   "choreography": 40,
     *   "landing": 20
     * }
     */
    MarkPercentageSolo: {
      /**
       * Technical
       * @description % of the technical part in the final score for solo runs
       * @default 40
       */
      technical?: number;
      /**
       * Choreography
       * @description % of the choreography part in the final score for solo runs
       * @default 40
       */
      choreography?: number;
      /**
       * Landing
       * @description % of the landing part in the final score for solo runs
       * @default 20
       */
      landing?: number;
    };
    /**
     * MarkPercentageSynchro
     * @example {
     *   "technical": 25,
     *   "choreography": 25,
     *   "landing": 25,
     *   "synchro": 25
     * }
     */
    MarkPercentageSynchro: {
      /**
       * Technical
       * @description % of the technical part in the final score for synchro runs
       * @default 20
       */
      technical?: number;
      /**
       * Choreography
       * @description % of the choreography part in the final score for synchro runs
       * @default 20
       */
      choreography?: number;
      /**
       * Landing
       * @description % of the landing part in the final score for synchro runs
       * @default 20
       */
      landing?: number;
      /**
       * Synchro
       * @description % of the synchro part in the final score for synchro runs
       * @default 40
       */
      synchro?: number;
    };
    /**
     * MarkPercentages
     * @example {
     *   "solo": {
     *     "technical": 40,
     *     "choreography": 40,
     *     "landing": 20
     *   },
     *   "synchro": {
     *     "technical": 25,
     *     "choreography": 25,
     *     "landing": 25,
     *     "synchro": 25
     *   }
     * }
     */
    MarkPercentages: {
      /**
       * Solo
       * @default {
       *   "technical": 40,
       *   "choreography": 40,
       *   "landing": 20
       * }
       */
      solo?: components["schemas"]["MarkPercentageSolo"];
      /**
       * Synchro
       * @default {
       *   "technical": 20,
       *   "choreography": 20,
       *   "landing": 20,
       *   "synchro": 40
       * }
       */
      synchro?: components["schemas"]["MarkPercentageSynchro"];
    };
    /**
     * MaxBonusPerRun
     * @example {
     *   "twist": 5,
     *   "reverse": 3,
     *   "flip": 1
     * }
     */
    MaxBonusPerRun: {
      /**
       * Twist
       * @description maximum number of twisted tricks per run
       * @default 5
       */
      twist?: number;
      /**
       * Reverse
       * @description maximum number of reverse tricks per run
       * @default 3
       */
      reverse?: number;
      /**
       * Flip
       * @description maximum number of flip tricks per run
       * @default 2
       */
      flip?: number;
    };
    /**
     * Pilot
     * @example {
     *   "civlid": 67619,
     *   "name": "Luke de Weert",
     *   "civl_link": "https://civlcomps.org/pilot/67619",
     *   "country": "nld",
     *   "about": "\"I am an athlete who believes that dedication is the core of the thing that keeps me pushing and motivating me to achieve all my goals, and even set new goals where I never thought it was possible.\"",
     *   "social_links": [
     *     {
     *       "name": "facebook",
     *       "link": "https://www.facebook.com/deweert.luke"
     *     },
     *     {
     *       "name": "instagram",
     *       "link": "https://www.instagram.com/luke_deweert/"
     *     },
     *     {
     *       "name": "twitter",
     *       "link": "https://twitter.com/luke_deweert"
     *     },
     *     {
     *       "name": "youtube",
     *       "link": "https://www.youtube.com/lukedeweert"
     *     },
     *     {
     *       "name": "Website",
     *       "link": "https://lukedeweert.nl"
     *     },
     *     {
     *       "name": "Tiktok",
     *       "link": "https://www.tiktok.com/@lukedeweert"
     *     }
     *   ],
     *   "sponsors": [
     *     {
     *       "name": "Sky Paragliders",
     *       "link": "https://sky-cz.com/en",
     *       "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/4cbe1ebac175a9cde7a4c9d8769ba0c4/509e4e83c097d02828403b5a67e8c0b5.png"
     *     },
     *     {
     *       "name": "Sinner",
     *       "link": "https://www.sinner.eu/nl/",
     *       "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/dddccfa819ee01d9b2410ba49fa432fc/eeff42d05ffefb8ef945dc83485007ea.png"
     *     },
     *     {
     *       "name": "Wanbound",
     *       "link": "https://www.wanbound.com/",
     *       "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/aa675f347b7d7933332df96f08b21199/4ff22ae0404446f203ba682751e1e7b8.png"
     *     },
     *     {
     *       "name": "KNVvL",
     *       "link": "https://www.knvvl.nl/",
     *       "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/53ee05f2c2172541b7f1dd99e67a59f9/0f68789e476c0494019a750a6da9c6aa.png"
     *     }
     *   ],
     *   "photo": "https://civlcomps.org/uploads/images/profile/676/7bdecbee5d2246b1ebc14248dc1af935/8bfbe7e62a481a19145c55c9dc97e6ab.jpeg",
     *   "background_picture": "https://civlcomps.org/uploads/images/pilot_header/9/c017697641aa9ef817c4c17728e9e6d6/08788da048eea61f93be8591e97f6a0c.jpg",
     *   "last_update": "2022-06-03T19:05:59.325692",
     *   "rank": 2
     * }
     */
    Pilot: {
      /** Id */
      _id: number;
      /**
       * Civlid
       * @description The CIVL ID of the pilot
       */
      civlid: number;
      /**
       * Name
       * @description The complete name of the pilot
       */
      name: string;
      /**
       * Civl Link
       * Format: uri
       * @description The link to the CIVL pilot page
       */
      civl_link: string;
      /**
       * Country
       * @description The country of the pilot
       */
      country: string;
      /**
       * About
       * @description About text of the pilot
       */
      about: string;
      /**
       * Social Links
       * @description List of pilot's links (socials medias, ...)
       */
      social_links: components["schemas"]["Link"][];
      /**
       * Sponsors
       * @description List of the pilot's sponsors
       */
      sponsors: components["schemas"]["Sponsor"][];
      /**
       * Photo
       * Format: uri
       * @description Link to the profile image of the pilot
       */
      photo: string;
      /**
       * Background Picture
       * Format: uri
       * @description Link to the background profile image of the pilot
       */
      background_picture: string;
      /**
       * Last Update
       * Format: date-time
       * @description Last time the pilot has been updated
       */
      last_update?: string;
      /**
       * Rank
       * @description Current pilot's ranking in the aerobatic solo overwall world ranking
       */
      rank: number;
      /**
       * @description Pilot's sex
       * @default man
       */
      gender?: components["schemas"]["GenderEnum"];
    };
    /**
     * PilotWithResults
     * @example {
     *   "civlid": 67619,
     *   "name": "Luke de Weert",
     *   "civl_link": "https://civlcomps.org/pilot/67619",
     *   "country": "nld",
     *   "about": "\"I am an athlete who believes that dedication is the core of the thing that keeps me pushing and motivating me to achieve all my goals, and even set new goals where I never thought it was possible.\"",
     *   "social_links": [
     *     {
     *       "name": "facebook",
     *       "link": "https://www.facebook.com/deweert.luke"
     *     },
     *     {
     *       "name": "instagram",
     *       "link": "https://www.instagram.com/luke_deweert/"
     *     },
     *     {
     *       "name": "twitter",
     *       "link": "https://twitter.com/luke_deweert"
     *     },
     *     {
     *       "name": "youtube",
     *       "link": "https://www.youtube.com/lukedeweert"
     *     },
     *     {
     *       "name": "Website",
     *       "link": "https://lukedeweert.nl"
     *     },
     *     {
     *       "name": "Tiktok",
     *       "link": "https://www.tiktok.com/@lukedeweert"
     *     }
     *   ],
     *   "sponsors": [
     *     {
     *       "name": "Sky Paragliders",
     *       "link": "https://sky-cz.com/en",
     *       "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/4cbe1ebac175a9cde7a4c9d8769ba0c4/509e4e83c097d02828403b5a67e8c0b5.png"
     *     },
     *     {
     *       "name": "Sinner",
     *       "link": "https://www.sinner.eu/nl/",
     *       "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/dddccfa819ee01d9b2410ba49fa432fc/eeff42d05ffefb8ef945dc83485007ea.png"
     *     },
     *     {
     *       "name": "Wanbound",
     *       "link": "https://www.wanbound.com/",
     *       "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/aa675f347b7d7933332df96f08b21199/4ff22ae0404446f203ba682751e1e7b8.png"
     *     },
     *     {
     *       "name": "KNVvL",
     *       "link": "https://www.knvvl.nl/",
     *       "img": "https://civlcomps.org/uploads/images/ems_event_sponsor_logo/1/53ee05f2c2172541b7f1dd99e67a59f9/0f68789e476c0494019a750a6da9c6aa.png"
     *     }
     *   ],
     *   "photo": "https://civlcomps.org/uploads/images/profile/676/7bdecbee5d2246b1ebc14248dc1af935/8bfbe7e62a481a19145c55c9dc97e6ab.jpeg",
     *   "background_picture": "https://civlcomps.org/uploads/images/pilot_header/9/c017697641aa9ef817c4c17728e9e6d6/08788da048eea61f93be8591e97f6a0c.jpg",
     *   "last_update": "2022-06-03T19:05:59.325692",
     *   "rank": 2
     * }
     */
    PilotWithResults: {
      /** Id */
      _id: number;
      /**
       * Civlid
       * @description The CIVL ID of the pilot
       */
      civlid: number;
      /**
       * Name
       * @description The complete name of the pilot
       */
      name: string;
      /**
       * Civl Link
       * Format: uri
       * @description The link to the CIVL pilot page
       */
      civl_link: string;
      /**
       * Country
       * @description The country of the pilot
       */
      country: string;
      /**
       * About
       * @description About text of the pilot
       */
      about: string;
      /**
       * Social Links
       * @description List of pilot's links (socials medias, ...)
       */
      social_links: components["schemas"]["Link"][];
      /**
       * Sponsors
       * @description List of the pilot's sponsors
       */
      sponsors: components["schemas"]["Sponsor"][];
      /**
       * Photo
       * Format: uri
       * @description Link to the profile image of the pilot
       */
      photo: string;
      /**
       * Background Picture
       * Format: uri
       * @description Link to the background profile image of the pilot
       */
      background_picture: string;
      /**
       * Last Update
       * Format: date-time
       * @description Last time the pilot has been updated
       */
      last_update?: string;
      /**
       * Rank
       * @description Current pilot's ranking in the aerobatic solo overwall world ranking
       */
      rank: number;
      /**
       * @description Pilot's sex
       * @default man
       */
      gender?: components["schemas"]["GenderEnum"];
      /**
       * Competitions Results
       * @description List of competitions results
       * @default []
       */
      competitions_results?: components["schemas"]["CompetitionResult"][];
      /**
       * Seasons Results
       * @description List of seasons results
       * @default []
       */
      seasons_results?: components["schemas"]["models__pilots_with_results__SeasonResult"][];
    };
    /** RunExport */
    RunExport: {
      state: components["schemas"]["RunState"];
      /** Pilots */
      pilots: components["schemas"]["Pilot"][];
      /** Teams */
      teams: components["schemas"]["TeamExport"][];
      /** Judges */
      judges: components["schemas"]["Judge"][];
      /** Repeatable Tricks */
      repeatable_tricks: components["schemas"]["Trick"][];
      config: components["schemas"]["CompetitionConfig"];
      /** Flights */
      flights: components["schemas"]["FlightExport"][];
    };
    /**
     * RunResultSummary
     * @example {
     *   "rank": "1",
     *   "score": 12.5
     * }
     */
    RunResultSummary: {
      /** Rank */
      rank: number;
      /** Score */
      score: number;
    };
    /** RunResultsExport */
    RunResultsExport: {
      /** Final */
      final: boolean;
      /** Type */
      type: string;
      /** Results */
      results: components["schemas"]["FlightExport"][];
    };
    /**
     * RunState
     * @description An enumeration.
     * @enum {string}
     */
    RunState: "init" | "open" | "closed";
    /**
     * Season
     * @example {
     *   "name": "Acro World Tour 2022",
     *   "code": "awt-2022"
     * }
     */
    Season: {
      /** Id */
      _id?: string;
      /**
       * Name
       * @description The name of the season
       */
      name: string;
      /**
       * Code
       * @description The short code of the season
       */
      code: string;
      /**
       * Year
       * @description The year of the season
       */
      year: number;
      /** Image */
      image?: string;
      /**
       * Image Url
       * Format: uri
       */
      image_url?: string;
      /**
       * Deleted
       * Format: date-time
       */
      deleted?: string;
    };
    /** SeasonExport */
    SeasonExport: {
      /** Id */
      _id: string;
      /** Name */
      name: string;
      /** Code */
      code: string;
      /** Year */
      year: number;
      /**
       * Image
       * Format: uri
       */
      image?: string;
      type: components["schemas"]["CompetitionType"];
      /** Number Of Pilots */
      number_of_pilots: number;
      /** Number Of Teams */
      number_of_teams: number;
      /** Competitions */
      competitions: components["schemas"]["CompetitionPublicExportWithResults"][];
      /** Results */
      results: components["schemas"]["SeasonResults"][];
    };
    /** SeasonResults */
    SeasonResults: {
      /** Type */
      type: string;
      /** Results */
      results: components["schemas"]["models__seasons__SeasonResult"][];
    };
    /** Sponsor */
    Sponsor: {
      /** Name */
      name: string;
      /**
       * Link
       * Format: uri
       */
      link: string;
      /** Img */
      img: string;
    };
    /**
     * Status
     * @example {
     *   "project": "Acropyx2",
     *   "version": "2.0.1"
     * }
     */
    Status: {
      /** Project */
      project: string;
      /** Version */
      version: string;
    };
    /**
     * Team
     * @example {
     *   "_id": "687687687687aze",
     *   "name": "Team Rocket",
     *   "pilots": [
     *     1234,
     *     4567
     *   ]
     * }
     */
    Team: {
      /** Id */
      _id?: string;
      /**
       * Name
       * @description The name of the team
       */
      name: string;
      /**
       * Pilots
       * @description The 2 pilots composing the team (by CIVLID)
       */
      pilots: number[];
      /**
       * Deleted
       * Format: date-time
       */
      deleted?: string;
    };
    /** TeamExport */
    TeamExport: {
      /** Id */
      _id: string;
      /** Name */
      name: string;
      /** Pilots */
      pilots: components["schemas"]["Pilot"][];
    };
    /**
     * Trick
     * @example {
     *   "_id": "bababababaabababababab",
     *   "name": "Misty to Helicopter",
     *   "acronym": "MH",
     *   "solo": true,
     *   "synchro": true,
     *   "directions": [
     *     "left",
     *     "right"
     *   ],
     *   "technical_coefficient": 1.75,
     *   "bonuses": [
     *     {
     *       "name": "twisted",
     *       "bonus": 3
     *     },
     *     {
     *       "name": "reverse",
     *       "bonus": 3
     *     }
     *   ],
     *   "first_maneuver": 0,
     *   "no_first_maneuver": 0,
     *   "last_maneuver": 0,
     *   "no_last_maneuver": 0,
     *   "repeatable": false,
     *   "tricks": [
     *     {
     *       "name": "left Misty to Helicopter",
     *       "acronym": "LMH",
     *       "technical_coefficient": 1.75,
     *       "bonus": 0
     *     },
     *     {
     *       "name": "right Misty to Helicopter",
     *       "acronym": "RMH",
     *       "technical_coefficient": 1.75,
     *       "bonus": 0
     *     },
     *     {
     *       "name": "twisted left Misty to Helicopter",
     *       "acronym": "/LMH",
     *       "technical_coefficient": 1.75,
     *       "bonus": 3
     *     },
     *     {
     *       "name": "twisted right Misty to Helicopter",
     *       "acronym": "/RMH",
     *       "technical_coefficient": 1.75,
     *       "bonus": 3
     *     },
     *     {
     *       "name": "left Misty to Helicopter reverse",
     *       "acronym": "LMHR",
     *       "technical_coefficient": 1.75,
     *       "bonus": 3
     *     },
     *     {
     *       "name": "right Misty to Helicopter reverse",
     *       "acronym": "RMHR",
     *       "technical_coefficient": 1.75,
     *       "bonus": 3
     *     },
     *     {
     *       "name": "twisted left Misty to Helicopter reverse",
     *       "acronym": "/LMHR",
     *       "technical_coefficient": 1.75,
     *       "bonus": 6
     *     },
     *     {
     *       "name": "twisted right Misty to Helicopter reverse",
     *       "acronym": "/RMHR",
     *       "technical_coefficient": 1.75,
     *       "bonus": 6
     *     }
     *   ]
     * }
     */
    Trick: {
      /** Id */
      _id?: string;
      /**
       * Name
       * @description The name of the trick (without bonuses)
       */
      name: string;
      /**
       * Acronym
       * @description The acronym of the trick (without bonuses)
       */
      acronym: string;
      /**
       * Solo
       * @description Is this trick valid for solo competitions
       */
      solo: boolean;
      /**
       * Synchro
       * @description Is this trick valid for synchro competitions
       */
      synchro: boolean;
      /**
       * Directions
       * @description List of allowed diredctions for the trick. Empty list implies a trick with a unique direction
       */
      directions: string[];
      /**
       * Technical Coefficient
       * @description The technical coefficient of the trick
       */
      technical_coefficient: number;
      /**
       * Bonuses
       * @description List of all bonuses that can apply to this trick
       */
      bonuses: components["schemas"]["Bonus"][];
      /**
       * Bonus Constraints
       * @description List of bonuses that are exclusive to each other
       * @default []
       */
      bonus_constraints?: string[][];
      /**
       * First Maneuver
       * @description If positive, indicates that the trick must be performed in the first N tricks of the run
       * @default 0
       */
      first_maneuver?: number;
      /**
       * No First Maneuver
       * @description If positive, indicates that the trick must not be performed in the first N tricks of the run
       * @default 0
       */
      no_first_maneuver?: number;
      /**
       * Last Maneuver
       * @description If positive, indicates that the trick must be performed in the last N tricks of the run
       * @default 0
       */
      last_maneuver?: number;
      /**
       * No Last Maneuver
       * @description If positive, indicates that the trick must not be performed in the last N tricks of the run
       * @default 0
       */
      no_last_maneuver?: number;
      /**
       * Tricks
       * @description List of all the variant of the trick (this is automatically generated)
       * @default []
       */
      tricks?: components["schemas"]["UniqueTrick"][];
      /**
       * Repeatable
       * @description Is this trick can be repeatable
       * @default false
       */
      repeatable?: boolean;
      /**
       * Deleted
       * Format: date-time
       */
      deleted?: string;
    };
    /**
     * UniqueTrick
     * @example {
     *   "name": "twisted left Misty to Helicopter reverse",
     *   "acronym": "/LMHR",
     *   "technical_coefficient": 1.75,
     *   "bonus": 6,
     *   "bonus_types": [
     *     "twist",
     *     "reverse"
     *   ],
     *   "uniqueness": [
     *     "left",
     *     "reverse"
     *   ],
     *   "base_trick": "Misty To Helicoper"
     * }
     */
    UniqueTrick: {
      /** Name */
      name: string;
      /** Acronym */
      acronym: string;
      /** Technical Coefficient */
      technical_coefficient: number;
      /** Bonus */
      bonus: number;
      /** Bonus Types */
      bonus_types: string[];
      /** Base Trick */
      base_trick: string;
      /** Uniqueness */
      uniqueness: string[];
    };
    /** ValidationError */
    ValidationError: {
      /** Location */
      loc: (string | number)[];
      /** Message */
      msg: string;
      /** Error Type */
      type: string;
    };
    /** SeasonResult */
    models__pilots_with_results__SeasonResult: {
      season: components["schemas"]["Season"];
      /** Rank */
      rank: number;
    };
    /** SeasonResult */
    models__seasons__SeasonResult: {
      pilot?: components["schemas"]["Pilot"];
      team?: components["schemas"]["TeamExport"];
      /** Score */
      score: number;
    };
  };
  responses: never;
  parameters: never;
  requestBodies: never;
  headers: never;
  pathItems: never;
}
