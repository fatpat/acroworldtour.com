import logging
from pydantic import BaseModel, Field
from typing import Optional

from core.config import settings

class JudgeWeights(BaseModel):
    senior: int = Field(settings.competitions.judges_weight_senior, description="weight of a senior judge's mark")
    certified: int = Field(settings.competitions.judges_weight_certified, description="weight of a certified judge's mark")
    trainee: int = Field(settings.competitions.judges_weight_trainee, description="weight of a trainee judge's mark")

    class Config:
        schema_extra = {
            "example": {
                "senior": 100,
                "certified": 100,
                "trainee": 20,
            }
        }

class MarkPercentageSolo(BaseModel):
    technical: int = Field(settings.competitions.mark_percentage_solo_technical, description="% of the technical part in the final score for solo runs")
    choreography: int = Field(settings.competitions.mark_percentage_solo_choreography, description="% of the choreography part in the final score for solo runs")
    landing: int = Field(settings.competitions.mark_percentage_solo_landing, description="% of the landing part in the final score for solo runs")

    class Config:
        schema_extra = {
            "example": {
                "technical": 40,
                "choreography": 40,
                "landing": 20,
            }
        }

class MarkPercentageSynchro(BaseModel):
    technical: int = Field(settings.competitions.mark_percentage_synchro_technical, description="% of the technical part in the final score for synchro runs")
    choreography: int = Field(settings.competitions.mark_percentage_synchro_choreography, description="% of the choreography part in the final score for synchro runs")
    landing: int = Field(settings.competitions.mark_percentage_synchro_landing, description="% of the landing part in the final score for synchro runs")
    synchro: int = Field(settings.competitions.mark_percentage_synchro_synchro, description="% of the synchro part in the final score for synchro runs")

    class Config:
        schema_extra = {
            "example": {
                "technical": 25,
                "choreography": 25,
                "landing": 25,
                "synchro": 25
            }
        }

class MarkPercentages(BaseModel):
    solo: MarkPercentageSolo = MarkPercentageSolo()
    synchro: MarkPercentageSynchro = MarkPercentageSynchro()

    class Config:
        schema_extra = {
            "example": {
                "solo": {
                    "technical": 40,
                    "choreography": 40,
                    "landing": 20
                },
                "synchro": {
                    "technical": 25,
                    "choreography": 25,
                    "landing": 25,
                    "synchro": 25
                }
            }
        }

class MaxBonusPerRun(BaseModel):
    twist: int = Field(settings.competitions.max_bonus_twist_per_run, description="maximum number of twisted tricks per run")
    reverse: int = Field(settings.competitions.max_bonus_reverse_per_run, description="maximum number of reverse tricks per run")
    flip: int = Field(settings.competitions.max_bonus_flip_per_run, description="maximum number of flip tricks per run")

    class Config:
        schema_extra = {
            "example": {
                "twist": 5,
                "reverse": 3,
                "flip": 1
            }
        }

class CompetitionConfig(BaseModel):
    warning: float = Field(settings.competitions.warning, description="The point deduction for a warning")
    malus_repetition: float = Field(settings.competitions.malus_repartition, description="% reduction malus of choreography for repetition")
    warnings_to_dsq: int = Field(settings.competitions.warnings_to_dsq, description="number of warnings in a comp that lead to DSQ")
    judge_weights: JudgeWeights = JudgeWeights()
    mark_percentages: MarkPercentages = MarkPercentages()
    max_bonus_per_run: MaxBonusPerRun = MaxBonusPerRun()

    class Config:
        schema_extra = {
            "example": {
                "warning": 0.5,
                "malus_repetition": 13,
                "warnings_to_dsq": 3,
                "judges_weight": {
                    "senior": 100,
                    "certified": 100,
                    "trainee": 20,
                },
                "mark_percentages": {
                    "solo": {
                        "technical": 40,
                        "choreography": 40,
                        "landing": 20,
                    },
                    "synchro": {
                        "technical": 25,
                        "choreography": 25,
                        "landing": 25,
                        "synchro": 25
                    }
                },
                "max_bonus_per_run": {
                    "twist": 5,
                    "reverse": 3,
                    "flip": 1
                }
            }
        }

