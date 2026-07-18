from __future__ import annotations

import json
from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any
from urllib.parse import parse_qs

import httpx
import pytest


@dataclass
class CapturedRequest:
    method: str
    path: str
    query: dict[str, list[str]]
    headers: dict[str, str]
    content: bytes
    json_body: object | None
    form: dict[str, list[str]]


@dataclass
class RequestRecorder:
    handler: Callable[[httpx.Request], httpx.Response]
    requests: list[CapturedRequest] = field(default_factory=list)

    def transport(self) -> httpx.MockTransport:
        return httpx.MockTransport(self._handle)

    def _handle(self, request: httpx.Request) -> httpx.Response:
        content = request.read()
        json_body: object | None = None
        form: dict[str, list[str]] = {}
        content_type = request.headers.get("content-type", "")

        if content and content_type.startswith("application/json"):
            json_body = json.loads(content.decode("utf-8"))
        if content and content_type.startswith("application/x-www-form-urlencoded"):
            form = parse_qs(content.decode("utf-8"), keep_blank_values=True)

        self.requests.append(
            CapturedRequest(
                method=request.method,
                path=request.url.path,
                query=parse_qs(request.url.query.decode("utf-8"), keep_blank_values=True),
                headers=dict(request.headers),
                content=content,
                json_body=json_body,
                form=form,
            )
        )
        return self.handler(request)


@pytest.fixture
def response_json() -> Callable[[dict[str, Any], int], httpx.Response]:
    def make(payload: dict[str, Any], status_code: int = 200) -> httpx.Response:
        return httpx.Response(status_code, json=payload)

    return make
