from utils.public_urls import normalize_public_url, public_api_url


class DummyConfig:
    def __init__(self, value: str):
        self.value = value

    def get(self, key: str, default=None):
        return self.value if key == "server.api_url" else default


def test_normalize_public_url_keeps_absolute_path_without_trailing_slash():
    assert normalize_public_url("https://skin.example.com/skin/api/") == "https://skin.example.com/skin/api"


def test_normalize_public_url_adds_https_for_host_only_values():
    assert normalize_public_url("skin.example.com/skinapi") == "https://skin.example.com/skinapi"


def test_normalize_public_url_rejects_relative_values():
    assert normalize_public_url("/skinapi") == ""


def test_public_api_url_uses_configured_value():
    assert public_api_url(DummyConfig("https://configured.example/ygg/")) == "https://configured.example/ygg"
