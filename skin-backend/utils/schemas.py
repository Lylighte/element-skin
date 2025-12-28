from typing import Optional, List, Dict, Any
from pydantic import BaseModel

class Agent(BaseModel):
    name: str = "Minecraft"
    version: int = 1

class AuthRequest(BaseModel):
    username: str
    password: str
    clientToken: Optional[str] = None
    requestUser: bool = False
    agent: Optional[Agent] = None

class RefreshRequest(BaseModel):
    accessToken: str
    clientToken: Optional[str] = None
    requestUser: bool = False
    selectedProfile: Optional[Dict] = None

class ValidationRequest(BaseModel):
    accessToken: str
    clientToken: Optional[str] = None

class JoinRequest(BaseModel):
    accessToken: str
    selectedProfile: str
    serverId: str
