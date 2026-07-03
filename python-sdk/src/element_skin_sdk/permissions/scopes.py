"""Known Element Skin permission scope constants."""


class AccountScopes:
    READ_SELF = "account.read.self"
    UPDATE_SELF = "account.update.self"
    DELETE_SELF = "account.delete.self"
    PASSWORD_UPDATE_SELF = "account_password.update.self"


class ProfileScopes:
    READ_OWNED = "profile.read.owned"
    READ_BOUND = "profile.read.bound_profile"
    UPDATE_OWNED = "profile.update.owned"
    DELETE_OWNED = "profile.delete.owned"
    CREATE_OWNED = "profile.create.owned"
    UPDATE_BOUND = "profile.update.bound_profile"


class TextureScopes:
    READ_OWNED = "texture.read.owned"
    READ_PUBLIC = "texture.read.public"
    UPLOAD_OWNED = "texture.upload.owned"
    UPDATE_OWNED = "texture.update.owned"
    DELETE_OWNED = "texture.delete.owned"
    UPDATE_PUBLIC_OWNED = "texture_public.update.owned"


class WardrobeScopes:
    READ_OWNED = "wardrobe.read.owned"
    ENTRY_READ_OWNED = "wardrobe_entry.read.owned"
    ENTRY_ADD_OWNED = "wardrobe_entry.add.owned"
    ENTRY_APPLY_OWNED = "wardrobe_entry.apply.owned"
    ENTRY_UPDATE_OWNED = "wardrobe_entry.update.owned"
    ENTRY_REMOVE_OWNED = "wardrobe_entry.remove.owned"


class NoticeScopes:
    READ_DELIVERED = "notice.read.delivered"
    IGNORE_DELIVERED = "notice.ignore.delivered"


class OAuthScopes:
    CLIENT_CREATE_SELF = "oauth_client.create.self"
    CLIENT_READ_SELF = "oauth_client.read.self"
    CLIENT_UPDATE_SELF = "oauth_client.update.self"
    CLIENT_DELETE_SELF = "oauth_client.delete.self"
    CLIENT_SUBMIT_SELF = "oauth_client.submit.self"
    GRANT_READ_SELF = "oauth_grant.read.self"
    GRANT_REVOKE_SELF = "oauth_grant.revoke.self"


class MinecraftScopes:
    PROFILE_READ_PUBLIC = "minecraft_profile.read.public"
    TEXTURE_PROPERTY_READ_PUBLIC = "minecraft_texture_property.read.public"
    SESSION_HASJOINED_SERVER = "minecraft_session.hasjoined.server"
