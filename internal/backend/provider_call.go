package backend

import (
	"context"
)

func (c Client) callProviderResult(
	ctx context.Context,
	method string,
	params any,
	result any,
) error {
	callCtx, cancel := context.WithTimeout(ctx, providerCallTimeout)
	defer cancel()
	return c.withProviderSession(callCtx, method, func(session *providerSession) error {
		return session.call(method, params, result)
	})
}
