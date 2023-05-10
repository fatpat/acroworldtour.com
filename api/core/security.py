import logging
import secrets
from fastapi import Depends, HTTPException, status
from fastapi.security import HTTPBasic, HTTPBasicCredentials, OAuth2PasswordBearer
from jose import jwt, JWTError
from pydantic import BaseModel
from typing import Optional

from datetime import datetime, timedelta

from core.config import settings

logger = logging.getLogger(__name__)
security = HTTPBasic()
oauth2_scheme = OAuth2PasswordBearer(tokenUrl=f"/auth/login")

class TokenData(BaseModel):
    username: Optional[str] = None

def check_credentials(username: str, password: str) -> bool:
    if settings.ADMIN_USER is None or settings.ADMIN_PASS is None:
        return True
    correct_username = secrets.compare_digest(username, settings.ADMIN_USER)
    correct_password = secrets.compare_digest(password, settings.ADMIN_PASS)
    return (correct_username and correct_password)


def create_access_token(sub: str) -> str:
    return jwt_create_token(
        token_type="access_token",
        lifetime=timedelta(minutes=settings.ACCESS_TOKEN_EXPIRE_MINUTES),
        sub=sub,
    )

def jwt_create_token(
    token_type: str,
    lifetime: timedelta,
    sub: str,
) -> str:
    payload = {}
    expire = datetime.utcnow() + lifetime
    payload["type"] = token_type

    # https://datatracker.ietf.org/doc/html/rfc7519#section-4.1.3
    # The "exp" (expiration time) claim identifies the expiration time on
    # or after which the JWT MUST NOT be accepted for processing
    payload["exp"] = expire

    # The "iat" (issued at) claim identifies the time at which the
    # JWT was issued.
    payload["iat"] = datetime.utcnow()

    # The "sub" (subject) claim identifies the principal that is the
    # subject of the JWT
    payload["sub"] = str(sub)
    return jwt.encode(payload, settings.JWT_SECRET, algorithm=settings.JWT_ALGORITHM)

def no_auth():
    return True

def auth_http_basic(credentials: HTTPBasicCredentials = Depends(security)):
    if not check_credentials(username=credentials.username, password=credentials.password):
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="Incorrect email or password",
            headers={"WWW-Authenticate": "Basic"},
        )
    return True

def auth_jwt(
    token: str = Depends(oauth2_scheme)
) -> str:
    credentials_exception = HTTPException(
        status_code=status.HTTP_401_UNAUTHORIZED,
        detail="Could not validate credentials",
        headers={"WWW-Authenticate": "Bearer"},
    )
    try:
        payload = jwt.decode(
            token,
            settings.JWT_SECRET,
            algorithms=[settings.JWT_ALGORITHM],
            options={"verify_aud": False},
        )
        username: str = payload.get("sub")
        if username is None:
            raise credentials_exception
        token_data = TokenData(username=username)
    except JWTError:
        raise credentials_exception

    return settings.ADMIN_USER

if settings.ADMIN_USER is not None and settings.ADMIN_PASS is not None:
    auth = auth_jwt
    logger.info("using JWT authentication")
else:
    logger.info("disabling authentication at all")
    auth = no_auth
