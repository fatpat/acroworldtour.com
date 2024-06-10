import logging
from pydantic import ConfigDict, BaseModel
from typing import Optional

class Status(BaseModel):
    project: str
    version: str
    model_config = ConfigDict(json_schema_extra={
        "example": {
            "project": "Acropyx2",
            "version":"2.0.1",
        }
    })

