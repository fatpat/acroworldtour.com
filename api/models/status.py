import logging
from pydantic import BaseModel
from typing import Optional

class Status(BaseModel):
    project: str
    version: str

    class Config:
        schema_extra = {
            "example": {
                "project": "Acropyx2",
                "version":"2.0.1",
            }
        }

