package oauth

import (
	"context"
	"fmt"
	"sort"
	"strings"

	dboauth "element-skin/backend/internal/database/oauth"
	"element-skin/backend/internal/permission"
	noticesvc "element-skin/backend/internal/service/notice"
)

func (s Service) notifyPermissionDependencyChanges(ctx context.Context, result PermissionDependencyResult) error {
	grantGroups := map[string][]dboauth.RevokedGrantDependency{}
	for _, grant := range result.RevokedGrants {
		grantGroups[grant.UserID] = append(grantGroups[grant.UserID], grant)
	}
	for _, userID := range sortedKeys(grantGroups) {
		if err := s.notifyRevokedGrantDependencies(ctx, userID, grantGroups[userID]); err != nil {
			return err
		}
	}

	clientGroups := map[string][]dboauth.DisabledClientDependency{}
	for _, client := range result.DisabledClients {
		clientGroups[client.OwnerUserID] = append(clientGroups[client.OwnerUserID], client)
	}
	for _, userID := range sortedKeys(clientGroups) {
		if err := s.notifyDisabledClientDependencies(ctx, userID, clientGroups[userID]); err != nil {
			return err
		}
	}
	return nil
}

func (s Service) notifyRevokedGrantDependencies(ctx context.Context, userID string, grants []dboauth.RevokedGrantDependency) error {
	if len(grants) == 0 {
		return nil
	}
	content := "你的站点权限发生变化，以下第三方应用授权已自动撤销：\n\n" +
		revokedGrantListMarkdown(grants) +
		"\n这些授权包含你当前已不再拥有的权限，后续访问会失败。需要继续使用时，请在权限恢复后重新授权。"
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "第三方应用授权已自动撤销",
		Summary:         fitNoticeText(fmt.Sprintf("你的权限发生变化，%d 个第三方应用授权已自动撤销。", len(grants)), noticesvc.MaxSummaryLen),
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelWarning,
		LinkText:        "查看授权",
		LinkURL:         "/dashboard/oauth",
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          noticeExpiresAt(),
		TargetUserIDs:   []string{userID},
	})
	return err
}

func (s Service) notifyDisabledClientDependencies(ctx context.Context, userID string, clients []dboauth.DisabledClientDependency) error {
	if len(clients) == 0 {
		return nil
	}
	content := "你的站点权限发生变化，以下第三方应用已自动停用：\n\n" +
		disabledClientListMarkdown(clients) +
		"\n这些应用申请了你当前已不再拥有的权限。请调整应用权限后重新提交审核。"
	_, err := noticesvc.Service{DB: s.DB}.Create(ctx, permission.SystemMaintenanceActor(), noticesvc.CreateInput{
		Type:            noticesvc.TypeSystem,
		Title:           "第三方应用已自动停用",
		Summary:         fitNoticeText(fmt.Sprintf("你的权限发生变化，%d 个你创建的第三方应用已自动停用。", len(clients)), noticesvc.MaxSummaryLen),
		ContentMarkdown: content,
		DisplayMode:     noticesvc.DisplayDetail,
		Level:           noticesvc.LevelWarning,
		LinkText:        "查看应用",
		LinkURL:         "/dashboard/oauth",
		Audience:        noticesvc.AudienceTargeted,
		EndsAt:          noticeExpiresAt(),
		TargetUserIDs:   []string{userID},
	})
	return err
}

func revokedGrantListMarkdown(grants []dboauth.RevokedGrantDependency) string {
	var b strings.Builder
	for _, grant := range grants {
		fmt.Fprintf(&b, "- %s（`%s`）\n", grant.ClientName, grant.ClientID)
	}
	return b.String()
}

func disabledClientListMarkdown(clients []dboauth.DisabledClientDependency) string {
	var b strings.Builder
	for _, client := range clients {
		fmt.Fprintf(&b, "- %s（`%s`）\n", client.Name, client.ClientID)
	}
	return b.String()
}

func sortedKeys[T any](m map[string][]T) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
