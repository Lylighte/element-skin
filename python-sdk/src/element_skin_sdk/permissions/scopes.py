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
    READ_OWNED = "notice.read.owned"
    DISMISS_OWNED = "notice.dismiss.owned"
    READ_ANY = "notice.read.any"
    CREATE_ANY = "notice.create.any"
    UPDATE_ANY = "notice.update.any"
    DELETE_ANY = "notice.delete.any"


class InviteScopes:
    READ_ANY = "invite.read.any"
    CREATE_ANY = "invite.create.any"
    DELETE_ANY = "invite.delete.any"


class OAuthScopes:
    APP_READ_OWNED = "oauth_app.read.owned"
    APP_CREATE_OWNED = "oauth_app.create.owned"
    APP_UPDATE_OWNED = "oauth_app.update.owned"
    APP_DELETE_OWNED = "oauth_app.delete.owned"
    APP_READ_ANY = "oauth_app.read.any"
    APP_UPDATE_ANY = "oauth_app.update.any"
    APP_DELETE_ANY = "oauth_app.delete.any"
    GRANT_READ_OWNED = "oauth_grant.read.owned"
    GRANT_REVOKE_OWNED = "oauth_grant.revoke.owned"
    GRANT_READ_ANY = "oauth_grant.read.any"
    GRANT_REVOKE_ANY = "oauth_grant.revoke.any"
    TOKEN_REVOKE_OWNED = "oauth_token.revoke.owned"
    TOKEN_INTROSPECT_ANY = "oauth_token.introspect.any"


class MinecraftScopes:
    PROFILE_READ_PUBLIC = "minecraft_profile.read.public"
    TEXTURE_PROPERTY_READ_PUBLIC = "minecraft_texture_property.read.public"
    SESSION_HASJOINED_SERVER = "minecraft_session.hasjoined.server"
