package engine

import "context"

type NodeDispatcher struct {
}

func (x NodeDispatcher) Send(ctx context.Context, msg interface{}) (response interface{}, err error) {
	return nil, nil
}
