package oauth

import (
	"context"
	"strings"

	"element-skin/backend/internal/database"
	"element-skin/backend/internal/permission"
	"element-skin/backend/internal/util"
)

func (s Service) CreateClient(ctx context.Context, actor permission.Actor, input ClientInput) (map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_app.create.owned")); err != nil {
		return nil, forbidden()
	}
	client, permissionIDs, permissionCodes, err := s.clientFromInput(actor, input)
	if err != nil {
		return nil, err
	}
	client.ID, err = util.GenerateUUIDNoDash()
	if err != nil {
		return nil, err
	}
	client.OwnerUserID = actor.UserID
	client.Status = StatusPending
	client.CreatedAt = database.NowMS()
	client.UpdatedAt = client.CreatedAt
	secret := ""
	if client.ClientType == ClientTypeConfidential {
		secret, client.SecretHash, err = generateSecret()
		if err != nil {
			return nil, err
		}
	}
	if err := s.DB.OAuth.CreateClient(ctx, client, permissionIDs); err != nil {
		return nil, err
	}
	if err := s.notifyAdminsClientSubmitted(ctx, client); err != nil {
		return nil, err
	}
	return clientResponse(client, permissionCodes, secret), nil
}

func (s Service) ListClients(ctx context.Context, actor permission.Actor, limit int) ([]map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_app.read.owned")); err != nil {
		return nil, forbidden()
	}
	clients, err := s.DB.OAuth.ListClientsByOwner(ctx, actor.UserID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(clients))
	for _, client := range clients {
		codes, err := s.clientPermissionCodes(ctx, client.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, clientResponse(client, codes, ""))
	}
	return out, nil
}

func (s Service) ListClientsForAdmin(ctx context.Context, actor permission.Actor, status string, limit int) ([]map[string]any, error) {
	if err := actor.Require(permission.MustDefinitionByCode("oauth_app.read.any")); err != nil {
		return nil, forbidden()
	}
	status = strings.TrimSpace(status)
	if status != "" && status != "all" && !validClientStatus(status) {
		return nil, badRequest("invalid status")
	}
	clients, err := s.DB.OAuth.ListClientsByStatus(ctx, status, limit)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]any, 0, len(clients))
	for _, client := range clients {
		out = append(out, adminClientSummary(client))
	}
	return out, nil
}

func (s Service) GetClient(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.read.owned", "oauth_app.read.any")
	if err != nil {
		return nil, err
	}
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, ""), nil
}

func (s Service) UpdateClient(ctx context.Context, actor permission.Actor, clientID string, input ClientInput, status string) (map[string]any, error) {
	current, err := s.clientForActor(ctx, actor, clientID, "oauth_app.update.owned", "oauth_app.update.any")
	if err != nil {
		return nil, err
	}
	client, permissionIDs, permissionCodes, err := s.clientFromInput(actor, input)
	if err != nil {
		return nil, err
	}
	if !actor.Has(permission.MustDefinitionByCode("oauth_app.update.any")) {
		status = current.Status
	}
	if status == "" {
		status = current.Status
	}
	if !validClientStatus(status) {
		return nil, badRequest("invalid status")
	}
	client.ID = current.ID
	client.OwnerUserID = current.OwnerUserID
	client.SecretHash = current.SecretHash
	client.Status = status
	client.CreatedAt = current.CreatedAt
	client.UpdatedAt = database.NowMS()
	updated, err := s.DB.OAuth.UpdateClient(ctx, client, permissionIDs)
	if err != nil {
		return nil, err
	}
	if !updated {
		return nil, notFound("oauth client not found")
	}
	return clientResponse(client, permissionCodes, ""), nil
}

func (s Service) SubmitClientForReview(ctx context.Context, actor permission.Actor, clientID string) (map[string]any, error) {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.update.owned", "oauth_app.update.any")
	if err != nil {
		return nil, err
	}
	client.Status = StatusPending
	client.UpdatedAt = database.NowMS()
	ok, err := s.DB.OAuth.UpdateClientStatus(ctx, client.ID, client.Status, client.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, notFound("oauth client not found")
	}
	if err := s.notifyAdminsClientSubmitted(ctx, *client); err != nil {
		return nil, err
	}
	codes, err := s.clientPermissionCodes(ctx, client.ID)
	if err != nil {
		return nil, err
	}
	return clientResponse(*client, codes, ""), nil
}

func (s Service) DeleteClient(ctx context.Context, actor permission.Actor, clientID string) error {
	client, err := s.clientForActor(ctx, actor, clientID, "oauth_app.delete.owned", "oauth_app.delete.any")
	if err != nil {
		return err
	}
	owner := client.OwnerUserID
	if actor.Has(permission.MustDefinitionByCode("oauth_app.delete.any")) {
		owner = ""
	}
	ok, err := s.DB.OAuth.DeleteClient(ctx, client.ID, owner)
	if err != nil {
		return err
	}
	if !ok {
		return notFound("oauth client not found")
	}
	return nil
}
