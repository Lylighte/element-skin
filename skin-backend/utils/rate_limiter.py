import time
from collections import defaultdict
from typing import Dict, Tuple
from fastapi import Request, HTTPException

class RateLimiter:
    def __init__(self, db: "Database"):
        self.db = db
        self._attempts: Dict[Tuple[str, str], list] = defaultdict(list)

    async def _get_setting(self, key: str, default: str) -> str:
        return await self.db.setting.get(key, default)

    async def is_enabled(self) -> bool:
        enabled = await self._get_setting("rate_limit_enabled", "true")
        return enabled.lower() == "true"

    def _clean_old_attempts(self, ip: str, endpoint: str, window_seconds: int):
        current_time = time.time()
        key = (ip, endpoint)
        self._attempts[key] = [
            (ts, count)
            for ts, count in self._attempts[key]
            if current_time - ts < window_seconds
        ]

    async def _check_limit(self, ip: str, endpoint: str, max_val: int, window_seconds: int) -> bool:
        if not await self.is_enabled():
            return True

        self._clean_old_attempts(ip, endpoint, window_seconds)

        key = (ip, endpoint)
        current_attempts = sum(count for _, count in self._attempts[key])

        if current_attempts >= max_val:
            return False

        self._attempts[key].append((time.time(), 1))
        return True

    async def check(self, request: Request, is_auth_endpoint: bool = False):
        if not await self.is_enabled():
            return

        ip = request.client.host
        endpoint = request.url.path

        if is_auth_endpoint:
            max_attempts = int(await self._get_setting("rate_limit_auth_attempts", "5"))
            window_minutes = int(await self._get_setting("rate_limit_auth_window", "15"))
            if not await self._check_limit(ip, endpoint, max_attempts, window_minutes * 60):
                raise HTTPException(status_code=429, detail="Too many attempts. Please try again later.")
        else:
            # General limit can be hardcoded or made configurable if needed
            if not await self._check_limit(ip, endpoint, 100, 60): # Default general limit
                 raise HTTPException(status_code=429, detail="Rate limit exceeded. Please slow down.")

    def reset(self, ip: str, endpoint: str):
        key = (ip, endpoint)
        if key in self._attempts:
            del self._attempts[key]
