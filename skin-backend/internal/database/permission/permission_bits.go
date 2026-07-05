package permission

import core "element-skin/backend/internal/permission"

var runtimePermissionBitIndexes = permissionBitIndexMap()

func setPermissionBit(bits core.BitSet, permissionID int64) {
	if bitIndex, ok := bitIndexForPermissionID(permissionID); ok {
		bits.Set(bitIndex)
	}
}

func bitIndexForPermissionID(permissionID int64) (int, bool) {
	bitIndex, ok := runtimePermissionBitIndexes[permissionID]
	return bitIndex, ok
}

func permissionBitIndexMap() map[int64]int {
	out := make(map[int64]int, len(core.Definitions))
	for _, def := range core.Definitions {
		out[int64(def.ID)] = def.BitIndex
	}
	return out
}
