import logging
from pydantic import BaseModel


log = logging.getLogger(__name__)

class SvgData(BaseModel):
    rank: int
    country: str | None = None
    name: str
    score: float
