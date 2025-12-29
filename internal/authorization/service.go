package authorization

import "context"

type Service interface {
	Authorize(ctx context.Context, actor string, orgID string, object string, action string) error
}
