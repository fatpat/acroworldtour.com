import logging
from core.config import settings
from bson import ObjectId
import motor.motor_asyncio

log = logging.getLogger(__name__)
client = motor.motor_asyncio.AsyncIOMotorClient(settings.MONGODB_URL)
db = client[settings.DATABASE]

class PyObjectId(ObjectId):
    @classmethod
    def __get_validators__(cls):
        yield cls.validate

    @classmethod
    def validate(cls, v):
        if not ObjectId.is_valid(v):
            raise ValueError("Invalid objectid")
        return ObjectId(v)

    @classmethod
    def __modify_schema__(cls, field_schema):
        field_schema.update(type="string")

async def clean_database():
    for collection in await db.list_collection_names():
        log.info(f"Dropping {collection}")
        await db.drop_collection(collection)
