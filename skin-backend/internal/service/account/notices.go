package account

import (
	"context"
	"fmt"
	"time"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	noticesvc "element-skin/backend/internal/service/notice"
)

const (
	accountBanReasonMaxRunes = 500
	accountBanNoticeTTL      = 30 * 24 * time.Hour
	accountPermissionTTL     = 30 * 24 * time.Hour
)

func (s AccountService) createBanNotice(ctx context.Context, targetID string, bannedUntil int64, reason string) error {
	endsAt := database.NowMS() + accountBanNoticeTTL.Milliseconds()
	content := fmt.Sprintf("你的账号已被管理员封禁。\n\n封禁截止时间：%d\n\n原因：\n\n%s", bannedUntil, reason)
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "账号已被封禁",
		Summary:         "你的账号已被管理员封禁，详情请查看通知。",
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelDanger,
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          &endsAt,
		TargetUserIDs:   []string{targetID},
	})
	return err
}

func (s AccountService) createRoleChangeNotice(ctx context.Context, targetID, roleID, action string) error {
	roleName := roleDisplayName(roleID)
	title := "权限已更新：角色已授予"
	content := fmt.Sprintf("你的站点角色已被授予：%s。", roleName)
	if action == "revoke" {
		title = "权限已更新：角色已撤销"
		content = fmt.Sprintf("你的站点角色已被撤销：%s。", roleName)
	}
	endsAt := database.NowMS() + accountPermissionTTL.Milliseconds()
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           title,
		Summary:         "你的站点角色已更新，详情请查看通知。",
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelInfo,
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          &endsAt,
		TargetUserIDs:   []string{targetID},
	})
	return err
}

func (s AccountService) createProtectedSubjectTransferNotice(ctx context.Context, targetID string, gained bool) error {
	title := "权限已更新：受保护主体已转出"
	summary := "你不再是受保护权限主体，详情请查看通知。"
	result := "已转移"
	if gained {
		title = "权限已更新：受保护主体已转入"
		summary = "你已成为受保护权限主体，详情请查看通知。"
		result = "允许"
	}
	endsAt := database.NowMS() + accountPermissionTTL.Milliseconds()
	content := fmt.Sprintf(
		"你的受保护权限主体状态已变更。\n\n权限：`%s`\n\n说明：%s\n\n结果：%s",
		manageProtectedPermission.Code,
		manageProtectedPermission.Description,
		result,
	)
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           title,
		Summary:         summary,
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelInfo,
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          &endsAt,
		TargetUserIDs:   []string{targetID},
	})
	return err
}

func roleDisplayName(roleID string) string {
	for _, role := range permission.Roles {
		if role.ID == roleID {
			return fmt.Sprintf("%s（%s）", role.Name, role.ID)
		}
	}
	return roleID
}
