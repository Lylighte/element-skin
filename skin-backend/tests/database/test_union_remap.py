import pytest
import time
from utils.typing import PlayerProfile, Token
from utils.uuid_utils import generate_random_uuid


@pytest.mark.asyncio
async def test_remap_basic_cascade(db_session, user_factory):
    """Single remap updates profiles.id and cascades to tokens.profile_id."""
    user = await user_factory()
    old_uuid = generate_random_uuid()
    new_uuid = generate_random_uuid()

    profile = PlayerProfile(old_uuid, user.id, "RemapPlayer")
    await db_session.user.create_profile(profile)

    token = Token(
        access_token=generate_random_uuid(),
        client_token="test-client",
        user_id=user.id,
        profile_id=old_uuid,
        created_at=int(time.time() * 1000),
    )
    await db_session.user.add_token(token)

    await db_session.union.remap_uuids({old_uuid: new_uuid})

    # Profile id updated
    p = await db_session.user.get_profile_by_id(new_uuid)
    assert p is not None
    assert p.name == "RemapPlayer"

    # Old uuid gone
    assert await db_session.user.get_profile_by_id(old_uuid) is None

    # Token profile_id cascaded
    t = await db_session.user.get_token(token.access_token)
    assert t is not None
    assert t.profile_id == new_uuid


@pytest.mark.asyncio
async def test_remap_overlapping_chain(db_session, user_factory):
    """Overlapping A→B→C remap: profile A ends up as C, profile B ends up as C.

    Uses two profiles and a NEW final uuid, processed in dependency order.
    """
    user = await user_factory()
    uuid_a = generate_random_uuid()
    uuid_b = generate_random_uuid()
    uuid_c = generate_random_uuid()

    profile_a = PlayerProfile(uuid_a, user.id, "Alpha")
    profile_b = PlayerProfile(uuid_b, user.id, "Bravo")
    await db_session.user.create_profile(profile_a)
    await db_session.user.create_profile(profile_b)

    token_a = Token(
        access_token=generate_random_uuid(),
        client_token="ch-ct",
        user_id=user.id,
        profile_id=uuid_a,
        created_at=int(time.time() * 1000),
    )
    token_b = Token(
        access_token=generate_random_uuid(),
        client_token="ch-ct",
        user_id=user.id,
        profile_id=uuid_b,
        created_at=int(time.time() * 1000),
    )
    await db_session.user.add_token(token_a)
    await db_session.user.add_token(token_b)

    await db_session.union.remap_uuids({uuid_a: uuid_b, uuid_b: uuid_c})

    # uuid_a no longer exists
    assert await db_session.user.get_profile_by_id(uuid_a) is None
    # uuid_b: the profile that was A now has id B (it was remapped A→B)
    p_b = await db_session.user.get_profile_by_id(uuid_b)
    assert p_b is not None
    assert p_b.name == "Alpha"
    # uuid_c: the profile that was B now has id C
    p_c = await db_session.user.get_profile_by_id(uuid_c)
    assert p_c is not None
    assert p_c.name == "Bravo"

    # Token cascades
    t_a = await db_session.user.get_token(token_a.access_token)
    assert t_a.profile_id == uuid_b  # A's token → B
    t_b = await db_session.user.get_token(token_b.access_token)
    assert t_b.profile_id == uuid_c  # B's token → C


@pytest.mark.asyncio
async def test_remap_collision_rejected(db_session, user_factory):
    """Remapping to an existing UUID that is not being remapped raises ValueError."""
    user = await user_factory()
    existing_uuid = generate_random_uuid()
    other_uuid = generate_random_uuid()

    profile1 = PlayerProfile(existing_uuid, user.id, "Existing")
    profile2 = PlayerProfile(other_uuid, user.id, "Other")
    await db_session.user.create_profile(profile1)
    await db_session.user.create_profile(profile2)

    with pytest.raises(ValueError, match="UUID collision"):
        await db_session.union.remap_uuids({other_uuid: existing_uuid})

    # Verify nothing was changed
    p = await db_session.user.get_profile_by_id(other_uuid)
    assert p is not None
    assert p.name == "Other"


@pytest.mark.asyncio
async def test_remap_empty_noop(db_session):
    """Empty remapped dict is a no-op (no crash, no DB activity)."""
    count_before = await db_session.fetchval("SELECT COUNT(*) FROM profiles")
    await db_session.union.remap_uuids({})
    count_after = await db_session.fetchval("SELECT COUNT(*) FROM profiles")
    assert count_after == count_before


@pytest.mark.asyncio
async def test_remap_self_referencing_skip(db_session, user_factory):
    """Self-referencing remap (A→A) is a no-op, profiles and tokens unchanged."""
    user = await user_factory()
    uuid = generate_random_uuid()
    profile = PlayerProfile(uuid, user.id, "SamePlayer")
    await db_session.user.create_profile(profile)

    token = Token(
        access_token=generate_random_uuid(),
        client_token="sf-ct",
        user_id=user.id,
        profile_id=uuid,
        created_at=int(time.time() * 1000),
    )
    await db_session.user.add_token(token)

    await db_session.union.remap_uuids({uuid: uuid})

    p = await db_session.user.get_profile_by_id(uuid)
    assert p is not None
    assert p.name == "SamePlayer"

    t = await db_session.user.get_token(token.access_token)
    assert t is not None
    assert t.profile_id == uuid
