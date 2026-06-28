package permission

import "errors"

var ErrForbidden = errors.New("permission denied")

type Actor struct {
	SubjectID         string
	UserID            string
	SessionKind       string
	Entrypoint        string
	SessionID         string
	BoundProfileID    string
	DelegationID      string
	DelegatedClientID string
	Permissions       BitSet
}

func (a Actor) Has(def Definition) bool {
	return a.Permissions.Has(def.BitIndex)
}

func (a Actor) Require(def Definition) error {
	if !a.Has(def) {
		return ErrForbidden
	}
	return nil
}
