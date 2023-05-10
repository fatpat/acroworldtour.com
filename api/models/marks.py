import logging
from pydantic import BaseModel, Field, validator
from bson import ObjectId
from typing import List, Optional; validator
from fastapi.encoders import jsonable_encoder
import pymongo
from enum import Enum
from datetime import datetime

from models.judges import Judge

from core.config import settings
from core.utils import float2digits, float3digits

class JudgeMarkExport(BaseModel):
    judge: Optional[Judge]
    technical: Optional[float]
    choreography: Optional[float]
    landing: Optional[float]
    synchro: Optional[float]

    _technical = validator('technical', allow_reuse=True)(float3digits)
    _choreography = validator('choreography', allow_reuse=True)(float3digits)
    _landing = validator('landing', allow_reuse=True)(float3digits)
    _synchro = validator('synchro', allow_reuse=True)(float3digits)

    class Config:
        json_encoders = {ObjectId: str}


class JudgeMark(BaseModel):
    judge: str
    technical: Optional[float] = Field(None, ge=0)
    choreography: Optional[float] = Field(None, ge=0)
    landing: Optional[float] = Field(None, ge=0)
    synchro: Optional[float] = Field(None, ge=0)

    _technical = validator('technical', allow_reuse=True)(float3digits)
    _choreography = validator('choreography', allow_reuse=True)(float3digits)
    _landing = validator('landing', allow_reuse=True)(float3digits)
    _synchro = validator('synchro', allow_reuse=True)(float3digits)

    class Config:
        schema_extra = {
            "example": {
                "judge": "Jerry The Judge",
                "technical": 2.5,
                "choreography": 7,
                "landing": 7,
                "synchro": 7
            }
        }

    async def export(self, cache:dict = {}) -> JudgeMarkExport:
        judge = None
        if self.judge != "":
            judge = await Judge.get(self.judge, cache=cache)

        return JudgeMarkExport(
            judge = judge,
            technical = self.technical,
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
    malus: float 
    notes: List[str] 

    _technicity = validator('technicity', allow_reuse=True)(float3digits)
    _bonus_percentage = validator('bonus_percentage', allow_reuse=True)(float3digits)
    _technical = validator('technical', allow_reuse=True)(float3digits)
    _choreography = validator('choreography', allow_reuse=True)(float3digits)
    _landing = validator('landing', allow_reuse=True)(float3digits)
    _synchro = validator('synchro', allow_reuse=True)(float3digits)
    _bonus = validator('bonus', allow_reuse=True)(float3digits)
    _score = validator('score', allow_reuse=True)(float3digits)

    class Config:
        json_encoders = {ObjectId: str}


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
    malus: float = Field(..., ge=0)
    notes: List[str] = []

    _technicity = validator('technicity', allow_reuse=True)(float3digits)
    _bonus_percentage = validator('bonus_percentage', allow_reuse=True)(float3digits)
    _technical = validator('technical', allow_reuse=True)(float3digits)
    _choreography = validator('choreography', allow_reuse=True)(float3digits)
    _landing = validator('landing', allow_reuse=True)(float3digits)
    _synchro = validator('synchro', allow_reuse=True)(float3digits)
    _bonus = validator('bonus', allow_reuse=True)(float3digits)
    _score = validator('score', allow_reuse=True)(float3digits)

    class Config:
        schema_extra = {
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
        }

    async def export(self, cache:dict = {}) -> FinalMarkExport:
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
            malus = self.malus,
            notes = self.notes,
        )
