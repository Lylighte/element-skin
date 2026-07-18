package permissions

import (
	"context"
	"fmt"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	noticesvc "element-skin/backend/internal/service/notice"
)

const permissionNoticeTTL = 30 * 24 * time.Hour

func (s PermissionService) createOverrideChangeNotice(ctx context.Context, targetID string, def permission.Definition, effect string) error {
	endsAt := database.NowMS() + permissionNoticeTTL.Milliseconds()
	content := fmt.Sprintf(
		"你的单项权限已被调整。\n\n权限：`%s`\n\n说明：%s\n\n结果：%s",
		def.Code,
		def.Description,
		permissionEffectLabel(effect),
	)
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "权限已更新：单项权限已调整",
		Summary:         "你的单项权限已被管理员调整，详情请查看通知。",
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelInfo,
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          &endsAt,
		TargetUserIDs:   []string{targetID},
	})
	return err
}

func (s PermissionService) createOverrideClearNotice(ctx context.Context, targetID string, def permission.Definition, previousEffect string) error {
	endsAt := database.NowMS() + permissionNoticeTTL.Milliseconds()
	content := fmt.Sprintf(
		"你的单项权限覆盖已被移除。\n\n权限：`%s`\n\n说明：%s\n\n原覆盖结果：%s\n\n当前结果将由你的角色和其他权限规则决定。",
		def.Code,
		def.Description,
		permissionEffectLabel(previousEffect),
	)
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "权限已更新：单项权限覆盖已移除",
		Summary:         "你的单项权限覆盖已被移除，详情请查看通知。",
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelInfo,
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          &endsAt,
		TargetUserIDs:   []string{targetID},
	})
	return err
}

func permissionEffectLabel(effect string) string {
	switch effect {
	case "allow":
		return "允许"
	case "deny":
		return "拒绝"
	default:
		return effect
	}
}
