package model

type YggdrasilPublicKey struct {
	PublicKey string `json:"publicKey"`
}

type YggdrasilPublicKeys struct {
	PlayerCertificateKeys []YggdrasilPublicKey `json:"playerCertificateKeys"`
	ProfilePropertyKeys   []YggdrasilPublicKey `json:"profilePropertyKeys"`
}
