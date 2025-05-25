import secrets
from typing import List, Union, Dict, Optional

from pydantic import AnyHttpUrl, validator
from pydantic_settings import SettingsConfigDict, BaseSettings


class Settings(BaseSettings):
    PROJECT_NAME: str = "acroworldtour.com API"
    PROJECT_DESCRIPTION: str = "API to manage and access Paragliding Aerobatics competitions results"
    VERSION: str = "2025.4"
    JWT_SECRET: str = "TEST_SECRET_DO_NOT_USE_IN_PROD"
    JWT_ALGORITHM: str = "HS256"
    # 60 minutes * 24 hours * 8 days = 8 days
    ACCESS_TOKEN_EXPIRE_MINUTES: int = 60 * 24 * 8
    SERVER_NAME: str = PROJECT_NAME

    MONGODB_URL: str = 'mongodb://127.0.0.1/'
    DATABASE: str = 'acropyx2-database-developtment'
    TESTDB: Optional[str] = None
    # no auth by default
    ADMIN_USER: str = 'admin'
    ADMIN_PASS: Optional[str] = None

    model_config = SettingsConfigDict(case_sensitive=True)

    LOG_LEVEL: str = "INFO"

    class pilots:
        civl_link_all_pilots = 'https://civlcomps.org/ranking/paragliding-aerobatics/pilots'
        civl_link_one_pilot = 'https://civlcomps.org/pilot/'

    class tricks:
        available_bonuses = [
            {"name": "reverse", "post_acronym": "R", "type": "reverse"},
            {"name": "twisted", "pre_acronym": "/", "type": "twist"},
            {"name": "twisted exit", "post_acronym": "/", "type": "twist"},
            {"name": "full twisted", "post_acronym": "\\", "type": "twist"},
            {"name": "devil twist", "post_acronym": "X", "type": "twist"},
            {"name": "devil twist stall", "post_acronym": "XX", "type": "twist"},
            {"name": "to twisted sat", "post_acronym": "S", "type": "twist"},
            {"name": "cab slide", "post_acronym": "C", "type": "twist"},
            {"name": "hardcore", "post_acronym": "HC", "type": "twist"}, #TODO(jlo): remove once migrate to hardcore entry
            {"name": "hardcore entry", "post_acronym": "HC", "type": "twist"},
            {"name": "flip", "post_acronym": "F", "type": "flip"},
            {"name": "double flip", "post_acronym": "FF", "type": "flip"},
            {"name": "wing touch", "post_acronym": "T", "type": "other"},
            {"name": "wing touch", "post_acronym": "WT", "type": "other"},
        ]

        available_directions = [
            {"name": "right", "acronym": "R" },
            {"name": "left", "acronym": "L" },
            {"name": "opposite", "acronym": "O" },
        ]

    class competitions:
        warning : float = 0.5
        warning2 : float = 1
        malus_repartition : float = 13
        warnings_to_dsq : int = 3
        judges_weight_senior : int = 100
        judges_weight_certified : int = 100
        judges_weight_trainee : int = 20
        mark_percentage_solo_technical : int = 40
        mark_percentage_solo_choreography : int = 40
        mark_percentage_solo_landing : int = 20
        mark_percentage_synchro_technical : int = 20
        mark_percentage_synchro_choreography : int = 20
        mark_percentage_synchro_landing : int = 20
        mark_percentage_synchro_synchro : int = 40
        max_bonus_twist_per_run : int = 5
        max_bonus_reverse_per_run : int = 3
        max_bonus_flip_per_run : int = 2
        max_tricks_per_type : Dict[str, int] = {
            "tumbling" : 2,
            "to infinite" : 1,
        }


    class runs:
        sample: int = 0

settings = Settings()
