import base64

from cryptography.hazmat.backends import default_backend
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import padding
from cryptography.hazmat.primitives.serialization import load_pem_private_key

class CryptoUtils:
    def __init__(self, private_key_path: str):
        with open(private_key_path, "rb") as f:
            self.private_key = load_pem_private_key(f.read(), password=None)

    def reload_from_pem(self, pem_str: str):
        if "BEGIN PRIVATE KEY" not in pem_str and "BEGIN RSA PRIVATE KEY" not in pem_str:
            raise ValueError("Invalid PEM: must contain a private key")
        self.private_key = load_pem_private_key(pem_str.encode("utf-8"), password=None)

    def sign_data(self, data: str) -> str:
        signature = self.private_key.sign(
            data.encode("utf-8"), padding.PKCS1v15(), hashes.SHA1()
        )
        return base64.b64encode(signature).decode("utf-8")

    def get_public_key_pem(self) -> str:
        public_key = self.private_key.public_key()
        pem = public_key.public_bytes(
            encoding=serialization.Encoding.PEM,
            format=serialization.PublicFormat.SubjectPublicKeyInfo,
        )
        return pem.decode("utf-8")


def compute_key_fingerprint(pem_str: str) -> str:
    """Compute the SHA-256 fingerprint of a private key's public key.

    Args:
        pem_str: PEM-encoded private key string.

    Returns:
        Fingerprint in the format ``sha256:<64-lowercase-hex-chars>``.

    Raises:
        ValueError: If the PEM string is invalid or cannot be parsed.
    """
    try:
        private_key = load_pem_private_key(pem_str.encode("utf-8"), password=None)
    except Exception as exc:
        raise ValueError(f"Invalid PEM: {exc}") from exc

    public_key = private_key.public_key()
    der_bytes = public_key.public_bytes(
        encoding=serialization.Encoding.DER,
        format=serialization.PublicFormat.SubjectPublicKeyInfo,
    )

    digest = hashes.Hash(hashes.SHA256(), backend=default_backend())
    digest.update(der_bytes)
    return f"sha256:{digest.finalize().hex()}"
