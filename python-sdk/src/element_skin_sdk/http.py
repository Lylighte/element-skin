"""HTTP transport and error handling."""

from __future__ import annotations

from typing import Any, Mapping

import httpx

from .exceptions import APIError, AuthenticationError, NotFound, OAuthError, PermissionDenied


class HTTPClient:
    def __init__(
        self,
        base_url: str,
        *,
        access_token: str | None = None,
        client: httpx.Client | None = None,
        transport: httpx.BaseTransport | None = None,
        timeout: float = 10.0,
    ):
        self.base_url = base_url.rstrip("/")
        self.access_token = access_token
        self._owns_client = client is None
        self._client = client or httpx.Client(
            base_url=self.base_url,
            transport=transport,
            timeout=timeout,
        )

    def close(self) -> None:
        if self._owns_client:
            self._client.close()

    def __enter__(self) -> "HTTPClient":
        return self

    def __exit__(self, exc_type: object, exc: object, tb: object) -> None:
        self.close()

    def request(
        self,
        method: str,
        path: str,
        *,
        params: Mapping[str, Any] | None = None,
        json: object | None = None,
        data: Mapping[str, Any] | None = None,
        files: Mapping[str, Any] | None = None,
        headers: Mapping[str, str] | None = None,
        oauth_error: bool = False,
    ) -> Any:
        request_headers = dict(headers or {})
        if self.access_token:
            request_headers["Authorization"] = f"Bearer {self.access_token}"

        response = self._client.request(
            method,
            path,
            params=params,
            json=json,
            data=data,
            files=files,
            headers=request_headers,
        )
        if 200 <= response.status_code < 300:
            if response.status_code == 204 or not response.content:
                return None
            return response.json()

        raise self._error_from_response(response, oauth_error=oauth_error)

    def get(self, path: str, **kwargs: Any) -> Any:
        return self.request("GET", path, **kwargs)

    def post(self, path: str, **kwargs: Any) -> Any:
        return self.request("POST", path, **kwargs)

    def patch(self, path: str, **kwargs: Any) -> Any:
        return self.request("PATCH", path, **kwargs)

    def delete(self, path: str, **kwargs: Any) -> Any:
        return self.request("DELETE", path, **kwargs)

    @staticmethod
    def _error_from_response(response: httpx.Response, *, oauth_error: bool) -> APIError:
        body: object
        try:
            body = response.json()
        except ValueError:
            body = response.text

        error_code: str | None = None
        detail = response.reason_phrase
        if isinstance(body, dict):
            if "detail" in body:
                detail = str(body["detail"])
            if "error" in body:
                error_code = str(body["error"])
                detail = str(body.get("error_description") or body["error"])

        error_cls: type[APIError]
        if oauth_error:
            error_cls = OAuthError
        elif response.status_code == 401:
            error_cls = AuthenticationError
        elif response.status_code == 403:
            error_cls = PermissionDenied
        elif response.status_code == 404:
            error_cls = NotFound
        else:
            error_cls = APIError

        return error_cls(
            response.status_code,
            detail,
            response_body=body,
            error=error_code,
        )
