import logging
from pydantic import ConfigDict, BaseModel, Field, validator
from bson import ObjectId
from typing import List, Optional; validator
from fastapi.encoders import jsonable_encoder
import pymongo
from enum import Enum
from datetime import datetime

from models.judges import Judge
from models.cache import Cache

from core.config import settings
from core.utils import float2digits, float3digits

class JudgeMarkExport(BaseModel):
    judge: Optional[Judge] = None
    technical: Optional[float] = None
    technical_per_trick: Optional[List[Optional[float]]] = Field(None)
    choreography: Optional[float] = None
    landing: Optional[float] = None
    synchro: Optional[float] = None

    _technical = validator('technical', allow_reuse=True)(float3digits)
    _choreography = validator('choreography', allow_reuse=True)(float3digits)
    _landing = validator('landing', allow_reuse=True)(float3digits)
    _synchro = validator('synchro', allow_reuse=True)(float3digits)
    # TODO[pydantic]: The following keys were removed: `json_encoders`.
    # Check https://docs.pydantic.dev/dev-v2/migration/#changes-to-config for more information.
    model_config = ConfigDict(json_encoders={ObjectId: str})


class JudgeMark(BaseModel):
    judge: str
    technical: Optional[float] = Field(None, ge=0)
    technical_per_trick: Optional[List[Optional[float]]] = Field(None)
    choreography: Optional[float] = Field(None, ge=0)
    landing: Optional[float] = Field(None, ge=0)
    synchro: Optional[float] = Field(None, ge=0)

    _technical = validator('technical', allow_reuse=True)(float3digits)
    _choreography = validator('choreography', allow_reuse=True)(float3digits)
    _landing = validator('landing', allow_reuse=True)(float3digits)
    _synchro = validator('synchro', allow_reuse=True)(float3digits)
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "judge": "Jerry The Judge",
            "technical": 2.5,
            "choreography": 7,
            "landing": 7,
            "synchro": 7
        }
    })

    async def export(self, cache:Cache = None) -> JudgeMarkExport:
        judge = None
        if self.judge != "":
            judge = await Judge.get(self.judge, cache=cache)

        return JudgeMarkExport(
            judge = judge,
            technical = self.technical,
            technical_per_trick = self.technical_per_trick,
            choreography = self.choreography,
            landing = self.landing,
            synchro = self.synchro,
        )

class FinalMarkExport(BaseModel):
    judges_mark: JudgeMarkExport
    technicity: float 
    bonus_percentage: float 
    technical: float 
    choreography: float 
    landing: float 
    synchro: float 
    bonus: float 
    score: float 
    warnings: list[str]
    warnings2: list[str] = []
    malus: float 
    notes: List[str] 
    mark_type: Optional[str] = None

    _technicity = validator('technicity', allow_reuse=True)(float3digits)
    _bonus_percentage = validator('bonus_percentage', allow_reuse=True)(float3digits)
    _technical = validator('technical', allow_reuse=True)(float3digits)
    _choreography = validator('choreography', allow_reuse=True)(float3digits)
    _landing = validator('landing', allow_reuse=True)(float3digits)
    _synchro = validator('synchro', allow_reuse=True)(float3digits)
    _bonus = validator('bonus', allow_reuse=True)(float3digits)
    _score = validator('score', allow_reuse=True)(float3digits)
    # TODO[pydantic]: The following keys were removed: `json_encoders`.
    # Check https://docs.pydantic.dev/dev-v2/migration/#changes-to-config for more information.
    model_config = ConfigDict(json_encoders={ObjectId: str})


class FinalMark(BaseModel):
    judges_mark: JudgeMark
    technicity: float = Field(..., ge=0)
    bonus_percentage: float
    technical: float = Field(..., ge=0)
    choreography: float = Field(..., ge=0)
    landing: float = Field(..., ge=0)
    synchro: float = Field(..., ge=0)
    bonus: float
    score: float = Field(..., ge=0)
    warnings: list[str]
    warnings2: list[str] = []
    malus: float = Field(..., ge=0)
    notes: List[str] = []
    mark_type: Optional[str] = None

    _technicity = validator('technicity', allow_reuse=True)(float3digits)
    _bonus_percentage = validator('bonus_percentage', allow_reuse=True)(float3digits)
    _technical = validator('technical', allow_reuse=True)(float3digits)
    _choreography = validator('choreography', allow_reuse=True)(float3digits)
    _landing = validator('landing', allow_reuse=True)(float3digits)
    _synchro = validator('synchro', allow_reuse=True)(float3digits)
    _bonus = validator('bonus', allow_reuse=True)(float3digits)
    _score = validator('score', allow_reuse=True)(float3digits)
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "judges_mark": {
                "judge": "Average of the judges marks",
                "technical": 2.5,
                "choreography": 7,
                "landing": 7,
                "synchro": 7
            },
            "technicity": 1.87,
            "bonus_percentage": 23,
            "technical": 7,
            "choreography": 6,
            "landing": 7,
            "synchro": 7,
            "bonus": 1.23,
            "score": 9.244,
            "warnings": ["box","late at briefing"],
            "malus": 13,
            "notes": ["Yellow card: big ear to start the run"]
        }
    })

    async def export(self, cache:Cache = None) -> FinalMarkExport:
        return FinalMarkExport(
            judges_mark = await self.judges_mark.export(cache=cache),
            technicity = self.technicity,
            bonus_percentage = self.bonus_percentage,
            technical = self.technical,
            choreography = self.choreography,
            landing = self.landing,
            synchro = self.synchro,
            bonus = self.bonus,
            score = self.score,
            warnings = self.warnings,
            warnings2 = self.warnings2,
            malus = self.malus,
            notes = self.notes,
            mark_type = self.mark_type,
        )
