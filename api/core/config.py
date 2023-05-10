import secrets
from typing import List, Union

from pydantic import AnyHttpUrl, BaseSettings, validator


class Settings(BaseSettings):
    PROJECT_NAME: str = "acropyx2 API"
    PROJECT_DESCRIPTION: str = "Manage Paragliding Aerobatics competitions"
    VERSION: str = "2.0.1"
    JWT_SECRET: str = "TEST_SECRET_DO_NOT_USE_IN_PROD"
    JWT_ALGORITHM: str = "HS256"
    # 60 minutes * 24 hours * 8 days = 8 days
    ACCESS_TOKEN_EXPIRE_MINUTES: int = 60 * 24 * 8
    SERVER_NAME: str = PROJECT_NAME
    SERVER_HOST: AnyHttpUrl = "http://localhost:8000"

    MONGODB_URL: str = 'mongodb://127.0.0.1/'
    DATABASE: str = 'acropyx2-database-developtment'
    TESTDB: str = None
    # no auth by default
    ADMIN_USER: str = 'admin'
    ADMIN_PASS: str = None

    CACHE_EXPIRES: int = 60

    class Config:
        case_sensitive = True

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
            {"name": "to twisted sat", "post_acronym": "S", "type": "twist"},
            {"name": "flip", "post_acronym": "F", "type": "flip"},
            {"name": "double flip", "post_acronym": "FF", "type": "flip"},
            {"name": "wing touch", "post_acronym": "T", "type": "other"},
        ]

        available_directions = [
            {"name": "right", "acronym": "R" },
            {"name": "left", "acronym": "L" },
            {"name": "mirror", "acronym": "M" },
            {"name": "opposite", "acronym": "O"},
        ]

        bonus_constraints = [
#                ["twisted exit"],
                ["twisted", "devil twist"],
                ["twisted", "full twist"],
                ["twisted", "to twisted sat"],
                ["twisted", "flip"],
                ["twisted", "double flip"],

#                ["devil twist", "twisted exit"],
                ["devil twist", "full twist"],
                ["devil twist", "to twisted sat"],
                ["devil twist", "flip"],
                ["devil twist", "double flip"],

                ["full twisted", "twisted exit"],
                ["full twisted", "to twisted sat"],
                ["full twisted", "flip"],
                ["full twisted", "double flip"],

                ["twisted exit", "to twisted sat"],
                ["twisted exit", "flip"],
                ["twisted exit", "double flip"],

                ["flip", "double flip"],
        ]

    class competitions:
        warning : float = 0.5
        malus_repartition : float = 13
        warnings_to_dsq : int = 3
        judges_weight_senior : int = 100
        judges_weight_certified : int = 100
        judges_weight_trainee : int = 20
        mark_percentage_solo_technical : int = 40
        mark_percentage_solo_choreography : int = 40
        mark_percentage_solo_landing : int = 20
        mark_percentage_synchro_technical : int = 25
        mark_percentage_synchro_choreography : int = 25
        mark_percentage_synchro_landing : int = 25
        mark_percentage_synchro_synchro : int = 25
        max_bonus_twist_per_run : int = 5
        max_bonus_reverse_per_run : int = 3
        max_bonus_flip_per_run : int = 2


    class runs:
        sample: int = 0

settings = Settings()
