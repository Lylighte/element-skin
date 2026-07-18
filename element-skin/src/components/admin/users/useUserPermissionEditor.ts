import { computed, ref, watch } from 'vue'
import type {
  PermissionDefinition,
  PermissionOverrideEffect,
  PermissionRole,
  User,
  UserPermissionsResponse,
} from '@/api/types'
import {
  createPermissionDisplayItem,
  groupPermissionItems,
  normalizeSelectedResource,
  selectedPermissionGroup,
  type PermissionDisplayGroup,
  type PermissionDisplayItem,
} from '@/components/permissions/permissionDisplay'

type PermissionGroupKind = 'inherited' | 'override'

interface UserPermissionEditorInput {
  visible: () => boolean
  user: () => User
  permissionState: () => UserPermissionsResponse | null
  currentPermissions: () => string[]
  currentUserProtected: () => boolean
  isSelf: () => boolean
}

export function useUserPermissionEditor(input: UserPermissionEditorInput) {
  const selectedRoleId = ref('')
  const selectedPermissionCode = ref('')
  const selectedPermissionEffect = ref<PermissionOverrideEffect>('allow')
  const selectedInheritedResource = ref('')
  const selectedOverrideResource = ref('')

  const roleIds = computed(
    () => new Set(input.permissionState()?.roles || input.user().roles || []),
  )
  const effectivePermissions = computed(
    () => new Set(input.permissionState()?.effective_permissions || input.user().permissions || []),
  )
  const overrideMap = computed(() => {
    const out = new Map<string, PermissionOverrideEffect>()
    for (const item of input.permissionState()?.overrides || [])
      out.set(item.permission_code, item.effect)
    return out
  })
  const currentPermissionSet = computed(() => new Set(input.currentPermissions()))
  const canManageProtected = computed(() =>
    currentPermissionSet.value.has('permission_protected.manage.any'),
  )
  const canGrantPermission = computed(() => currentPermissionSet.value.has('permission.grant.any'))
  const canRevokePermission = computed(() =>
    currentPermissionSet.value.has('permission.revoke.any'),
  )
  const targetProtected = computed(
    () => input.permissionState()?.protected || input.user().protected || false,
  )
  const canTransferProtectedSubject = computed(
    () =>
      input.currentUserProtected() &&
      canManageProtected.value &&
      !input.isSelf() &&
      !targetProtected.value,
  )
  const assignedRoleLabels = computed(() => {
    const roles = input.permissionState()?.catalog.roles || []
    const selected = roles.filter((role) => roleIds.value.has(role.id))
    if (selected.length) return selected
    return (input.user().roles || []).map((role) => ({
      id: role,
      name: role,
      description: '',
      system_role: true,
      protected: false,
      permissions: [],
    }))
  })
  const grantableRoles = computed(() =>
    (input.permissionState()?.catalog.roles || []).filter((role) => !roleIds.value.has(role.id)),
  )
  const permissionByCode = computed(() => {
    const out = new Map<string, PermissionDefinition>()
    for (const item of input.permissionState()?.catalog.permissions || []) out.set(item.code, item)
    return out
  })
  const inheritedPermissionGroups = computed(() => {
    const inherited = new Map<string, PermissionDisplayItem>()
    for (const role of input.permissionState()?.catalog.roles || []) {
      if (!roleIds.value.has(role.id)) continue
      for (const code of role.permissions) {
        if (overrideMap.value.has(code)) continue
        if (!effectivePermissions.value.has(code)) continue
        const definition = permissionByCode.value.get(code)
        inherited.set(code, createPermissionDisplayItem(code, definition))
      }
    }
    return groupPermissionItems([...inherited.values()])
  })
  const overridePermissionGroups = computed(() => {
    const items = (input.permissionState()?.overrides || []).map((item) => ({
      ...createPermissionDisplayItem(
        item.permission_code,
        permissionByCode.value.get(item.permission_code),
      ),
      effect: item.effect,
    }))
    return groupPermissionItems(items)
  })
  const selectedInheritedPermissionGroup = computed(() =>
    selectedPermissionGroup(inheritedPermissionGroups.value, selectedInheritedResource.value),
  )
  const selectedOverridePermissionGroup = computed(() =>
    selectedPermissionGroup(overridePermissionGroups.value, selectedOverrideResource.value),
  )
  const grantablePermissionOptions = computed(() =>
    (input.permissionState()?.catalog.permissions || []).filter(
      (item) => !overrideMap.value.has(item.code),
    ),
  )
  const selectedPermission = computed(() =>
    selectedPermissionCode.value ? permissionByCode.value.get(selectedPermissionCode.value) : null,
  )
  const canAddSelectedPermission = computed(() => {
    if (!selectedPermission.value) return false
    if (selectedPermissionEffect.value === 'allow' && !canGrantPermission.value) return false
    if (selectedPermissionEffect.value === 'deny' && !canRevokePermission.value) return false
    return !permissionControlDisabled(selectedPermission.value)
  })

  watch(
    [() => input.visible(), inheritedPermissionGroups, overridePermissionGroups],
    ([open, inheritedGroups, overrideGroups]) => {
      if (!open) {
        selectedRoleId.value = ''
        selectedPermissionCode.value = ''
        selectedPermissionEffect.value = 'allow'
        selectedInheritedResource.value = ''
        selectedOverrideResource.value = ''
        return
      }

      selectedInheritedResource.value = normalizeSelectedResource(
        selectedInheritedResource.value,
        inheritedGroups,
      )
      selectedOverrideResource.value = normalizeSelectedResource(
        selectedOverrideResource.value,
        overrideGroups,
      )
    },
  )

  function selectPermissionGroup(resource: string, kind: PermissionGroupKind) {
    if (kind === 'inherited') selectedInheritedResource.value = resource
    else selectedOverrideResource.value = resource
  }

  function isSelectedPermissionGroup(group: PermissionDisplayGroup, kind: PermissionGroupKind) {
    return kind === 'inherited'
      ? selectedInheritedResource.value === group.resource
      : selectedOverrideResource.value === group.resource
  }

  function roleTagClosable(role: PermissionRole) {
    if (role.id === 'user') return false
    if (input.isSelf() && role.protected) return false
    if (role.protected && !canManageProtected.value) return false
    return canRevokePermission.value
  }

  function permissionControlDisabled(row: PermissionDefinition) {
    if (row.code === 'permission_protected.manage.any') return true
    if (input.isSelf() && isProtectedPermission(row)) return true
    if (isProtectedPermission(row) && !canManageProtected.value) return true
    const current = overrideMap.value.get(row.code) || 'inherit'
    if (current === 'allow') return !canRevokePermission.value
    if (current === 'deny') return !canGrantPermission.value
    return !canGrantPermission.value && !canRevokePermission.value
  }

  function consumeSelectedRole() {
    if (!selectedRoleId.value) return ''
    const roleId = selectedRoleId.value
    selectedRoleId.value = ''
    return roleId
  }

  function consumeSelectedPermission() {
    if (!selectedPermissionCode.value || !canAddSelectedPermission.value) return null
    const payload = {
      code: selectedPermissionCode.value,
      effect: selectedPermissionEffect.value,
    }
    selectedPermissionCode.value = ''
    return payload
  }

  return {
    selectedRoleId,
    selectedPermissionCode,
    selectedPermissionEffect,
    canManageProtected,
    canGrantPermission,
    canRevokePermission,
    assignedRoleLabels,
    grantableRoles,
    inheritedPermissionGroups,
    overridePermissionGroups,
    selectedInheritedPermissionGroup,
    selectedOverridePermissionGroup,
    grantablePermissionOptions,
    canAddSelectedPermission,
    canTransferProtectedSubject,
    selectPermissionGroup,
    isSelectedPermissionGroup,
    roleTagClosable,
    permissionControlDisabled,
    consumeSelectedRole,
    consumeSelectedPermission,
  }
}

function isProtectedPermission(row: PermissionDefinition) {
  return row.scope === 'system' || row.resource === 'permission_protected'
}
