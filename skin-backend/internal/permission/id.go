package permission

import "fmt"

const (
	maxPartID = uint64(0xffff)
)

type ResourceID uint16
type ActionID uint16
type ScopeID uint16
type ID uint64

func ComposeID(resource ResourceID, action ActionID, scope ScopeID) (ID, error) {
	if resource == 0 {
		return 0, fmt.Errorf("resource id is required")
	}
	if action == 0 {
		return 0, fmt.Errorf("action id is required")
	}
	if scope == 0 {
		return 0, fmt.Errorf("scope id is required")
	}
	return ID(uint64(resource)<<32 | uint64(action)<<16 | uint64(scope)), nil
}

func MustComposeID(resource ResourceID, action ActionID, scope ScopeID) ID {
	id, err := ComposeID(resource, action, scope)
	if err != nil {
		panic(err)
	}
	return id
}

func (id ID) ResourceID() ResourceID {
	return ResourceID((uint64(id) >> 32) & maxPartID)
}

func (id ID) ActionID() ActionID {
	return ActionID((uint64(id) >> 16) & maxPartID)
}

func (id ID) ScopeID() ScopeID {
	return ScopeID(uint64(id) & maxPartID)
}

func (id ID) Valid() bool {
	return id.ResourceID() != 0 && id.ActionID() != 0 && id.ScopeID() != 0 && uint64(id)>>48 == 0
}
