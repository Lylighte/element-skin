from __future__ import annotations

import pytest

from element_skin_sdk.oauth.pkce import generate_code_verifier


def test_generate_code_verifier_respects_requested_length() -> None:
    verifier = generate_code_verifier(43)

    assert len(verifier) == 43


@pytest.mark.parametrize("length", [42, 129])
def test_generate_code_verifier_rejects_invalid_lengths(length: int) -> None:
    with pytest.raises(ValueError) as exc:
        generate_code_verifier(length)

    assert str(exc.value) == "code verifier length must be between 43 and 128"
