import asyncio
import pytest
from unittest.mock import AsyncMock, patch
from backends.site_backend import SiteBackend
from backends.admin_backend import AdminBackend
from routes_reference import texture_storage
from utils.typing import PlayerProfile
from utils.uuid_utils import generate_random_uuid


@pytest.mark.asyncio
async def test_site_delete_user_syncs_all_profiles(db_session, test_config, user_factory):
    """User with 3 profiles → delete_user fires 3 sync_profile_delete calls."""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()

    pids = [generate_random_uuid() for _ in range(3)]
    for i, pid in enumerate(pids):
        await db_session.user.create_profile(
            PlayerProfile(pid, user.id, f"SyncUser{i}", "default", None, None)
        )

    mock_sync = AsyncMock()
    backend.set_union_backend(mock_sync)

    await backend.delete_user(user.id)

    assert mock_sync.sync_profile_delete.call_count == 3
    called_pids = {call.args[0] for call in mock_sync.sync_profile_delete.call_args_list}
    assert called_pids == set(pids)
    assert await db_session.user.get_by_id(user.id) is None


@pytest.mark.asyncio
async def test_site_delete_user_sync_failure_does_not_block(db_session, test_config, user_factory):
    """sync_profile_delete raises → user still deleted (fire-and-forget)."""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()

    pid = generate_random_uuid()
    await db_session.user.create_profile(
        PlayerProfile(pid, user.id, "FailUser", "default", None, None)
    )

    mock_sync = AsyncMock()
    mock_sync.sync_profile_delete.side_effect = Exception("Union unreachable")
    backend.set_union_backend(mock_sync)

    await backend.delete_user(user.id)

    assert mock_sync.sync_profile_delete.call_count == 1
    assert await db_session.user.get_by_id(user.id) is None


@pytest.mark.asyncio
async def test_site_delete_user_no_profiles_no_sync(db_session, test_config, user_factory):
    """User with 0 profiles → no sync calls, clean deletion."""
    backend = SiteBackend(db_session, test_config, texture_storage)
    user = await user_factory()

    mock_sync = AsyncMock()
    backend.set_union_backend(mock_sync)

    await backend.delete_user(user.id)

    mock_sync.sync_profile_delete.assert_not_called()
    assert await db_session.user.get_by_id(user.id) is None


@pytest.mark.asyncio
async def test_admin_delete_user_syncs_all_profiles(db_session, test_config, user_factory):
    """Admin backend: user with 3 profiles → 3 sync calls before cascade delete."""
    backend = AdminBackend(db_session, test_config)
    user = await user_factory()

    pids = [generate_random_uuid() for _ in range(3)]
    for i, pid in enumerate(pids):
        await db_session.user.create_profile(
            PlayerProfile(pid, user.id, f"AdminSync{i}", "default", None, None)
        )

    mock_sync = AsyncMock()
    backend.set_union_backend(mock_sync)

    await backend.delete_user(user.id, is_admin_action=True)

    assert mock_sync.sync_profile_delete.call_count == 3
    called_pids = {call.args[0] for call in mock_sync.sync_profile_delete.call_args_list}
    assert called_pids == set(pids)
    assert await db_session.user.get_by_id(user.id) is None


@pytest.mark.asyncio
async def test_admin_delete_user_sync_failure_does_not_block(db_session, test_config, user_factory):
    """Admin backend: sync failure → deletion still succeeds."""
    backend = AdminBackend(db_session, test_config)
    user = await user_factory()

    pid = generate_random_uuid()
    await db_session.user.create_profile(
        PlayerProfile(pid, user.id, "AdminFail", "default", None, None)
    )

    mock_sync = AsyncMock()
    mock_sync.sync_profile_delete.side_effect = Exception("Union unreachable")
    backend.set_union_backend(mock_sync)

    await backend.delete_user(user.id, is_admin_action=True)

    assert mock_sync.sync_profile_delete.call_count == 1
    assert await db_session.user.get_by_id(user.id) is None


@pytest.mark.asyncio
async def test_admin_delete_user_no_profiles_no_sync(db_session, test_config, user_factory):
    """Admin backend: user with 0 profiles → no sync calls."""
    backend = AdminBackend(db_session, test_config)
    user = await user_factory()

    mock_sync = AsyncMock()
    backend.set_union_backend(mock_sync)

    await backend.delete_user(user.id, is_admin_action=True)

    mock_sync.sync_profile_delete.assert_not_called()
    assert await db_session.user.get_by_id(user.id) is None
