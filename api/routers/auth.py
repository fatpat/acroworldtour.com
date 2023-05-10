from fastapi import APIRouter, Depends, HTTPException
from fastapi.security import OAuth2PasswordRequestForm
from typing import Any

from core.security import check_credentials, create_access_token, auth
from core.config import settings

auth_router = APIRouter()

@auth_router.post("/login")
def login(
    form_data: OAuth2PasswordRequestForm = Depends()
) -> Any:
    """
    Get the JWT for a user with data from OAuth2 request form body.
    """

    user = check_credentials(username=form_data.username, password=form_data.password)
    if not user:
        raise HTTPException(status_code=401, detail="Incorrect username or password")

    return {
        "access_token": create_access_token(sub=form_data.username),
        "token_type": "bearer",
    }

@auth_router.get(
    "/me",
    response_description="return user informations",
    dependencies=[Depends(auth)]
)
def me():
    return {
        'name': settings.ADMIN_USER,
        'nickname': settings.ADMIN_USER,
        'email': f"{settings.ADMIN_USER}@acropyx",
        'org_id': 'acropyx',
    }
